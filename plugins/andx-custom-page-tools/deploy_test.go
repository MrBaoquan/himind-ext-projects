package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDeployPreviewIsIdempotentAndRedactsToken(t *testing.T) {
	project := t.TempDir()
	writeTestFile(t, project, "dist/index.html", `<!doctype html><html><body><script src="./app.js"></script></body></html>`)
	writeTestFile(t, project, "dist/app.js", `console.log("deploy")`)
	packageResult, err := packagePage(input{
		ProjectRoot: project,
		SourceRoot:  "dist",
		OutputZip:   "artifacts/page-demo.zip",
		PageKey:     "page-demo",
		Version:     "1.0.0",
		DisplayMode: "embedded",
		BuiltAt:     "2026-11-06T00:00:00Z",
		Force:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	artifactSHA := packageResult["sha256"].(string)
	token := "secret-token-for-test"
	var uploads atomic.Int32
	var lists atomic.Int32
	var uploaded atomic.Bool
	deploymentData := func() map[string]any {
		return map[string]any{
			"id":             "deployment-1",
			"tenantId":       "1",
			"pageKey":        "page-demo",
			"version":        "1.0.0",
			"displayMode":    "embedded",
			"entryFile":      "index.html",
			"bridgeVersion":  "1.0",
			"url":            "https://andx.example.com/interaction-pages/1/page-demo/1.0.0/index.html",
			"artifactSha256": artifactSHA,
			"artifactBytes":  packageResult["artifact_bytes"],
			"fileCount":      packageResult["file_count"],
			"status":         "READY",
			"activatedAt":    nil,
			"createdAt":      "2026-11-06T00:00:00Z",
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("authorization") != "Bearer "+token {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		writer.Header().Set("content-type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/admin/custom-pages":
			lists.Add(1)
			list := []any{}
			if uploaded.Load() {
				list = append(list, deploymentData())
			}
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data":    map[string]any{"list": list},
			})
		case request.Method == http.MethodPost && request.URL.Path == "/api/admin/custom-pages":
			uploads.Add(1)
			uploaded.Store(true)
			writer.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data":    deploymentData(),
			})
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/health-check"):
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"code":    0,
				"message": "ok",
				"data": map[string]any{
					"ok":         true,
					"url":        "https://andx.example.com/interaction-pages/1/page-demo/1.0.0/index.html",
					"statusCode": 200,
					"latencyMs":  12,
					"checkedAt":  "2026-11-06T00:00:00Z",
					"message":    "部署地址可访问且返回 HTML",
				},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	profile := fmt.Sprintf(`{"base_url":%q,"admin_token":%q,"tenant_id":"1"}`, server.URL, token)
	t.Setenv("ANDX_PAGE_PROFILE_TEST", profile)
	in := input{ProjectRoot: project, ArtifactZip: "artifacts/page-demo.zip", Profile: "test"}
	first, err := deployPreview(in)
	if err != nil {
		t.Fatal(err)
	}
	if first["ok"] != true || first["idempotent"] != false {
		t.Fatalf("unexpected first receipt: %#v", first)
	}
	second, err := deployPreview(in)
	if err != nil {
		t.Fatal(err)
	}
	if second["ok"] != true || second["idempotent"] != true {
		t.Fatalf("unexpected second receipt: %#v", second)
	}
	if uploads.Load() != 1 || lists.Load() != 2 {
		t.Fatalf("unexpected request counts: uploads=%d lists=%d", uploads.Load(), lists.Load())
	}
	for _, receipt := range []map[string]any{first, second} {
		serialized, _ := json.Marshal(receipt)
		if strings.Contains(string(serialized), token) {
			t.Fatal("deployment receipt leaked admin token")
		}
	}
}

func TestDeployPreviewRejectsMissingProfile(t *testing.T) {
	project := t.TempDir()
	writeTestFile(t, project, "dist/index.html", `<!doctype html><html></html>`)
	_, err := packagePage(input{
		ProjectRoot: project,
		SourceRoot:  "dist",
		OutputZip:   "artifacts/page.zip",
		PageKey:     "page-demo",
		Version:     "1.0.0",
		DisplayMode: "embedded",
		Force:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = deployPreview(input{ProjectRoot: project, ArtifactZip: "artifacts/page.zip", Profile: "missing"})
	if err == nil || !strings.Contains(err.Error(), "未配置部署 Profile") {
		t.Fatalf("expected missing profile error, got %v", err)
	}
}

func TestLoadDeployProfileRejectsUnsafeBaseURL(t *testing.T) {
	t.Setenv("ANDX_PAGE_PROFILE_BAD", `{"base_url":"http://example.com","admin_token":"x","tenant_id":"1"}`)
	_, _, err := loadDeployProfile("bad")
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("expected HTTPS error, got %v", err)
	}
}

func TestInspectArtifactArchiveReadsManifest(t *testing.T) {
	project := t.TempDir()
	writeTestFile(t, project, "dist/index.html", `<!doctype html><html></html>`)
	_, err := packagePage(input{
		ProjectRoot: project,
		SourceRoot:  "dist",
		OutputZip:   "page.zip",
		PageKey:     "page-demo",
		Version:     "1.0.0",
		DisplayMode: "immersive",
		Force:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := inspectArtifactArchive(filepath.Join(project, "page.zip"))
	if err != nil {
		t.Fatal(err)
	}
	if artifact.PageKey != "page-demo" || artifact.Version != "1.0.0" || artifact.DisplayMode != "immersive" {
		t.Fatalf("unexpected artifact: %#v", artifact)
	}
	if _, err := os.Stat(artifact.Path); err != nil {
		t.Fatal(err)
	}
}
