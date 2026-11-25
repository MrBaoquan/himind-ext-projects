package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var workflowReceiptTypes = map[string]string{
	"custom-page-spec":      "custom_page_spec",
	"bridge-test-report":    "custom_page_bridge_test",
	"custom-page-candidate": "custom_page_candidate",
	"deployment-receipt":    "custom_page_deployment_receipt",
	"acceptance-evidence":   "custom_page_acceptance_evidence",
	"evidence-manifest":     "custom_page_evidence_manifest",
}

func saveWorkflowReceipt(in input) (map[string]any, error) {
	if in.ReceiptArtifactID != "custom-page-spec" && in.ReceiptArtifactID != "acceptance-evidence" {
		return nil, errors.New("只允许保存规格或候选验收报告，发布回执必须来自真实 API")
	}
	if in.ReceiptData == nil {
		return nil, errors.New("缺少结构化报告")
	}
	return attachWorkflowReceipt(map[string]any{"ok": true}, in, in.ReceiptArtifactID, in.ReceiptData)
}

// Receipts are content-addressed files. The runner validates the actual file
// against its declared schema and hash before accepting the artifact.
func attachWorkflowReceipt(result map[string]any, in input, id string, payload any) (map[string]any, error) {
	root := in.ProjectRoot
	if root == "" {
		root = in.WorkspaceRoot
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	if len(body) > 1024*1024 || strings.Contains(string(body), "andx_dev_") {
		return nil, errors.New("回执超限或包含敏感凭证")
	}
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	dir := filepath.Join(root, "artifacts", "andx-receipts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil || !pathWithin(root, dir) {
		return nil, errors.New("回执目录必须位于当前工程内")
	}
	path := filepath.Join(dir, id+"-"+digest+".json")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if os.IsExist(err) {
		existing, readErr := os.ReadFile(path)
		if readErr != nil || string(existing) != string(body) {
			return nil, errors.New("回执文件冲突")
		}
	} else if err != nil {
		return nil, err
	} else {
		_, writeErr := file.Write(body)
		closeErr := file.Close()
		if writeErr != nil {
			return nil, writeErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	portablePath := strings.TrimPrefix(filepath.ToSlash(path), "//?/")
	uri := (&url.URL{Scheme: "file", Path: "/" + strings.TrimPrefix(portablePath, "/")}).String()
	result["artifacts"] = []map[string]any{{"artifact_id": id, "artifact_type": workflowReceiptTypes[id], "name": id, "uri": uri, "sha256": digest, "size_bytes": len(body)}}
	return result, nil
}
