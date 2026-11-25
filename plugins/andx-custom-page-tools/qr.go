package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type interactionQRResponse struct {
	Available   bool   `json:"available"`
	DataURL     string `json:"dataUrl"`
	URL         string `json:"url"`
	Reason      string `json:"reason"`
	Scene       string `json:"scene"`
	ProjectCode string `json:"projectCode"`
	ExhibitCode string `json:"exhibitCode"`
}

func exportInteractionQR(in input) (map[string]any, error) {
	profile, _, err := deliveryProfile(in, "andx.page.qr.export")
	if err != nil {
		return nil, err
	}
	projectID := strings.TrimSpace(in.ProjectID)
	exhibitID := strings.TrimSpace(in.ExhibitID)
	if projectID == "" || exhibitID == "" {
		return nil, errors.New("project_id 和 exhibit_id 必填")
	}
	workspace := strings.TrimSpace(in.ProjectRoot)
	outputPNG := strings.TrimSpace(in.OutputPNG)
	if workspace == "" || outputPNG == "" {
		return nil, errors.New("project_root 和 output_png 必填")
	}
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	workspaceAbs, err = filepath.EvalSymlinks(workspaceAbs)
	if err != nil {
		return nil, fmt.Errorf("project_root 不存在：%w", err)
	}
	if !filepath.IsAbs(outputPNG) {
		outputPNG = filepath.Join(workspaceAbs, outputPNG)
	}
	outputPNG, err = filepath.Abs(outputPNG)
	if err != nil {
		return nil, err
	}
	if !pathWithin(workspaceAbs, outputPNG) {
		return nil, errors.New("output_png 必须位于 project_root 内")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	path := "/api/admin/projects/" + url.PathEscape(projectID) + "/exhibits/" + url.PathEscape(exhibitID) + "/interaction-qrcode"
	if in.Profile == "" {
		path = developerBase(in) + "/interaction-qrcode"
	}
	response, err := doAPIRequest(ctx, client, profile, http.MethodGet, path, nil, "")
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, apiError(response, profile.AdminToken)
	}
	var qr interactionQRResponse
	if err := json.Unmarshal(response.Data, &qr); err != nil {
		return nil, fmt.Errorf("解析互动二维码失败：%w", err)
	}
	if !qr.Available || qr.DataURL == "" {
		return nil, errors.New("互动二维码不可用：" + qr.Reason)
	}
	if err := assertInteractionEntryURL(qr.URL, qr.Scene); err != nil {
		return nil, err
	}
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(qr.DataURL, prefix) {
		return nil, errors.New("互动二维码 dataUrl 不是 PNG")
	}
	body, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(qr.DataURL, prefix))
	if err != nil || len(body) == 0 {
		return nil, errors.New("互动二维码 PNG 解码失败")
	}
	if err := os.MkdirAll(filepath.Dir(outputPNG), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(outputPNG, body, 0o644); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	return map[string]any{
		"schema":       "andx-custom-page-qr/v1",
		"ok":           true,
		"qr_png":       outputPNG,
		"sha256":       hex.EncodeToString(sum[:]),
		"bytes":        len(body),
		"url":          qr.URL,
		"scene":        qr.Scene,
		"project_code": qr.ProjectCode,
		"exhibit_code": qr.ExhibitCode,
	}, nil
}

func assertInteractionEntryURL(rawURL, scene string) error {
	if strings.TrimSpace(scene) == "" {
		return errors.New("互动二维码缺少 scene")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return errors.New("互动二维码 URL 无效")
	}
	expected := "/x/" + url.PathEscape(scene)
	if !strings.HasSuffix(strings.TrimRight(parsed.Path, "/"), expected) {
		return fmt.Errorf("互动二维码必须指向稳定入口 /x/{publicCode}，当前为 %s", parsed.Path)
	}
	return nil
}
