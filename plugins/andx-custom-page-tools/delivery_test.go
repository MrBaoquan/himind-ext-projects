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

func TestActivationIsIdempotentAndChecksHealth(t *testing.T) {
	token := "activation-secret"
	var activated atomic.Bool
	var activateCalls atomic.Int32
	deploymentID := "deployment-1"
	sha := strings.Repeat("a", 64)
	deployment := func() map[string]any {
		return map[string]any{
			"id": deploymentID, "tenantId": "1", "pageKey": "page-demo", "version": "1.0.0",
			"displayMode": "embedded", "entryFile": "index.html", "bridgeVersion": "1.0",
			"url":            "https://andx.example.com/interaction-pages/1/page-demo/1.0.0/index.html",
			"artifactSha256": sha, "artifactBytes": 100, "fileCount": 2, "status": "READY",
			"activatedAt": nil, "createdAt": "2026-11-06T00:00:00Z",
		}
	}
	activation := func() map[string]any {
		return map[string]any{
			"id": "activation-1", "tenantId": "1", "scope": "exhibit", "exhibitId": "9",
			"pageKey": "page-demo", "deploymentId": deploymentID, "version": "1.0.0",
			"bridgeVersion": "1.0", "action": "ACTIVATE", "previousActivationId": nil,
			"previousDeploymentId": nil, "previousVersion": nil, "activatedBy": "admin-1",
			"reason": "approved", "createdAt": "2026-11-06T00:00:01Z",
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("authorization") != "Bearer "+token {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		writer.Header().Set("content-type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/admin/custom-pages/"+deploymentID:
			_ = json.NewEncoder(writer).Encode(map[string]any{"code": 0, "message": "ok", "data": deployment()})
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/health-check"):
			_ = json.NewEncoder(writer).Encode(map[string]any{"code": 0, "message": "ok", "data": map[string]any{"ok": true, "url": "https://andx.example.com/page", "statusCode": 200, "latencyMs": 1, "checkedAt": "now", "message": "ok"}})
		case request.Method == http.MethodGet && request.URL.Path == "/api/admin/custom-pages/activations":
			list := []any{}
			if activated.Load() {
				list = append(list, activation())
			}
			_ = json.NewEncoder(writer).Encode(map[string]any{"code": 0, "message": "ok", "data": map[string]any{"list": list}})
		case request.Method == http.MethodPost && strings.HasSuffix(request.URL.Path, "/activate"):
			activateCalls.Add(1)
			activated.Store(true)
			_ = json.NewEncoder(writer).Encode(map[string]any{"code": 0, "message": "ok", "data": deployment()})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	t.Setenv("ANDX_PAGE_PROFILE_TEST", fmt.Sprintf(`{"base_url":%q,"admin_token":%q,"tenant_id":"1"}`, server.URL, token))
	in := input{Profile: "test", DeploymentID: deploymentID, Scope: "exhibit", ExhibitID: "9", ExpectedSHA256: sha, Reason: "approved"}
	first, err := activateDeployment(in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := activateDeployment(in)
	if err != nil {
		t.Fatal(err)
	}
	if first["idempotent"] != false || second["idempotent"] != true {
		t.Fatalf("unexpected idempotency: first=%#v second=%#v", first["idempotent"], second["idempotent"])
	}
	if activateCalls.Load() != 1 {
		t.Fatalf("expected one activation call, got %d", activateCalls.Load())
	}
	for _, receipt := range []map[string]any{first, second} {
		body, _ := json.Marshal(receipt)
		if strings.Contains(string(body), token) {
			t.Fatal("activation receipt leaked token")
		}
	}
}

func TestActivationRejectsWrongSHA(t *testing.T) {
	token := "secret"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("authorization") != "Bearer "+token {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		writer.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{"code": 0, "message": "ok", "data": map[string]any{
			"id": "deployment-1", "tenantId": "1", "pageKey": "page-demo", "version": "1.0.0",
			"displayMode": "embedded", "entryFile": "index.html", "bridgeVersion": "1.0",
			"url": "https://andx.example.com/page", "artifactSha256": strings.Repeat("b", 64),
			"artifactBytes": 1, "fileCount": 2, "status": "READY", "activatedAt": nil, "createdAt": "now",
		}})
	}))
	defer server.Close()
	t.Setenv("ANDX_PAGE_PROFILE_TEST", fmt.Sprintf(`{"base_url":%q,"admin_token":%q,"tenant_id":"1"}`, server.URL, token))
	_, err := activateDeployment(input{Profile: "test", DeploymentID: "deployment-1", Scope: "tenant", ExpectedSHA256: strings.Repeat("a", 64), Reason: "approved"})
	if err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("expected SHA rejection, got %v", err)
	}
}

func TestExportInteractionQRWritesStableEntryCode(t *testing.T) {
	token := "qr-secret"
	png := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9Zl1sAAAAASUVORK5CYII="
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("authorization") != "Bearer "+token {
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}
		writer.Header().Set("content-type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{"code": 0, "message": "ok", "data": map[string]any{
			"available": true, "dataUrl": "data:image/png;base64," + png, "url": "https://andx.example.com/x/P1-E9",
			"reason": nil, "scene": "P1-E9", "projectCode": "P1", "exhibitCode": "E9",
		}})
	}))
	defer server.Close()
	project := t.TempDir()
	t.Setenv("ANDX_PAGE_PROFILE_TEST", fmt.Sprintf(`{"base_url":%q,"admin_token":%q,"tenant_id":"1"}`, server.URL, token))
	result, err := exportInteractionQR(input{ProjectRoot: project, Profile: "test", ProjectID: "1", ExhibitID: "9", OutputPNG: "artifacts/qr.png"})
	if err != nil {
		t.Fatal(err)
	}
	if result["url"] != "https://andx.example.com/x/P1-E9" {
		t.Fatalf("unexpected QR URL: %#v", result["url"])
	}
	if _, err := os.Stat(filepath.Join(project, "artifacts", "qr.png")); err != nil {
		t.Fatal(err)
	}
}

func TestPackEvidenceIncludesArtifactAndQR(t *testing.T) {
	project := t.TempDir()
	writeTestFile(t, project, "dist/index.html", `<!doctype html><html></html>`)
	_, err := packagePage(input{ProjectRoot: project, SourceRoot: "dist", OutputZip: "artifacts/page.zip", PageKey: "page-demo", Version: "1.0.0", DisplayMode: "embedded", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, project, "artifacts/qr.png", "png")
	result, err := packEvidence(input{WorkspaceRoot: project, ArtifactZip: "artifacts/page.zip", QRPNG: "artifacts/qr.png", OutputJSON: "artifacts/evidence.json"})
	if err != nil {
		t.Fatal(err)
	}
	if result["artifact_count"] != 2 {
		t.Fatalf("unexpected artifact count: %#v", result["artifact_count"])
	}
	body, err := os.ReadFile(filepath.Join(project, "artifacts", "evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "andx-custom-page-evidence-manifest/v1") {
		t.Fatal("evidence manifest schema missing")
	}
}
