package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseDocxExtractsParagraphsAndTables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "script.docx")
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	file, err := writer.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	xml := `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>展项标题</w:t></w:r></w:p><w:tbl><w:tr><w:tc><w:p><w:r><w:t>界面</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>互动</w:t></w:r></w:p></w:tc></w:tr></w:tbl></w:body></w:document>`
	if _, err := file.Write([]byte(xml)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := parseScript(path)
	if err != nil {
		t.Fatal(err)
	}
	blocks, ok := result["blocks"].([]documentBlock)
	if !ok || len(blocks) != 2 {
		t.Fatalf("blocks = %#v", result["blocks"])
	}
	if blocks[0].Text != "展项标题" || blocks[1].Type != "table" {
		t.Fatalf("unexpected blocks: %#v", blocks)
	}
}

func TestValidateSpecRejectsEmptyRequirements(t *testing.T) {
	result, err := validateSpec(input{ContentJSON: map[string]any{"schema_version": "exhibit-development-spec.v1", "requirements": []any{}}})
	if err != nil {
		t.Fatal(err)
	}
	if result["valid"] != false {
		t.Fatalf("expected invalid result, got %#v", result)
	}
}

func TestInspectProjectDetectsUnityAndUnihper(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"Assets/Resources/UNIHper", "Packages", "ProjectSettings"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(name)), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("Packages/manifest.json", `{"dependencies":{}}`)
	write("ProjectSettings/ProjectVersion.txt", "m_EditorVersion: 2022.3.62f1\n")
	write("Assets/Resources/UNIHper/assemblies.json", `{"assemblies":[]}`)
	write("Assets/Resources/UNIHper/resources.json", `{"groups":[]}`)
	result, err := inspectProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if result["is_unity"] != true || result["unihper"] != true || result["unity_version"] != "2022.3.62f1" {
		encoded, _ := json.Marshal(result)
		t.Fatalf("unexpected inspection: %s", encoded)
	}
}

func TestPackEvidenceHashAndPathContainment(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "shot.png")
	if err := os.WriteFile(path, []byte("png-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := packEvidence(root, []evidenceItem{{Path: "shot.png", ArtifactKind: "development-screenshot", RequirementIDs: []string{"REQ-001"}}})
	if err != nil {
		t.Fatal(err)
	}
	if result["artifact_count"] != 1 {
		t.Fatalf("unexpected manifest: %#v", result)
	}
	if _, err := packEvidence(root, []evidenceItem{{Path: "../outside.png", ArtifactKind: "development-screenshot"}}); err == nil {
		t.Fatal("expected path escape to fail")
	}
}
