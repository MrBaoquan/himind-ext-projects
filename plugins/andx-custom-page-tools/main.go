package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"himind-plugin/andx-custom-page-tools/internal/himindjsonrpc"
)

type input struct {
	Name                 string              `json:"name"`
	AfterID              string              `json:"after_id"`
	BindingRevision      string              `json:"binding_revision"`
	ReceiptData          map[string]any      `json:"data"`
	ReceiptArtifactID    string              `json:"artifact_id"`
	BuildReceipt         map[string]any      `json:"build_receipt"`
	PageActions          []string            `json:"page_actions"`
	CredentialRef        string              `json:"credential_ref"`
	ExpectedActivationID string              `json:"expected_activation_id"`
	Rollback             bool                `json:"rollback"`
	ProjectRoot          string              `json:"project_root"`
	SourceRoot           string              `json:"source_root"`
	OutputZip            string              `json:"output_zip"`
	PageKey              string              `json:"page_key"`
	Version              string              `json:"version"`
	DisplayMode          string              `json:"display_mode"`
	RequiredCapabilities []string            `json:"required_capabilities"`
	OptionalCapabilities []string            `json:"optional_capabilities"`
	BuiltAt              string              `json:"built_at"`
	Force                bool                `json:"force"`
	NodePath             string              `json:"node_path"`
	ChromePath           string              `json:"chrome_path"`
	SdkRoot              string              `json:"sdk_root"`
	OutputDir            string              `json:"output_dir"`
	TimeoutSeconds       int                 `json:"timeout_seconds"`
	Profile              string              `json:"profile"`
	ArtifactZip          string              `json:"artifact_zip"`
	ExpectedSHA256       string              `json:"expected_sha256"`
	Description          string              `json:"description"`
	Scope                string              `json:"scope"`
	DeploymentID         string              `json:"deployment_id"`
	Reason               string              `json:"reason"`
	ProjectID            string              `json:"project_id"`
	ExhibitID            string              `json:"exhibit_id"`
	OutputPNG            string              `json:"output_png"`
	WorkspaceRoot        string              `json:"workspace_root"`
	QRPNG                string              `json:"qr_png"`
	OutputJSON           string              `json:"output_json"`
	EvidenceItems        []evidenceItemInput `json:"items"`
	PageRoot             string              `json:"page_root"`
	PublicCode           string              `json:"public_code"`
}

func main() {
	_ = himindjsonrpc.Serve(os.Stdin, os.Stdout, handle)
}

func handle(request himindjsonrpc.Request) (any, *himindjsonrpc.Error) {
	switch request.Method {
	case "credential.import.verify":
		return verifyAccessImport(request)
	case "andx.page.access.directory", "andx.page.exhibit.create", "andx.page.workspace.bind", "andx.page.workspace.resolve":
		return accessOperation(request)
	case "andx.page.receipt.save":
		var in input
		if rpcError := himindjsonrpc.DecodeParams(request, &in); rpcError != nil {
			return nil, rpcError
		}
		result, err := saveWorkflowReceipt(in)
		if err != nil {
			return nil, operationError(err)
		}
		return result, nil
	case "andx.page.device.enrollment":
		var in input
		if rpcError := himindjsonrpc.DecodeParams(request, &in); rpcError != nil {
			return nil, rpcError
		}
		result, err := developerEnrollment(in)
		if err != nil {
			return nil, operationError(err)
		}
		return result, nil
	case "andx.page.project.inspect":
		return handleProjectInspect(request)
	case "andx.page.contract.validate":
		return handleContractValidate(request)
	case "andx.page.package":
		return handlePackage(request)
	case "andx.page.test.run":
		return handleBridgeTest(request)
	case "andx.page.deploy.preview":
		return handleDeployPreview(request)
	case "andx.page.deploy.activate":
		return handleDeployActivate(request)
	case "andx.page.qr.export":
		return handleQRExport(request)
	case "andx.page.evidence.pack":
		return handleEvidencePack(request)
	default:
		return nil, &himindjsonrpc.Error{Code: -32601, Message: "unknown method: " + request.Method}
	}
}

func handleProjectInspect(request himindjsonrpc.Request) (any, *himindjsonrpc.Error) {
	var in input
	if rpcError := himindjsonrpc.DecodeParams(request, &in); rpcError != nil {
		return nil, rpcError
	}
	result, err := inspectSource(in)
	if err != nil {
		return nil, operationError(err)
	}
	return result, nil
}

func handleContractValidate(request himindjsonrpc.Request) (any, *himindjsonrpc.Error) {
	var in input
	if rpcError := himindjsonrpc.DecodeParams(request, &in); rpcError != nil {
		return nil, rpcError
	}
	page, err := validatePageInput(in)
	if err != nil {
		var issues *validationIssues
		if errors.As(err, &issues) {
			return map[string]any{"ok": false, "errors": issues.Issues, "warnings": issues.Warnings}, nil
		}
		return nil, operationError(err)
	}
	return map[string]any{
		"ok": true, "source_root": page.Inventory.Root, "source_kind": page.Inventory.SourceKind,
		"entry_file": "index.html", "page_key": page.PageKey, "version": page.Version,
		"display_mode": page.DisplayMode, "bridge_version": bridgeVersion,
		"required_capabilities": page.RequiredCapabilities, "optional_capabilities": page.OptionalCapabilities,
		"file_count": len(page.Inventory.Files) + 1, "total_source_bytes": page.Inventory.TotalBytes,
		"skipped_runtime_files": page.Inventory.SkippedFiles, "warnings": page.Inventory.Warnings,
	}, nil
}

func handlePackage(request himindjsonrpc.Request) (any, *himindjsonrpc.Error) {
	var in input
	if rpcError := himindjsonrpc.DecodeParams(request, &in); rpcError != nil {
		return nil, rpcError
	}
	result, err := packagePage(in)
	if err == nil {
		result, err = attachWorkflowReceipt(result, in, "custom-page-candidate", result)
	}
	if err != nil {
		return nil, operationError(err)
	}
	return result, nil
}

func handleBridgeTest(request himindjsonrpc.Request) (any, *himindjsonrpc.Error) {
	var in input
	if rpcError := himindjsonrpc.DecodeParams(request, &in); rpcError != nil {
		return nil, rpcError
	}
	result, err := runBridgeFixture(in)
	if err == nil {
		result, err = attachWorkflowReceipt(result, in, "bridge-test-report", result)
	}
	if err != nil {
		return nil, operationError(err)
	}
	return result, nil
}

func handleDeployPreview(request himindjsonrpc.Request) (any, *himindjsonrpc.Error) {
	var in input
	if rpcError := himindjsonrpc.DecodeParams(request, &in); rpcError != nil {
		return nil, rpcError
	}
	result, err := deployPreview(in)
	if err == nil {
		result, err = attachWorkflowReceipt(result, in, "deployment-receipt", result)
	}
	if err != nil {
		return nil, operationError(err)
	}
	return result, nil
}

func handleDeployActivate(request himindjsonrpc.Request) (any, *himindjsonrpc.Error) {
	var in input
	if rpcError := himindjsonrpc.DecodeParams(request, &in); rpcError != nil {
		return nil, rpcError
	}
	result, err := activateDeployment(in)
	if err == nil {
		result, err = attachWorkflowReceipt(result, in, "deployment-receipt", result)
	}
	if err != nil {
		return nil, operationError(err)
	}
	return result, nil
}

func handleQRExport(request himindjsonrpc.Request) (any, *himindjsonrpc.Error) {
	var in input
	if rpcError := himindjsonrpc.DecodeParams(request, &in); rpcError != nil {
		return nil, rpcError
	}
	result, err := exportInteractionQR(in)
	if err != nil {
		return nil, operationError(err)
	}
	return result, nil
}

func handleEvidencePack(request himindjsonrpc.Request) (any, *himindjsonrpc.Error) {
	var in input
	if rpcError := himindjsonrpc.DecodeParams(request, &in); rpcError != nil {
		return nil, rpcError
	}
	result, err := packEvidence(in)
	if err == nil {
		result, err = attachWorkflowReceipt(result, in, "evidence-manifest", result["manifest"])
	}
	if err != nil {
		return nil, operationError(err)
	}
	return result, nil
}

func operationError(err error) *himindjsonrpc.Error {
	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = "AndX 定制页工具执行失败"
	}
	return &himindjsonrpc.Error{Code: -32000, Message: message}
}

func validationError(issues []string, warnings []string) error {
	return &validationIssues{Issues: issues, Warnings: warnings}
}

type validationIssues struct {
	Issues   []string
	Warnings []string
}

func (e *validationIssues) Error() string {
	if len(e.Issues) == 0 {
		return "定制页契约校验失败"
	}
	return fmt.Sprintf("定制页契约校验失败：%s", strings.Join(e.Issues, "; "))
}
