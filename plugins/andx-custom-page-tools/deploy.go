package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"strconv"
	"strings"
	"time"
)

const deploymentReceiptSchema = "andx-custom-page-deployment/v1"

var profileNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,31}$`)

type deployProfile struct {
	BaseURL    string `json:"base_url"`
	AdminToken string `json:"admin_token"`
	TenantID   string `json:"tenant_id"`
}

type deploymentView struct {
	ID             json.RawMessage `json:"id"`
	TenantID       string          `json:"tenantId"`
	PageKey        string          `json:"pageKey"`
	Version        string          `json:"version"`
	DisplayMode    string          `json:"displayMode"`
	EntryFile      string          `json:"entryFile"`
	BridgeVersion  string          `json:"bridgeVersion"`
	URL            string          `json:"url"`
	ArtifactSHA256 string          `json:"artifactSha256"`
	ArtifactBytes  int64           `json:"artifactBytes"`
	FileCount      int             `json:"fileCount"`
	Status         string          `json:"status"`
	ActivatedAt    any             `json:"activatedAt"`
	CreatedAt      string          `json:"createdAt"`
}

type deploymentList struct {
	List []deploymentView `json:"list"`
}

type deploymentHealth struct {
	OK         bool   `json:"ok"`
	URL        string `json:"url"`
	StatusCode int    `json:"statusCode"`
	LatencyMS  int64  `json:"latencyMs"`
	CheckedAt  string `json:"checkedAt"`
	Message    string `json:"message"`
}

type apiResult struct {
	StatusCode int
	Code       json.RawMessage
	Message    string
	Data       json.RawMessage
}

func deployPreview(in input) (map[string]any, error) {
	if strings.TrimSpace(in.Profile) == "" {
		return developerPreview(in)
	}
	projectRoot, artifactPath, err := resolveArtifactPath(in)
	if err != nil {
		return nil, err
	}
	local, err := inspectArtifactArchive(artifactPath)
	if err != nil {
		return nil, err
	}
	if expected := strings.TrimSpace(in.ExpectedSHA256); expected != "" && !strings.EqualFold(expected, local.SHA256) {
		return nil, errors.New("artifact_zip 的 SHA-256 与 expected_sha256 不一致")
	}
	profile, profileEnv, err := loadDeployProfile(in.Profile)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	existing, found, err := findDeployment(ctx, client, profile, local.PageKey, local.Version)
	if err != nil {
		return nil, err
	}
	idempotent := false
	var deployment deploymentView
	if found {
		if !strings.EqualFold(existing.ArtifactSHA256, local.SHA256) {
			return nil, errors.New("相同 page_key/version 已存在但 SHA-256 不一致，拒绝覆盖")
		}
		deployment = existing
		idempotent = true
	} else {
		deployment, err = uploadDeployment(ctx, client, profile, artifactPath, local, in.Description)
		if err != nil {
			if !isConflict(err) {
				return nil, err
			}
			existing, found, lookupErr := findDeployment(ctx, client, profile, local.PageKey, local.Version)
			if lookupErr != nil {
				return nil, lookupErr
			}
			if !found || !strings.EqualFold(existing.ArtifactSHA256, local.SHA256) {
				return nil, errors.New("上传冲突后未找到相同 SHA-256 的部署版本")
			}
			deployment = existing
			idempotent = true
		}
	}
	health, err := checkDeploymentHealth(ctx, client, profile, deployment)
	if err != nil {
		return nil, err
	}
	receipt := map[string]any{
		"schema":       deploymentReceiptSchema,
		"ok":           health.OK,
		"action":       "preview",
		"profile":      strings.TrimSpace(in.Profile),
		"profile_env":  profileEnv,
		"idempotent":   idempotent,
		"project_root": projectRoot,
		"artifact": map[string]any{
			"path":   artifactPath,
			"sha256": local.SHA256,
			"bytes":  local.Bytes,
		},
		"deployment":  deployment,
		"health":      health,
		"completedAt": time.Now().UTC().Format(time.RFC3339),
	}
	if !health.OK {
		return receipt, nil
	}
	return receipt, nil
}

type localArtifact struct {
	Path        string
	SHA256      string
	Bytes       int64
	PageKey     string
	Version     string
	DisplayMode string
	Archive     []byte
}

func resolveArtifactPath(in input) (string, string, error) {
	projectRoot := strings.TrimSpace(in.ProjectRoot)
	artifactInput := strings.TrimSpace(in.ArtifactZip)
	if projectRoot == "" {
		return "", "", errors.New("project_root is required")
	}
	if artifactInput == "" {
		return "", "", errors.New("artifact_zip is required")
	}
	projectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", "", err
	}
	projectRoot, err = filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return "", "", fmt.Errorf("project_root 不存在或无法解析：%w", err)
	}
	if !filepath.IsAbs(artifactInput) {
		artifactInput = filepath.Join(projectRoot, artifactInput)
	}
	artifactPath, err := filepath.Abs(artifactInput)
	if err != nil {
		return "", "", err
	}
	if !pathWithin(projectRoot, artifactPath) {
		return "", "", errors.New("artifact_zip 必须位于 project_root 内")
	}
	info, err := os.Stat(artifactPath)
	if err != nil || !info.Mode().IsRegular() {
		return "", "", errors.New("artifact_zip 必须是已存在的 ZIP 文件")
	}
	return projectRoot, artifactPath, nil
}

func inspectArtifactArchive(path string) (localArtifact, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return localArtifact{}, err
	}
	sum := sha256.Sum256(body)
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return localArtifact{}, fmt.Errorf("artifact_zip 不是有效 ZIP：%w", err)
	}
	var manifestBytes []byte
	for _, entry := range reader.File {
		if entry.Name == "manifest.json" {
			if entry.FileInfo().IsDir() {
				continue
			}
			file, err := entry.Open()
			if err != nil {
				return localArtifact{}, err
			}
			manifestBytes, err = io.ReadAll(file)
			_ = file.Close()
			if err != nil {
				return localArtifact{}, err
			}
			break
		}
	}
	if len(manifestBytes) == 0 {
		return localArtifact{}, errors.New("artifact_zip 缺少根目录 manifest.json")
	}
	var manifest pageManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return localArtifact{}, fmt.Errorf("artifact_zip 的 manifest.json 无效：%w", err)
	}
	if manifest.EntryFile != "index.html" {
		return localArtifact{}, errors.New("artifact_zip 的 entryFile 必须为 index.html")
	}
	return localArtifact{
		Path:        path,
		SHA256:      hex.EncodeToString(sum[:]),
		Bytes:       int64(len(body)),
		PageKey:     manifest.PageKey,
		Version:     manifest.Version,
		DisplayMode: manifest.DisplayMode,
		Archive:     body,
	}, nil
}

func loadDeployProfile(name string) (deployProfile, string, error) {
	name = strings.TrimSpace(name)
	if !profileNamePattern.MatchString(name) {
		return deployProfile{}, "", errors.New("profile 只能包含字母、数字、下划线和连字符")
	}
	envName := "ANDX_PAGE_PROFILE_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	raw := strings.TrimSpace(os.Getenv(envName))
	if raw == "" {
		return deployProfile{}, envName, fmt.Errorf("未配置部署 Profile 环境变量 %s", envName)
	}
	var profile deployProfile
	if err := json.Unmarshal([]byte(raw), &profile); err != nil {
		return deployProfile{}, envName, fmt.Errorf("部署 Profile %s 不是合法 JSON", envName)
	}
	parsed, err := url.Parse(strings.TrimSpace(profile.BaseURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && !isLoopbackHTTP(parsed)) {
		return deployProfile{}, envName, errors.New("Profile base_url 必须是 HTTPS；本机回环地址可使用 HTTP")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return deployProfile{}, envName, errors.New("Profile base_url 只能包含站点根地址")
	}
	profile.BaseURL = strings.TrimRight(parsed.String(), "/")
	profile.AdminToken = strings.TrimSpace(profile.AdminToken)
	profile.TenantID = strings.TrimSpace(profile.TenantID)
	if profile.AdminToken == "" {
		return deployProfile{}, envName, errors.New("Profile 缺少 admin_token")
	}
	if _, err := strconv.ParseUint(profile.TenantID, 10, 64); err != nil {
		return deployProfile{}, envName, errors.New("Profile tenant_id 必须是数字字符串")
	}
	return profile, envName, nil
}

func isLoopbackHTTP(value *url.URL) bool {
	if value.Scheme != "http" {
		return false
	}
	host := value.Hostname()
	return host == "127.0.0.1" || host == "::1" || strings.EqualFold(host, "localhost")
}

func findDeployment(ctx context.Context, client *http.Client, profile deployProfile, pageKey, version string) (deploymentView, bool, error) {
	query := url.Values{}
	query.Set("tenantId", profile.TenantID)
	query.Set("pageKey", pageKey)
	query.Set("page", "1")
	query.Set("pageSize", "100")
	response, err := doAPIRequest(ctx, client, profile, http.MethodGet, "/api/admin/custom-pages?"+query.Encode(), nil, "")
	if err != nil {
		return deploymentView{}, false, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return deploymentView{}, false, apiError(response, profile.AdminToken)
	}
	var list deploymentList
	if err := json.Unmarshal(response.Data, &list); err != nil {
		return deploymentView{}, false, fmt.Errorf("解析定制页列表失败：%w", err)
	}
	for _, item := range list.List {
		if item.Version == version {
			return item, true, nil
		}
	}
	return deploymentView{}, false, nil
}

func uploadDeployment(ctx context.Context, client *http.Client, profile deployProfile, artifactPath string, local localArtifact, description string) (deploymentView, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"tenantId":    profile.TenantID,
		"pageKey":     local.PageKey,
		"version":     local.Version,
		"displayMode": local.DisplayMode,
	}
	if description = strings.TrimSpace(description); description != "" {
		fields["description"] = description
	}
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			return deploymentView{}, err
		}
	}
	filePart, err := writer.CreateFormFile("file", filepath.Base(artifactPath))
	if err != nil {
		return deploymentView{}, err
	}
	if _, err := filePart.Write(local.Archive); err != nil {
		return deploymentView{}, err
	}
	if err := writer.Close(); err != nil {
		return deploymentView{}, err
	}
	response, err := doAPIRequest(ctx, client, profile, http.MethodPost, "/api/admin/custom-pages", body.Bytes(), writer.FormDataContentType())
	if err != nil {
		return deploymentView{}, err
	}
	if response.StatusCode == http.StatusConflict {
		return deploymentView{}, &conflictError{Message: redactMessage(response.Message, profile.AdminToken)}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return deploymentView{}, apiError(response, profile.AdminToken)
	}
	var deployment deploymentView
	if err := json.Unmarshal(response.Data, &deployment); err != nil {
		return deploymentView{}, fmt.Errorf("解析上传回执失败：%w", err)
	}
	if !strings.EqualFold(deployment.ArtifactSHA256, local.SHA256) {
		return deploymentView{}, errors.New("服务端部署回执的 SHA-256 与本地制品不一致")
	}
	return deployment, nil
}

func checkDeploymentHealth(ctx context.Context, client *http.Client, profile deployProfile, deployment deploymentView) (deploymentHealth, error) {
	id, err := jsonScalarString(deployment.ID)
	if err != nil || id == "" {
		return deploymentHealth{}, errors.New("部署回执缺少 deployment id")
	}
	query := url.Values{}
	query.Set("tenantId", profile.TenantID)
	response, err := doAPIRequest(ctx, client, profile, http.MethodPost, "/api/admin/custom-pages/"+url.PathEscape(id)+"/health-check?"+query.Encode(), []byte("{}"), "application/json")
	if err != nil {
		return deploymentHealth{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return deploymentHealth{}, apiError(response, profile.AdminToken)
	}
	var health deploymentHealth
	if err := json.Unmarshal(response.Data, &health); err != nil {
		return deploymentHealth{}, fmt.Errorf("解析健康检查回执失败：%w", err)
	}
	return health, nil
}

func doAPIRequest(ctx context.Context, client *http.Client, profile deployProfile, method, path string, body []byte, contentType string) (apiResult, error) {
	request, err := http.NewRequestWithContext(ctx, method, profile.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return apiResult{}, err
	}
	request.Header.Set("accept", "application/json")
	request.Header.Set("authorization", "Bearer "+profile.AdminToken)
	if contentType != "" {
		request.Header.Set("content-type", contentType)
	}
	safeClient := *client
	safeClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	response, err := safeClient.Do(request)
	if err != nil {
		return apiResult{}, errors.New(redactMessage(err.Error(), profile.AdminToken))
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024))
	if err != nil {
		return apiResult{}, err
	}
	result := apiResult{StatusCode: response.StatusCode, Data: responseBody}
	var envelope struct {
		Code    json.RawMessage `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if len(responseBody) > 0 && json.Unmarshal(responseBody, &envelope) == nil && len(envelope.Data) > 0 {
		result.Code = envelope.Code
		result.Message = redactMessage(envelope.Message, profile.AdminToken)
		result.Data = envelope.Data
	}
	return result, nil
}

func apiError(response apiResult, token string) error {
	message := redactMessage(strings.TrimSpace(response.Message), token)
	if message == "" {
		message = redactMessage(string(response.Data), token)
	}
	if message == "" {
		message = "AndX 管理 API 请求失败"
	}
	return fmt.Errorf("AndX 管理 API 返回 %d：%s", response.StatusCode, message)
}

func redactMessage(value, token string) string {
	value = strings.TrimSpace(value)
	if token != "" {
		value = strings.ReplaceAll(value, token, "[REDACTED]")
	}
	return value
}

func jsonScalarString(value json.RawMessage) (string, error) {
	value = bytes.TrimSpace(value)
	if len(value) == 0 || bytes.Equal(value, []byte("null")) {
		return "", nil
	}
	var text string
	if err := json.Unmarshal(value, &text); err == nil {
		return text, nil
	}
	var number json.Number
	if err := json.Unmarshal(value, &number); err == nil {
		return number.String(), nil
	}
	return "", errors.New("id 必须是字符串或数字")
}

type conflictError struct{ Message string }

func (e *conflictError) Error() string {
	if e.Message == "" {
		return "部署版本冲突"
	}
	return e.Message
}

func isConflict(err error) bool {
	var conflict *conflictError
	return errors.As(err, &conflict)
}
