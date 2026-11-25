package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoFrontendBuildChainInPlugin(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{"npm run build", "pnpm build", "vite build", "webpack build"}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || entry.Name() == "boundary_test.go" {
			continue
		}
		body, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(string(body))
		for _, token := range forbidden {
			if strings.Contains(lower, token) {
				t.Fatalf("plugin contains frontend build command %q in %s", token, entry.Name())
			}
		}
	}
	var manifest struct {
		Capabilities []struct {
			ID string `json:"id"`
		} `json:"capabilities"`
	}
	body, err := os.ReadFile("plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, capability := range manifest.Capabilities {
		if strings.HasSuffix(capability.ID, ".build") {
			t.Fatalf("plugin must not expose a frontend build capability: %s", capability.ID)
		}
	}
}

func TestFixedBridgeHarnessHasKnownLocalRiskAndAccessCapabilitiesBelongToPlugin(t *testing.T) {
	var manifest struct {
		Capabilities []struct {
			ID   string `json:"id"`
			Risk string `json:"risk_level"`
		} `json:"capabilities"`
	}
	body, err := os.ReadFile("plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, capability := range manifest.Capabilities {
		if capability.ID == "andx.page.test.run" {
			found = true
			if capability.Risk != "local_action" {
				t.Fatal("fixed harness must declare known local risk without a core business exception")
			}
		}
	}
	if !found {
		t.Fatal("missing Bridge test capability")
	}
	for _, skill := range []string{"andx-custom-page-development", "andx-platform-integration"} {
		var metadata struct {
			Capabilities []struct {
				ID       string `json:"id"`
				Provider string `json:"provider"`
			} `json:"capabilities"`
		}
		body, err := os.ReadFile(filepath.Join("..", "..", "skills", skill, "skill.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &metadata); err != nil {
			t.Fatal(err)
		}
		for _, capability := range metadata.Capabilities {
			if strings.HasPrefix(capability.ID, "andx.page.") && capability.Provider != "com.himind.andx-custom-page-tools" {
				t.Fatalf("business capability %s points at core provider", capability.ID)
			}
		}
	}
}

func TestSkillAndWorkflowReuseMyriadBoxWebProvider(t *testing.T) {
	skillPath := filepath.Join("..", "..", "skills", "andx-custom-page-development", "SKILL.md")
	skill, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	skillText := string(skill)
	for _, expected := range []string{"exhibit.web.build", "构建回执实际返回的静态目录", "不额外创建第二套"} {
		if !strings.Contains(skillText, expected) {
			t.Fatalf("skill is missing build-provider boundary %q", expected)
		}
	}
	workflowPath := filepath.Join("..", "..", "workflows", "andx-custom-page-delivery", "workflow.json")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	workflowText := string(workflow)
	for _, expected := range []string{"\"capability_id\": \"exhibit.web.build\"", "\"target\": \"web\"", "\"source_root\": \"input.source_root\""} {
		if !strings.Contains(workflowText, expected) {
			t.Fatalf("workflow is missing build-provider boundary %q", expected)
		}
	}
}

func TestWorkflowUsesReceiptsAndStopsAtPreviewByDefault(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "workflows", "andx-custom-page-delivery", "workflow.json"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		DefaultExit string `json:"default_exitpoint"`
		Entrypoints []struct {
			ID string `json:"id"`
			At string `json:"at_step"`
		} `json:"entrypoints"`
		Steps []struct {
			ID       string         `json:"id"`
			Depends  []string       `json:"depends_on"`
			Input    map[string]any `json:"input"`
			Approval bool           `json:"approval_required"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(body, &workflow); err != nil {
		t.Fatal(err)
	}
	if workflow.DefaultExit != "preview_ready" {
		t.Fatal("a default launch must stop before production")
	}
	for _, entry := range workflow.Entrypoints {
		if entry.At != "ACP-BINDING" {
			t.Fatal("every entry must resolve the workspace target")
		}
	}
	steps := map[string]map[string]any{}
	for _, step := range workflow.Steps {
		steps[step.ID] = step.Input
		if step.ID == "ACP-ACTIVATE" && !step.Approval {
			t.Fatal("activation needs exact-artifact confirmation")
		}
	}
	for _, id := range []string{"ACP-CONTRACT", "ACP-PACKAGE"} {
		if steps[id]["source_root"] != "steps.ACP-PROJECT-INSPECT.source_root" {
			t.Fatalf("%s must consume inspected build output", id)
		}
	}
	if steps["ACP-PROJECT-INSPECT"]["build_receipt"] != "steps.ACP-MYRIADBOX-BUILD" || steps["ACP-ACTIVATE"]["expected_activation_id"] != "steps.ACP-PREVIEW.expected_activation_id" {
		t.Fatal("broken receipt chain")
	}
}

func TestInspectUsesActualMyriadBoxBuildReceipt(t *testing.T) {
	project := t.TempDir()
	writeTestFile(t, project, "artifacts/custom-depth/wwwroot/index.html", "<!doctype html><html><body>test</body></html>")
	receipt := map[string]any{"ok": true, "target": "web", "output": filepath.Join(project, "artifacts/custom-depth"), "artifact": map[string]any{"entrypoint": "wwwroot/index.html"}}
	result, err := inspectSource(input{ProjectRoot: project, SourceRoot: "wrong-manual-path", BuildReceipt: receipt})
	if err != nil {
		t.Fatal(err)
	}
	expected, _ := filepath.EvalSymlinks(filepath.Join(project, "artifacts/custom-depth/wwwroot"))
	if result["source_root"] != expected {
		t.Fatal("ignored build receipt")
	}
	receipt["ok"] = false
	if _, err := inspectSource(input{ProjectRoot: project, BuildReceipt: receipt}); err == nil {
		t.Fatal("failed build accepted")
	}
	receipt["ok"] = true
	receipt["output"] = t.TempDir()
	if _, err := inspectSource(input{ProjectRoot: project, BuildReceipt: receipt}); err == nil {
		t.Fatal("outside-project receipt accepted")
	}
}
