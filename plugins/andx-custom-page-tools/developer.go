package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var andxID = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)

// The host resolves the workspace binding before creating this invocation's
// broker. No API accepts a raw developer/device key in capability arguments.
func deliveryProfile(in input, purpose string) (deployProfile, string, error) {
	if in.Profile != "" {
		return loadDeployProfile(in.Profile)
	}
	if !andxID.MatchString(in.ProjectID) || !andxID.MatchString(in.ExhibitID) {
		return deployProfile{}, "", errors.New("工程尚未绑定 AndX 目标")
	}
	return acquireDeveloperProfile(in, purpose)
}

func acquireDeveloperProfile(in input, purpose string) (deployProfile, string, error) {
	if in.CredentialRef == "" {
		return deployProfile{}, "", errors.New("缺少安全导入的凭证引用")
	}
	brokerURL, brokerToken := os.Getenv("HIMIND_CREDENTIAL_BROKER_URL"), os.Getenv("HIMIND_CREDENTIAL_BROKER_TOKEN")
	parsed, err := url.Parse(brokerURL)
	if err != nil || parsed.Scheme != "http" || !isLoopbackHTTP(parsed) || parsed.Path != "/acquire" || brokerToken == "" {
		return deployProfile{}, "", errors.New("缺少本次调用的本机凭证代理，请更新 Agent 和插件")
	}
	body, _ := json.Marshal(map[string]string{"credential_id": "andx.developer", "purpose": purpose, "profile_ref": in.CredentialRef})
	request, err := http.NewRequest(http.MethodPost, brokerURL, bytes.NewReader(body))
	if err != nil {
		return deployProfile{}, "", errors.New("凭证代理地址无效")
	}
	request.Header.Set("Authorization", "Bearer "+brokerToken)
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return deployProfile{}, "", errors.New("本机凭证代理连接失败")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return deployProfile{}, "", errors.New("凭证代理拒绝本次目标或用途，请重新导入/绑定")
	}
	var receipt struct {
		Fields map[string]string `json:"fields"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 64*1024)).Decode(&receipt) != nil {
		return deployProfile{}, "", errors.New("凭证代理回执无效")
	}
	base, err := url.Parse(receipt.Fields["endpoint"])
	if err != nil || base.Host == "" || (base.Scheme != "https" && !(base.Scheme == "http" && isLoopbackHTTP(base))) || base.User != nil || base.RawQuery != "" || base.Fragment != "" || (base.Path != "" && base.Path != "/") {
		return deployProfile{}, "", errors.New("AndX 地址必须是 HTTPS 源站")
	}
	key := receipt.Fields["developer_key"]
	if !regexp.MustCompile(`^andx_dev_[A-Za-z0-9_-]{43}$`).MatchString(key) {
		return deployProfile{}, "", errors.New("开发凭证格式无效，请重新导入")
	}
	return deployProfile{BaseURL: strings.TrimRight(base.String(), "/"), AdminToken: key, TenantID: in.ProjectID}, "credential-broker", nil
}

func developerBase(in input) string {
	return "/api/developer/projects/" + url.PathEscape(in.ProjectID) + "/exhibits/" + url.PathEscape(in.ExhibitID)
}

func developerRequest(ctx context.Context, profile deployProfile, method, path string, body []byte, contentType string, out any) error {
	response, err := doAPIRequest(ctx, &http.Client{Timeout: 30 * time.Second}, profile, method, path, body, contentType)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return apiError(response, profile.AdminToken)
	}
	if string(response.Code) != "0" {
		return errors.New("AndX 开发 API 未返回成功信封")
	}
	if err := json.Unmarshal(response.Data, out); err != nil {
		return errors.New("AndX 开发 API 回执格式无效")
	}
	return nil
}

func developerPreview(in input) (map[string]any, error) {
	root, artifact, err := resolveArtifactPath(in)
	if err != nil {
		return nil, err
	}
	local, err := inspectArtifactArchive(artifact)
	if err != nil {
		return nil, err
	}
	if in.ExpectedSHA256 != "" && in.ExpectedSHA256 != local.SHA256 {
		return nil, errors.New("制品摘要与预期不符")
	}
	profile, _, err := deliveryProfile(in, "andx.page.deploy.preview")
	if err != nil {
		return nil, err
	}
	if local.PageKey != "exhibit-"+in.ExhibitID {
		return nil, errors.New("制品 page_key 与工程绑定展项不符，请按绑定上下文重新打包")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range map[string]string{"version": local.Version, "expectedSha256": local.SHA256, "displayMode": local.DisplayMode} {
		if err := writer.WriteField(key, value); err != nil {
			return nil, err
		}
	}
	if in.Description != "" {
		if err := writer.WriteField("description", in.Description); err != nil {
			return nil, err
		}
	}
	part, err := writer.CreateFormFile("file", filepath.Base(artifact))
	if err != nil {
		return nil, err
	}
	if _, err = part.Write(local.Archive); err != nil {
		return nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var upload struct {
		Deployment          deploymentView `json:"deployment"`
		Reused              bool           `json:"reused"`
		CurrentActivationID string         `json:"currentActivationId"`
	}
	if err := developerRequest(ctx, profile, http.MethodPost, developerBase(in)+"/pages", body.Bytes(), writer.FormDataContentType(), &upload); err != nil {
		return nil, err
	}
	if upload.Deployment.ArtifactSHA256 != local.SHA256 || upload.Deployment.TenantID != in.ProjectID || upload.Deployment.PageKey != local.PageKey {
		return nil, errors.New("服务端制品回执与绑定或本地摘要不符")
	}
	id, err := jsonScalarString(upload.Deployment.ID)
	if err != nil || !andxID.MatchString(id) {
		return nil, errors.New("部署编号无效")
	}
	var health deploymentHealth
	if err := developerRequest(ctx, profile, http.MethodPost, developerBase(in)+"/pages/"+id+"/health-check", []byte("{}"), "application/json", &health); err != nil {
		return nil, err
	}
	receipt := map[string]any{"schema": "andx.page.delivery.v2", "ok": health.OK, "action": "preview", "credential_ref": in.CredentialRef, "binding_revision": in.BindingRevision, "project_id": in.ProjectID, "exhibit_id": in.ExhibitID, "project_root": root, "deployment": upload.Deployment, "health": health, "expected_activation_id": upload.CurrentActivationID, "artifact": map[string]any{"path": artifact, "sha256": local.SHA256, "bytes": local.Bytes}, "completedAt": time.Now().UTC().Format(time.RFC3339)}
	if health.OK {
		var preview map[string]any
		if err := developerRequest(ctx, profile, http.MethodPost, developerBase(in)+"/pages/"+id+"/preview", []byte("{}"), "application/json", &preview); err != nil {
			return nil, err
		}
		receipt["preview"] = preview
	}
	return receipt, nil
}

func developerActivate(in input) (map[string]any, error) {
	if !andxID.MatchString(in.DeploymentID) || in.ExpectedActivationID == "" || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(in.ExpectedSHA256) || strings.TrimSpace(in.Reason) == "" {
		return nil, errors.New("必须提供预览回执的 deployment_id、expected_activation_id、精确摘要及发布原因")
	}
	if in.Scope != "" && in.Scope != "exhibit" {
		return nil, errors.New("开发者仅可发布到工程绑定展项")
	}
	profile, _, err := deliveryProfile(in, "andx.page.deploy.activate")
	if err != nil {
		return nil, err
	}
	payload, _ := json.Marshal(map[string]string{"expectedSha256": in.ExpectedSHA256, "expectedActivationId": in.ExpectedActivationID, "reason": in.Reason})
	action := "activate"
	if in.Rollback {
		action = "rollback"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var result map[string]any
	if err := developerRequest(ctx, profile, http.MethodPost, developerBase(in)+"/pages/"+in.DeploymentID+"/"+action, payload, "application/json", &result); err != nil {
		return nil, err
	}
	target, ok := result["target"].(map[string]any)
	if !ok {
		return nil, errors.New("发布回执缺少目标")
	}
	exhibit, _ := target["exhibit"].(map[string]any)
	project, _ := target["project"].(map[string]any)
	if fmt.Sprint(exhibit["id"]) != in.ExhibitID || fmt.Sprint(project["id"]) != in.ProjectID {
		return nil, errors.New("发布回执目标不符")
	}
	deployment, _ := result["deployment"].(map[string]any)
	activation, _ := result["activation"].(map[string]any)
	if fmt.Sprint(deployment["tenantId"]) != in.ProjectID || fmt.Sprint(deployment["pageKey"]) != "exhibit-"+in.ExhibitID || deployment["artifactSha256"] != in.ExpectedSHA256 {
		return nil, errors.New("发布回执制品摘要或归属不符")
	}
	if !in.Rollback && fmt.Sprint(deployment["id"]) != in.DeploymentID {
		return nil, errors.New("发布回执部署编号不符")
	}
	if !andxID.MatchString(fmt.Sprint(activation["id"])) || fmt.Sprint(activation["id"]) == in.ExpectedActivationID || fmt.Sprint(activation["tenantId"]) != in.ProjectID || fmt.Sprint(activation["exhibitId"]) != in.ExhibitID || fmt.Sprint(activation["deploymentId"]) != fmt.Sprint(deployment["id"]) || activation["scope"] != "exhibit" || !strings.EqualFold(fmt.Sprint(activation["action"]), action) {
		return nil, errors.New("发布回执激活记录不符")
	}
	result["schema"] = "andx.page.delivery.v2"
	result["ok"] = true
	result["action"] = action
	result["credential_ref"] = in.CredentialRef
	return result, nil
}

// Consume the short-lived code in the trusted process. Only the private receipt
// path reaches the model/workflow ledger, never the bearer code itself.
func developerEnrollment(in input) (map[string]any, error) {
	if in.Profile != "" {
		return nil, errors.New("设备接入码需要开发者工程绑定，不接受旧管理 Profile")
	}
	profile, _, err := deliveryProfile(in, "andx.page.device.enrollment")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var result map[string]any
	if err := developerRequest(ctx, profile, http.MethodPost, developerBase(in)+"/enrollment", []byte("{}"), "application/json", &result); err != nil {
		return nil, err
	}
	if result["schema"] != "andx.device.enrollment.v1" || fmt.Sprint(result["exhibitId"]) != in.ExhibitID {
		return nil, errors.New("设备接入回执目标不符")
	}
	code, _ := result["enrollmentToken"].(string)
	if len(code) < 20 || len(code) > 128 {
		return nil, errors.New("设备接入回执缺少有效的一次性码")
	}
	dataRoot := os.Getenv("HIMIND_PLUGIN_DATA_ROOT")
	if dataRoot == "" || !filepath.IsAbs(dataRoot) {
		return nil, errors.New("缺少 Agent 私有存储，接入码未导出，请更新 Agent")
	}
	dir := filepath.Join(dataRoot, "device-enrollment")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, errors.New("无法创建接入码私有目录")
	}
	file, err := os.CreateTemp(dir, "enrollment-*.json")
	if err != nil {
		return nil, errors.New("无法创建接入码私有文件")
	}
	path := file.Name()
	encoded, err := json.Marshal(result)
	if err == nil {
		_, err = file.Write(encoded)
	}
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(path)
		return nil, errors.New("接入码私有导出失败")
	}
	return map[string]any{"schema": "andx.device.enrollment.receipt.v1", "ok": true, "project_id": in.ProjectID, "exhibit_id": in.ExhibitID, "private_path": path, "expires_at": result["expiresAt"], "message": "一次性接入信息已保存到 Agent 私有文件，由可信设备部署流程读取；请勿复制到源码、模型或普通会话。"}, nil
}
