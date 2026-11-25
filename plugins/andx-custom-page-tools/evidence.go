package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type evidenceItemInput struct {
	Path           string         `json:"path"`
	ArtifactKind   string         `json:"artifact_kind"`
	RequirementIDs []string       `json:"requirement_ids"`
	Metadata       map[string]any `json:"metadata"`
}

type evidenceArtifact struct {
	Path           string         `json:"path"`
	RelativePath   string         `json:"relative_path"`
	ArtifactKind   string         `json:"artifact_kind"`
	FileName       string         `json:"file_name"`
	SizeBytes      int64          `json:"size_bytes"`
	SHA256         string         `json:"sha256"`
	RequirementIDs []string       `json:"requirement_ids,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type evidenceManifest struct {
	SchemaVersion string             `json:"schema_version"`
	GeneratedAt   string             `json:"generated_at"`
	WorkspaceRoot string             `json:"workspace_root"`
	ArtifactCount int                `json:"artifact_count"`
	TotalBytes    int64              `json:"total_bytes"`
	Artifacts     []evidenceArtifact `json:"artifacts"`
}

func packEvidence(in input) (map[string]any, error) {
	workspace := strings.TrimSpace(in.WorkspaceRoot)
	if workspace == "" {
		workspace = strings.TrimSpace(in.ProjectRoot)
	}
	if workspace == "" {
		return nil, errors.New("workspace_root is required")
	}
	workspaceAbs, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	workspaceAbs, err = filepath.EvalSymlinks(workspaceAbs)
	if err != nil {
		return nil, fmt.Errorf("workspace_root 不存在：%w", err)
	}
	outputJSON := strings.TrimSpace(in.OutputJSON)
	if outputJSON == "" {
		outputJSON = filepath.Join(workspaceAbs, "artifacts", "andx-page-evidence.json")
	} else if !filepath.IsAbs(outputJSON) {
		outputJSON = filepath.Join(workspaceAbs, outputJSON)
	}
	outputJSON, err = filepath.Abs(outputJSON)
	if err != nil {
		return nil, err
	}
	if !pathWithin(workspaceAbs, outputJSON) {
		return nil, errors.New("output_json 必须位于 workspace_root 内")
	}
	items := append([]evidenceItemInput{}, in.EvidenceItems...)
	if strings.TrimSpace(in.ArtifactZip) != "" {
		items = append(items, evidenceItemInput{Path: in.ArtifactZip, ArtifactKind: "build-package"})
	}
	if strings.TrimSpace(in.QRPNG) != "" {
		items = append(items, evidenceItemInput{Path: in.QRPNG, ArtifactKind: "delivery-evidence"})
	}
	if len(items) == 0 {
		return nil, errors.New("至少需要一个证据文件")
	}
	seen := map[string]struct{}{}
	artifacts := make([]evidenceArtifact, 0, len(items))
	var totalBytes int64
	for _, item := range items {
		path := strings.TrimSpace(item.Path)
		if path == "" {
			continue
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(workspaceAbs, path)
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		if !pathWithin(workspaceAbs, path) {
			return nil, fmt.Errorf("证据文件必须位于 workspace_root 内：%s", item.Path)
		}
		if _, duplicate := seen[path]; duplicate {
			continue
		}
		seen[path] = struct{}{}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("证据文件不存在：%s", item.Path)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(body)
		relative, err := filepath.Rel(workspaceAbs, path)
		if err != nil {
			return nil, err
		}
		kind := strings.TrimSpace(item.ArtifactKind)
		if kind == "" {
			kind = "other"
		}
		artifacts = append(artifacts, evidenceArtifact{
			Path:           filepath.ToSlash(relative),
			RelativePath:   filepath.ToSlash(relative),
			ArtifactKind:   kind,
			FileName:       filepath.Base(path),
			SizeBytes:      int64(len(body)),
			SHA256:         hex.EncodeToString(sum[:]),
			RequirementIDs: item.RequirementIDs,
			Metadata:       item.Metadata,
		})
		totalBytes += int64(len(body))
	}
	if len(artifacts) == 0 {
		return nil, errors.New("没有可打包的有效证据文件")
	}
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].RelativePath < artifacts[j].RelativePath })
	manifest := evidenceManifest{
		SchemaVersion: "andx-custom-page-evidence-manifest/v1",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		WorkspaceRoot: workspaceAbs,
		ArtifactCount: len(artifacts),
		TotalBytes:    totalBytes,
		Artifacts:     artifacts,
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	body = append(body, '\n')
	if err := os.MkdirAll(filepath.Dir(outputJSON), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(outputJSON, body, 0o644); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	return map[string]any{
		"ok":              true,
		"manifest_path":   outputJSON,
		"manifest_sha256": hex.EncodeToString(sum[:]),
		"artifact_count":  len(artifacts),
		"total_bytes":     totalBytes,
		"manifest":        manifest,
	}, nil
}
