package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type customPageActivationView struct {
	ID                   string `json:"id"`
	TenantID             string `json:"tenantId"`
	Scope                string `json:"scope"`
	ExhibitID            string `json:"exhibitId"`
	PageKey              string `json:"pageKey"`
	DeploymentID         string `json:"deploymentId"`
	Version              string `json:"version"`
	BridgeVersion        string `json:"bridgeVersion"`
	Action               string `json:"action"`
	PreviousActivationID string `json:"previousActivationId"`
	PreviousDeploymentID string `json:"previousDeploymentId"`
	PreviousVersion      string `json:"previousVersion"`
	ActivatedBy          string `json:"activatedBy"`
	Reason               string `json:"reason"`
	CreatedAt            string `json:"createdAt"`
}

type activationList struct {
	List []customPageActivationView `json:"list"`
}

func activateDeployment(in input) (map[string]any, error) {
	if strings.TrimSpace(in.Profile) == "" {
		return developerActivate(in)
	}
	profile, profileEnv, err := loadDeployProfile(in.Profile)
	if err != nil {
		return nil, err
	}
	deploymentID := strings.TrimSpace(in.DeploymentID)
	if deploymentID == "" {
		return nil, errors.New("deployment_id is required")
	}
	scope := strings.TrimSpace(in.Scope)
	if scope != "tenant" && scope != "exhibit" {
		return nil, errors.New("scope 只能是 tenant 或 exhibit")
	}
	exhibitID := strings.TrimSpace(in.ExhibitID)
	if scope == "exhibit" && exhibitID == "" {
		return nil, errors.New("展项级激活必须提供 exhibit_id")
	}
	if scope == "tenant" {
		exhibitID = ""
	}
	expectedSHA := strings.TrimSpace(in.ExpectedSHA256)
	if expectedSHA == "" {
		return nil, errors.New("expected_sha256 is required")
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, errors.New("reason is required")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	deployment, err := getDeploymentByID(ctx, client, profile, deploymentID)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(deployment.ArtifactSHA256, expectedSHA) {
		return nil, errors.New("部署版本的 SHA-256 与 expected_sha256 不一致")
	}
	if deployment.Status != "READY" {
		return nil, fmt.Errorf("部署版本状态为 %s，只有 READY 允许激活", deployment.Status)
	}
	health, err := checkDeploymentHealth(ctx, client, profile, deployment)
	if err != nil {
		return nil, err
	}
	if !health.OK {
		return nil, fmt.Errorf("部署健康检查失败：%s", health.Message)
	}
	idempotent := false
	latest, found, err := findLatestActivation(ctx, client, profile, scope, exhibitID, deployment.PageKey)
	if err != nil {
		return nil, err
	}
	if found && latest.DeploymentID == deploymentID && latest.Action != "CLEAR" {
		idempotent = true
	} else {
		if _, err := postActivation(ctx, client, profile, deploymentID, scope, exhibitID, reason); err != nil {
			return nil, err
		}
		latest, found, err = waitForActivation(ctx, client, profile, scope, exhibitID, deployment.PageKey, deploymentID)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, errors.New("激活成功但未找到对应的激活历史")
		}
	}
	return map[string]any{
		"schema":      deploymentReceiptSchema,
		"ok":          true,
		"action":      "activate",
		"profile":     strings.TrimSpace(in.Profile),
		"profile_env": profileEnv,
		"idempotent":  idempotent,
		"deployment":  deployment,
		"activation":  latest,
		"health":      health,
		"completedAt": time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func getDeploymentByID(ctx context.Context, client *http.Client, profile deployProfile, id string) (deploymentView, error) {
	query := url.Values{}
	query.Set("tenantId", profile.TenantID)
	response, err := doAPIRequest(ctx, client, profile, http.MethodGet, "/api/admin/custom-pages/"+url.PathEscape(id)+"?"+query.Encode(), nil, "")
	if err != nil {
		return deploymentView{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return deploymentView{}, apiError(response, profile.AdminToken)
	}
	var deployment deploymentView
	if err := json.Unmarshal(response.Data, &deployment); err != nil {
		return deploymentView{}, fmt.Errorf("解析部署版本失败：%w", err)
	}
	return deployment, nil
}

func findLatestActivation(ctx context.Context, client *http.Client, profile deployProfile, scope, exhibitID, pageKey string) (customPageActivationView, bool, error) {
	query := url.Values{}
	query.Set("tenantId", profile.TenantID)
	query.Set("scope", scope)
	if exhibitID != "" {
		query.Set("exhibitId", exhibitID)
	}
	if pageKey != "" {
		query.Set("pageKey", pageKey)
	}
	query.Set("page", "1")
	query.Set("pageSize", "20")
	response, err := doAPIRequest(ctx, client, profile, http.MethodGet, "/api/admin/custom-pages/activations?"+query.Encode(), nil, "")
	if err != nil {
		return customPageActivationView{}, false, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return customPageActivationView{}, false, apiError(response, profile.AdminToken)
	}
	var list activationList
	if err := json.Unmarshal(response.Data, &list); err != nil {
		return customPageActivationView{}, false, fmt.Errorf("解析激活历史失败：%w", err)
	}
	if len(list.List) == 0 {
		return customPageActivationView{}, false, nil
	}
	return list.List[0], true, nil
}

func postActivation(ctx context.Context, client *http.Client, profile deployProfile, deploymentID, scope, exhibitID, reason string) (deploymentView, error) {
	body := map[string]any{
		"tenantId": profile.TenantID,
		"scope":    scope,
		"reason":   reason,
	}
	if scope == "exhibit" {
		body["exhibitId"] = exhibitID
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return deploymentView{}, err
	}
	response, err := doAPIRequest(ctx, client, profile, http.MethodPost, "/api/admin/custom-pages/"+url.PathEscape(deploymentID)+"/activate", payload, "application/json")
	if err != nil {
		return deploymentView{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return deploymentView{}, apiError(response, profile.AdminToken)
	}
	var deployment deploymentView
	if err := json.Unmarshal(response.Data, &deployment); err != nil {
		return deploymentView{}, fmt.Errorf("解析激活回执失败：%w", err)
	}
	return deployment, nil
}

func waitForActivation(ctx context.Context, client *http.Client, profile deployProfile, scope, exhibitID, pageKey, deploymentID string) (customPageActivationView, bool, error) {
	for attempt := 0; attempt < 5; attempt++ {
		latest, found, err := findLatestActivation(ctx, client, profile, scope, exhibitID, pageKey)
		if err != nil {
			return customPageActivationView{}, false, err
		}
		if found && latest.DeploymentID == deploymentID {
			return latest, true, nil
		}
		select {
		case <-ctx.Done():
			return customPageActivationView{}, false, ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
	return customPageActivationView{}, false, nil
}

func jsonBody(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return bytes.TrimSpace(body), nil
}
