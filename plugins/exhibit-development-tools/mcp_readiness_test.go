package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVersionFromPackageRef(t *testing.T) {
	cases := map[string]string{
		"10.2.0": "10.2.0",
		"v10.2.0": "10.2.0",
		"https://github.com/CoplayDev/unity-mcp.git?path=/MCPForUnity#v10.2.0": "10.2.0",
		"https://github.com/CoplayDev/unity-mcp.git?path=/MCPForUnity#main":    "",
		"https://github.com/CoplayDev/unity-mcp.git?path=/MCPForUnity#30d2207": "",
		"": "",
	}
	for input, want := range cases {
		if got := versionFromPackageRef(input); got != want {
			t.Errorf("versionFromPackageRef(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLooksLikeSemver(t *testing.T) {
	for _, s := range []string{"10.2.0", "1.2", "1.2.3.4"} {
		if !looksLikeSemver(s) {
			t.Errorf("looksLikeSemver(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "main", "30d2207", "10", "1.2.x"} {
		if looksLikeSemver(s) {
			t.Errorf("looksLikeSemver(%q) = true, want false", s)
		}
	}
}

func TestInspectUnityMCPWithoutHeartbeat(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("UNITY_MCP_STATUS_DIR", filepath.Join(dir, "status-missing"))

	projectRoot := filepath.Join(dir, "proj")
	if err := os.MkdirAll(filepath.Join(projectRoot, "Assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "Packages"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"dependencies":{"com.coplaydev.unity-mcp":"https://x/y.git?path=/MCPForUnity#v10.2.0"}}`
	if err := os.WriteFile(filepath.Join(projectRoot, "Packages", "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	status := inspectUnityMCP(projectRoot, "2023.1.0f1")
	if status.Ready {
		t.Fatal("expected not ready without heartbeat")
	}
	if status.State != "not_running" {
		t.Fatalf("state = %q, want not_running", status.State)
	}
	if status.PackageVersion != "10.2.0" {
		t.Fatalf("package version = %q, want 10.2.0", status.PackageVersion)
	}
}
