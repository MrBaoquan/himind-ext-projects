package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMyriadBoxAdapterRemovesOnlyGeneratedStartupAndPreservesSource(t *testing.T) {
	root := t.TempDir()
	html := `<!doctype html><html><head><script>window.__MYRIADBOX_STARTUP__={"name":"欢迎屏","explicitReady":false,"timeoutSeconds":30,"motion":"system"};</script><script src="../sdk/brand/startup.js"></script></head><body><script src="./app.js"></script></body></html>`
	writeTestFile(t, root, "web/wwwroot/index.html", html)
	writeTestFile(t, root, "web/wwwroot/app.js", `document.body.dataset.ready='true'`)
	receipt := map[string]any{"ok": true, "target": "web", "output": filepath.Join(root, "web"), "artifact": map[string]any{"entrypoint": "wwwroot/index.html"}}
	result, err := inspectSource(input{ProjectRoot: root, BuildReceipt: receipt})
	if err != nil {
		t.Fatal(err)
	}
	adapted := result["source_root"].(string)
	body, err := os.ReadFile(filepath.Join(adapted, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "__MYRIADBOX_STARTUP__") || !strings.Contains(string(body), `src="./app.js"`) {
		t.Fatal("adapter changed business HTML")
	}
	original, _ := os.ReadFile(filepath.Join(root, "web/wwwroot/index.html"))
	if string(original) != html {
		t.Fatal("source build mutated")
	}
	if _, err := inspectSource(input{ProjectRoot: root, BuildReceipt: receipt}); err != nil {
		t.Fatal("idempotent adapter failed", err)
	}
	writeTestFile(t, root, "web/wwwroot/app.js", `window.MyriadBox.ready()`)
	if _, err := inspectSource(input{ProjectRoot: root, BuildReceipt: receipt}); err == nil {
		t.Fatal("host dependent business accepted")
	}
}
