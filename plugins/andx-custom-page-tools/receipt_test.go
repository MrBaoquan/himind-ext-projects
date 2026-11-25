package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowReceiptPersistsExactPayloadAndDigest(t *testing.T) {
	root := t.TempDir()
	payload := map[string]any{"schema_version": "andx-custom-page-spec.v1", "title": "欢迎屏"}
	result, err := saveWorkflowReceipt(input{ProjectRoot: root, ReceiptArtifactID: "custom-page-spec", ReceiptData: payload})
	if err != nil {
		t.Fatal(err)
	}
	artifact := result["artifacts"].([]map[string]any)[0]
	u, err := url.Parse(artifact["uri"].(string))
	if err != nil {
		t.Fatal(err)
	}
	p := u.Path
	if len(p) > 2 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	body, err := os.ReadFile(filepath.FromSlash(p))
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	if artifact["sha256"] != hex.EncodeToString(digest[:]) || artifact["size_bytes"] != len(body) {
		t.Fatal("receipt identity mismatch")
	}
	var restored map[string]any
	if json.Unmarshal(body, &restored) != nil || restored["title"] != "欢迎屏" {
		t.Fatal("receipt payload changed")
	}
	if _, err := saveWorkflowReceipt(input{ProjectRoot: root, ReceiptArtifactID: "deployment-receipt", ReceiptData: payload}); err == nil {
		t.Fatal("fabricated publication receipt accepted")
	}
	payload["secret"] = "andx_dev_" + strings.Repeat("x", 43)
	if _, err := saveWorkflowReceipt(input{ProjectRoot: root, ReceiptArtifactID: "custom-page-spec", ReceiptData: payload}); err == nil {
		t.Fatal("secret receipt accepted")
	}
}

func TestBridgeSDKDiscoveryUsesNearestRealPackage(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "node_modules/@mrbaoquan/andx/dist/index.js", "export {}")
	project := filepath.Join(root, "projects", "welcome")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	_, sdk, _, err := resolveBridgeTestPaths(input{ProjectRoot: project, OutputDir: "artifacts/bridge"})
	if err != nil {
		t.Fatal(err)
	}
	expected, _ := filepath.EvalSymlinks(filepath.Join(root, "node_modules", "@mrbaoquan", "andx"))
	if sdk != expected {
		t.Fatal("SDK was not resolved from parent node_modules")
	}
}
