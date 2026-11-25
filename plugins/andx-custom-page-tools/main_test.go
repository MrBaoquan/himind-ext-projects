package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPackageStaticDist(t *testing.T) {
	project := t.TempDir()
	writeTestFile(t, project, "dist/index.html", `<!doctype html><html><body><main id="app"></main><script src="./assets/app.js"></script></body></html>`)
	writeTestFile(t, project, "dist/assets/app.js", `document.getElementById("app").textContent = "ready";`)

	result, err := packagePage(input{
		ProjectRoot:          project,
		SourceRoot:           "dist",
		OutputZip:            "artifacts/page-1.0.0.zip",
		PageKey:              "page-demo",
		Version:              "1.0.0",
		DisplayMode:          "embedded",
		RequiredCapabilities: []string{"andx.payment.request"},
		OptionalCapabilities: []string{"andx.status.read"},
		BuiltAt:              "2026-11-06T00:00:00Z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result["ok"] != true {
		t.Fatalf("unexpected result: %#v", result)
	}
	outputZip, _ := result["output_zip"].(string)
	entries := readZipEntries(t, outputZip)
	if _, ok := entries["index.html"]; !ok {
		t.Fatal("index.html missing from ZIP")
	}
	if _, ok := entries["assets/app.js"]; !ok {
		t.Fatal("asset missing from ZIP")
	}
	var manifest pageManifest
	if err := json.Unmarshal(entries["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.PageKey != "page-demo" || manifest.Version != "1.0.0" {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	if len(manifest.Files) != 2 {
		t.Fatalf("expected 2 manifest files, got %d", len(manifest.Files))
	}
	if manifest.Bridge.Capabilities.Required[0] != "andx.payment.request" {
		t.Fatalf("unexpected required capabilities: %#v", manifest.Bridge.Capabilities.Required)
	}
	if result["sha256"] == "" {
		t.Fatal("sha256 is empty")
	}
}

func TestPackageMyriadBoxWebRootSkipsRuntime(t *testing.T) {
	project := t.TempDir()
	writeTestFile(t, project, "artifacts/demo/web/wwwroot/index.html", `<!doctype html><html><body><script src="./assets/app.js"></script></body></html>`)
	writeTestFile(t, project, "artifacts/demo/web/wwwroot/assets/app.js", `console.log("andx page")`)
	writeTestFile(t, project, "artifacts/demo/web/wwwroot/myriadbox.runtime.json", `{"platform":"web"}`)
	writeTestFile(t, project, "artifacts/demo/web/wwwroot/sdk/myriadbox.js", `window.MyriadBox = {};`)
	writeTestFile(t, project, "artifacts/demo/web/wwwroot/sdk/myriadbox.d.ts", `export {};`)

	result, err := packagePage(input{
		ProjectRoot: project,
		SourceRoot:  "artifacts/demo/web/wwwroot",
		OutputZip:   "artifacts/demo-page.zip",
		PageKey:     "page-demo",
		Version:     "2.0.0",
		DisplayMode: "immersive",
		Force:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	entries := readZipEntries(t, result["output_zip"].(string))
	for _, path := range []string{"myriadbox.runtime.json", "sdk/myriadbox.js", "sdk/myriadbox.d.ts"} {
		if _, exists := entries[path]; exists {
			t.Fatalf("MyriadBox runtime file must be excluded: %s", path)
		}
	}
	if result["source_kind"] != "myriadbox-web" {
		t.Fatalf("unexpected source kind: %#v", result["source_kind"])
	}
}

func TestPackageRejectsMyriadBoxRuntimeReference(t *testing.T) {
	project := t.TempDir()
	writeTestFile(t, project, "dist/index.html", `<!doctype html><html><body><script src="./sdk/myriadbox.js"></script></body></html>`)
	writeTestFile(t, project, "dist/sdk/myriadbox.js", `window.MyriadBox = {};`)
	_, err := packagePage(input{
		ProjectRoot: project,
		SourceRoot:  "dist",
		OutputZip:   "page.zip",
		PageKey:     "page-demo",
		Version:     "1.0.0",
		DisplayMode: "embedded",
	})
	if err == nil || !strings.Contains(err.Error(), "MyriadBox 运行时") {
		t.Fatalf("expected MyriadBox runtime rejection, got %v", err)
	}
}

func TestContractValidateReportsUnsupportedCapability(t *testing.T) {
	project := t.TempDir()
	writeTestFile(t, project, "dist/index.html", `<!doctype html><html><body><script src="./app.js"></script></body></html>`)
	writeTestFile(t, project, "dist/app.js", `console.log("ok")`)
	_, err := validatePageInput(input{
		ProjectRoot:          project,
		SourceRoot:           "dist",
		PageKey:              "page-demo",
		Version:              "1.0.0",
		DisplayMode:          "embedded",
		RequiredCapabilities: []string{"andx.unknown.capability"},
	})
	if err == nil || !strings.Contains(err.Error(), "不支持的 Bridge 能力") {
		t.Fatalf("expected unsupported capability error, got %v", err)
	}
}

func TestPackageRejectsInlineScriptAndRootAbsoluteAsset(t *testing.T) {
	project := t.TempDir()
	writeTestFile(t, project, "dist/index.html", `<!doctype html><html><body><script>alert(1)</script><img src="/assets/a.png"></body></html>`)
	writeTestFile(t, project, "dist/assets/a.png", string([]byte{0x89, 0x50, 0x4e, 0x47}))
	_, err := packagePage(input{
		ProjectRoot: project,
		SourceRoot:  "dist",
		OutputZip:   "page.zip",
		PageKey:     "page-demo",
		Version:     "1.0.0",
		DisplayMode: "embedded",
	})
	if err == nil || !strings.Contains(err.Error(), "内联 script") || !strings.Contains(err.Error(), "根绝对资源路径") {
		t.Fatalf("expected inline script and root path errors, got %v", err)
	}
}

func TestPackageRejectsSourceOutsideProject(t *testing.T) {
	project := t.TempDir()
	outside := t.TempDir()
	writeTestFile(t, outside, "dist/index.html", `<!doctype html><html></html>`)
	_, err := packagePage(input{
		ProjectRoot: project,
		SourceRoot:  outside,
		OutputZip:   "page.zip",
		PageKey:     "page-demo",
		Version:     "1.0.0",
		DisplayMode: "embedded",
	})
	if err == nil || !strings.Contains(err.Error(), "source_root 必须位于 project_root 内") {
		t.Fatalf("expected source boundary error, got %v", err)
	}
}

func writeTestFile(t *testing.T, root string, relative string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readZipEntries(t *testing.T, path string) map[string][]byte {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	result := map[string][]byte{}
	for _, entry := range reader.File {
		file, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
		result[entry.Name] = bytes.Clone(body)
	}
	return result
}
func TestPackageIsDeterministic(t *testing.T) {
	project := t.TempDir()
	writeTestFile(t, project, "dist/index.html", `<!doctype html><html><body><script src="./app.js"></script></body></html>`)
	writeTestFile(t, project, "dist/app.js", `console.log("stable")`)
	in := input{
		ProjectRoot: project,
		SourceRoot:  "dist",
		OutputZip:   "page.zip",
		PageKey:     "page-demo",
		Version:     "1.0.0",
		DisplayMode: "embedded",
		BuiltAt:     "2026-11-06T00:00:00Z",
		Force:       true,
	}
	first, err := packagePage(in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := packagePage(in)
	if err != nil {
		t.Fatal(err)
	}
	if first["sha256"] != second["sha256"] {
		t.Fatalf("package is not deterministic: %v != %v", first["sha256"], second["sha256"])
	}
}
func TestRunBridgeFixtureIntegration(t *testing.T) {
	sdkRoot := os.Getenv("ANDX_WEB_SDK_ROOT")
	if sdkRoot == "" {
		t.Skip("set ANDX_WEB_SDK_ROOT to run the Bridge fixture browser test")
	}
	project := t.TempDir()
	result, err := runBridgeFixture(input{
		ProjectRoot:    project,
		SdkRoot:        sdkRoot,
		OutputDir:      "artifacts/bridge-test",
		ChromePath:     os.Getenv("ANDX_CHROME_PATH"),
		TimeoutSeconds: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result["ok"] != true {
		t.Fatalf("bridge fixture failed: %#v", result)
	}
}

func TestBridgeFixtureDeadlineReturnsReportAndStopsBrowser(t *testing.T) {
	sdkRoot := os.Getenv("ANDX_WEB_SDK_ROOT")
	if sdkRoot == "" {
		t.Skip("set ANDX_WEB_SDK_ROOT to run the Bridge deadline browser test")
	}
	project := t.TempDir()
	// A valid SDK-shaped module that never completes the handshake.
	stalledSDK := filepath.Join(project, "stalled-sdk")
	writeTestFile(t, project, "stalled-sdk/dist/index.js", `export function createInteractionHost(){return {start(){}}}`)
	started := time.Now()
	result, err := runBridgeFixture(input{
		ProjectRoot: project, SdkRoot: stalledSDK, OutputDir: "artifacts/deadline",
		ChromePath: os.Getenv("ANDX_CHROME_PATH"), TimeoutSeconds: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result["ok"] != false || result["error"] != "Bridge test deadline exceeded" {
		t.Fatalf("deadline must return a diagnostic failure report: %#v", result)
	}
	if time.Since(started) > 10*time.Second {
		t.Fatal("deadline cleanup did not complete within its grace period")
	}
	if _, err := os.Stat(filepath.Join(project, "artifacts/deadline/bridge-report.json")); err != nil {
		t.Fatal("deadline report was not saved")
	}
}
