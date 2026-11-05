package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectWebProjectDetectsVueAndProvider(t *testing.T) {
	root := t.TempDir()
	mustWrite := func(name, content string) {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("exhibit.json", `{
		"schema":"exhibit-shell/v1",
		"id":"demo",
		"name":"demo",
		"entry":"wwwroot/index.html",
		"build":{"command":"npm run build"},
		"config":{"file":"config/exhibit.config.json","schema":"config/exhibit.schema.json"},
		"media":{"root":"media"},
		"providers":[{"id":"iotoolkit","type":"iotoolkit","executable":"providers/bridge.exe","port":19101}]
	}`)
	mustWrite("wwwroot/index.html", "<html></html>")
	mustWrite("config/exhibit.config.json", "{}")
	mustWrite("config/exhibit.schema.json", "{}")
	mustWrite("media/README.md", "media")
	mustWrite("package.json", `{"dependencies":{"vue":"^3.5.0"}}`)

	result, err := inspectWebProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if result["framework"] != "vue" || result["entry_exists"] != true {
		encoded, _ := json.Marshal(result)
		t.Fatalf("unexpected inspection: %s", encoded)
	}
	providers, ok := result["providers"].([]map[string]any)
	if !ok || len(providers) != 1 || providers[0]["exists"] != false {
		t.Fatalf("unexpected providers: %#v", result["providers"])
	}
}

func TestInspectWebProjectAcceptsUtf8Bom(t *testing.T) {
	root := t.TempDir()
	manifest := []byte("\uFEFF" + `{"schema":"exhibit-shell/v1","id":"bom","name":"bom","entry":"wwwroot/index.html"}`)
	if err := os.MkdirAll(filepath.Join(root, "wwwroot"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "exhibit.json"), manifest, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "wwwroot", "index.html"), []byte("<html></html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := inspectWebProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if result["manifest"].(map[string]any)["id"] != "bom" {
		t.Fatalf("unexpected manifest: %#v", result["manifest"])
	}
}

func TestReadProjectLogsReturnsNewestTail(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "logs", "MyriadBox.log")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := readProjectLogs(input{ProjectRoot: root, TailLines: 2})
	if err != nil {
		t.Fatal(err)
	}
	lines := result["lines"].([]string)
	if len(lines) != 2 || lines[1] != "three" {
		t.Fatalf("unexpected tail: %#v", lines)
	}
}

func TestAppendBuildArgsIncludesMultiTargetOptions(t *testing.T) {
	args := appendBuildArgs([]string{"build", "demo"}, input{
		Target:              "all",
		OutputDir:           "artifacts/final",
		CorePath:            "core",
		LauncherPath:        "MyriadBox.exe",
		IoDeviceRoot:        "iodevice",
		BridgeTransport:     "http",
		BridgeEndpoint:      "http://127.0.0.1:19101",
		AndroidWebViewDebug: true,
	})
	joined := strings.Join(args, " ")
	for _, expected := range []string{"--target all", "--output artifacts/final", "--core core", "--launcher MyriadBox.exe", "--iodevice-root iodevice", "--bridge-transport http", "--bridge-endpoint http://127.0.0.1:19101", "--android-webview-debug"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %q in %q", expected, joined)
		}
	}
}

func TestVerifyAPKDetectsArm64AndExhibitPayload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.apk")
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, name := range []string{
		"AndroidManifest.xml",
		"classes.dex",
		"lib/arm64-v8a/iodevice.so",
		"assets/exhibit/myriadbox.runtime.json",
		"assets/exhibit/target.manifest.json",
	} {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte("test")); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := verifyAPK(input{ApkPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if result["ok"] != true || result["has_arm64_iodevice"] != true || result["has_exhibit_runtime"] != true {
		encoded, _ := json.Marshal(result)
		t.Fatalf("unexpected APK verification: %s", encoded)
	}
}
