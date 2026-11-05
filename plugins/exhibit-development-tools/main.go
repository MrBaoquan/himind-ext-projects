package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/MrBaoquan/himind-extensions/sdk/jsonrpc"
)

const maxScriptBytes = int64(64 << 20)

type input struct {
	ScriptPath          string         `json:"script_path"`
	ContentJSON         map[string]any `json:"content_json"`
	SpecPath            string         `json:"spec_path"`
	ProjectRoot         string         `json:"project_root"`
	WorkspaceRoot       string         `json:"workspace_root"`
	Items               []evidenceItem `json:"items"`
	OutputDir           string         `json:"output_dir"`
	CliPath             string         `json:"cli_path"`
	BridgePath          string         `json:"bridge_path"`
	SmokeScript         string         `json:"smoke_script_path"`
	Port                int            `json:"port"`
	Mock                bool           `json:"mock"`
	Provider            bool           `json:"provider"`
	Shell               bool           `json:"shell"`
	Force               bool           `json:"force"`
	ProjectName         string         `json:"project_name"`
	TemplateID          string         `json:"template_id"`
	CdpPort             int            `json:"cdp_port"`
	NoProvider          bool           `json:"no_provider"`
	TailLines           int            `json:"tail_lines"`
	Target              string         `json:"target"`
	CorePath            string         `json:"core_path"`
	LauncherPath        string         `json:"launcher_path"`
	IoDeviceRoot        string         `json:"iodevice_root"`
	BridgeTransport     string         `json:"bridge_transport"`
	BridgeEndpoint      string         `json:"bridge_endpoint"`
	AndroidWebViewDebug bool           `json:"android_webview_debug"`
	ApkPath             string         `json:"apk_path"`
	AaptPath            string         `json:"aapt_path"`
	ApkSignerPath       string         `json:"apk_signer_path"`
	AdbPath             string         `json:"adb_path"`
	DeviceSerial        string         `json:"device_serial"`
	PackageID           string         `json:"package_id"`
	Activity            string         `json:"activity"`
	ProcessID           int            `json:"process_id"`
}

type evidenceItem struct {
	Path           string         `json:"path"`
	ArtifactKind   string         `json:"artifact_kind"`
	RequirementIDs []string       `json:"requirement_ids,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type documentBlock struct {
	Type  string     `json:"type"`
	Text  string     `json:"text,omitempty"`
	Rows  [][]string `json:"rows,omitempty"`
	Index int        `json:"index"`
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

func main() {
	if err := jsonrpc.Serve(os.Stdin, os.Stdout, handle); err != nil {
		fmt.Fprintln(os.Stderr, "展项开发工具已停止:", err)
	}
}

func handle(request jsonrpc.Request) (any, *jsonrpc.Error) {
	var in input
	if rpcError := jsonrpc.DecodeParams(request, &in); rpcError != nil {
		return nil, rpcError
	}
	switch request.Method {
	case "exhibit.script.parse":
		result, err := parseScript(in.ScriptPath)
		return rpcResult(result, err)
	case "exhibit.script.spec.validate":
		result, err := validateSpec(in)
		return rpcResult(result, err)
	case "exhibit.project.inspect":
		result, err := inspectProject(in.ProjectRoot)
		return rpcResult(result, err)
	case "exhibit.evidence.pack":
		result, err := packEvidence(in.WorkspaceRoot, in.Items)
		return rpcResult(result, err)
	case "exhibit.web.project.inspect":
		result, err := inspectWebProject(in.ProjectRoot)
		return rpcResult(result, err)
	case "exhibit.web.experience.inspect":
		result, err := inspectWebExperience(in.ProjectRoot)
		return rpcResult(result, err)
	case "exhibit.web.build":
		result, err := runMyriadCLI(in, "build")
		return rpcResult(result, err)
	case "exhibit.web.package":
		result, err := runMyriadCLI(in, "package")
		return rpcResult(result, err)
	case "exhibit.runtime.smoke":
		result, err := smokeRuntime(in)
		return rpcResult(result, err)
	case "exhibit.web.template.list":
		result, err := runMyriadArgs(in, []string{"template", "list"}, projectWorkingDirectory(in), 2*time.Minute)
		return rpcResult(result, err)
	case "exhibit.web.template.inspect":
		result, err := runMyriadArgs(in, []string{"template", "inspect", strings.TrimSpace(in.TemplateID)}, projectWorkingDirectory(in), 2*time.Minute)
		return rpcResult(result, err)
	case "exhibit.web.template.create":
		result, err := createTemplateProject(in)
		return rpcResult(result, err)
	case "exhibit.web.dev.start":
		result, err := startDevelopmentSession(in)
		return rpcResult(result, err)
	case "exhibit.web.dev.status":
		result, err := runMyriadArgs(in, []string{"dev", "status", strings.TrimSpace(in.ProjectRoot)}, strings.TrimSpace(in.ProjectRoot), 2*time.Minute)
		return rpcResult(result, err)
	case "exhibit.web.dev.stop":
		result, err := runMyriadArgs(in, []string{"dev", "stop", strings.TrimSpace(in.ProjectRoot)}, strings.TrimSpace(in.ProjectRoot), 2*time.Minute)
		return rpcResult(result, err)
	case "exhibit.web.dist.verify":
		result, err := verifyDist(in)
		return rpcResult(result, err)
	case "exhibit.web.apk.verify":
		result, err := verifyAPK(in)
		return rpcResult(result, err)
	case "exhibit.web.android.install":
		result, err := androidInstall(in)
		return rpcResult(result, err)
	case "exhibit.web.android.launch":
		result, err := androidLaunch(in)
		return rpcResult(result, err)
	case "exhibit.web.android.logs":
		result, err := androidLogs(in)
		return rpcResult(result, err)
	case "exhibit.web.android.cdp":
		result, err := androidCDP(in)
		return rpcResult(result, err)
	case "exhibit.web.logs.read":
		result, err := readProjectLogs(in)
		return rpcResult(result, err)
	default:
		return nil, &jsonrpc.Error{Code: -32601, Message: "unknown method: " + request.Method}
	}
}

func inspectWebProject(root string) (map[string]any, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("project_root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	manifestPath := filepath.Join(abs, "exhibit.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("exhibit.json cannot be read: %w", err)
	}
	var manifest map[string]any
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("exhibit.json is invalid: %w", err)
	}
	entry, _ := manifest["entry"].(string)
	build, _ := manifest["build"].(map[string]any)
	config, _ := manifest["config"].(map[string]any)
	media, _ := manifest["media"].(map[string]any)
	entryPath := filepath.Join(abs, filepath.FromSlash(entry))
	configFile, _ := config["file"].(string)
	schemaFile, _ := config["schema"].(string)
	mediaRoot, _ := media["root"].(string)
	providers, _ := manifest["providers"].([]any)
	providerRows := make([]map[string]any, 0, len(providers))
	for _, raw := range providers {
		provider, _ := raw.(map[string]any)
		executable, _ := provider["executable"].(string)
		providerRows = append(providerRows, map[string]any{
			"id":         provider["id"],
			"type":       provider["type"],
			"executable": executable,
			"exists":     executable != "" && pathIsFile(filepath.Join(abs, filepath.FromSlash(executable))),
		})
	}
	framework := "static"
	if data, err := os.ReadFile(filepath.Join(abs, "package.json")); err == nil {
		var pkg map[string]any
		if json.Unmarshal(data, &pkg) == nil {
			if deps, ok := pkg["dependencies"].(map[string]any); ok {
				if _, ok := deps["vue"]; ok {
					framework = "vue"
				}
				if _, ok := deps["react"]; ok {
					framework = "react"
				}
				if _, ok := deps["svelte"]; ok {
					framework = "svelte"
				}
			}
		}
	}
	buildCommand, _ := build["command"].(string)
	blockers := make([]string, 0)
	if strings.TrimSpace(entry) == "" {
		blockers = append(blockers, "entry is required")
	}
	if !pathIsFile(entryPath) {
		blockers = append(blockers, "entry file is missing; run exhibit.web.build before package")
	}
	if configFile != "" && !pathIsFile(filepath.Join(abs, filepath.FromSlash(configFile))) {
		blockers = append(blockers, "runtime config is missing")
	}
	if schemaFile != "" && !pathIsFile(filepath.Join(abs, filepath.FromSlash(schemaFile))) {
		blockers = append(blockers, "config schema is missing")
	}
	return map[string]any{
		"schema_version": "exhibit-web-project-inspection.v1",
		"project_root":   abs,
		"manifest":       manifest,
		"framework":      framework,
		"build_command":  buildCommand,
		"entry_exists":   pathIsFile(entryPath),
		"config_exists":  configFile == "" || pathIsFile(filepath.Join(abs, filepath.FromSlash(configFile))),
		"schema_exists":  schemaFile == "" || pathIsFile(filepath.Join(abs, filepath.FromSlash(schemaFile))),
		"media_exists":   mediaRoot == "" || pathIsDir(filepath.Join(abs, filepath.FromSlash(mediaRoot))),
		"providers":      providerRows,
		"blockers":       blockers,
	}, nil
}

func projectWorkingDirectory(in input) string {
	if root := strings.TrimSpace(in.ProjectRoot); root != "" && pathIsDir(root) {
		return root
	}
	if root := strings.TrimSpace(in.WorkspaceRoot); root != "" && pathIsDir(root) {
		return root
	}
	return "."
}

func runMyriadArgs(in input, args []string, workingDirectory string, timeout time.Duration) (map[string]any, error) {
	cliPath := strings.TrimSpace(in.CliPath)
	if cliPath == "" || !pathIsFile(cliPath) {
		return nil, errors.New("cli_path is required and must exist")
	}
	executable := cliPath
	commandArgs := append([]string(nil), args...)
	if strings.EqualFold(filepath.Ext(cliPath), ".dll") {
		executable = "dotnet"
		commandArgs = append([]string{cliPath}, commandArgs...)
	}
	return runProcessJSON(executable, commandArgs, workingDirectory, timeout)
}

func createTemplateProject(in input) (map[string]any, error) {
	name := strings.TrimSpace(in.ProjectName)
	template := strings.TrimSpace(in.TemplateID)
	if name == "" || template == "" {
		return nil, errors.New("project_name and template_id are required")
	}
	output := strings.TrimSpace(in.OutputDir)
	if output == "" {
		output = strings.TrimSpace(in.WorkspaceRoot)
	}
	if output == "" {
		output = "."
	}
	args := []string{"new", name, "--template", template, "--output", output}
	return runMyriadArgs(in, args, output, 5*time.Minute)
}

func startDevelopmentSession(in input) (map[string]any, error) {
	projectRoot := strings.TrimSpace(in.ProjectRoot)
	if projectRoot == "" || !pathIsDir(projectRoot) {
		return nil, errors.New("project_root is required and must be a directory")
	}
	args := []string{"dev", "start", projectRoot, "--json"}
	if in.Shell {
		args = append(args, "--shell")
	}
	if in.Mock {
		args = append(args, "--mock")
	}
	if in.Provider {
		args = append(args, "--provider")
	}
	if in.NoProvider {
		args = append(args, "--no-provider")
	}
	if in.Port > 0 {
		args = append(args, "--port", fmt.Sprintf("%d", in.Port))
	}
	if in.CdpPort > 0 {
		args = append(args, "--cdp-port", fmt.Sprintf("%d", in.CdpPort))
	}
	return runMyriadArgs(in, args, projectRoot, 2*time.Minute)
}

func verifyDist(in input) (map[string]any, error) {
	projectRoot := strings.TrimSpace(in.ProjectRoot)
	if projectRoot == "" || !pathIsDir(projectRoot) {
		return nil, errors.New("project_root is required and must be a directory")
	}
	dist := strings.TrimSpace(in.OutputDir)
	if dist == "" {
		dist = filepath.Join(projectRoot, "dist")
	}
	return runMyriadArgs(in, []string{"dist", "verify", dist}, projectRoot, 2*time.Minute)
}

func readProjectLogs(in input) (map[string]any, error) {
	projectRoot := strings.TrimSpace(in.ProjectRoot)
	if projectRoot == "" || !pathIsDir(projectRoot) {
		return nil, errors.New("project_root is required and must be a directory")
	}
	logRoot := filepath.Join(projectRoot, "logs")
	files, err := filepath.Glob(filepath.Join(logRoot, "*.log"))
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		left, _ := os.Stat(files[i])
		right, _ := os.Stat(files[j])
		return left.ModTime().After(right.ModTime())
	})
	limit := in.TailLines
	if limit <= 0 || limit > 2000 {
		limit = 200
	}
	if len(files) == 0 {
		return map[string]any{"ok": true, "project_root": projectRoot, "files": []string{}, "lines": []string{}}, nil
	}
	data, err := os.ReadFile(files[0])
	if err != nil {
		return nil, err
	}
	text := strings.TrimRight(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	lines := strings.Split(text, "\n")
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	return map[string]any{
		"ok":           true,
		"project_root": projectRoot,
		"file":         files[0],
		"tail_lines":   limit,
		"lines":        lines,
	}, nil
}
func runMyriadCLI(in input, command string) (map[string]any, error) {
	cliPath := strings.TrimSpace(in.CliPath)
	projectRoot := strings.TrimSpace(in.ProjectRoot)
	if cliPath == "" || !pathIsFile(cliPath) {
		return nil, errors.New("cli_path is required and must exist")
	}
	if projectRoot == "" {
		return nil, errors.New("project_root is required")
	}
	executable := cliPath
	args := []string{command, projectRoot}
	if strings.EqualFold(filepath.Ext(cliPath), ".dll") {
		executable = "dotnet"
		args = append([]string{cliPath}, args...)
	}
	if command == "package" && strings.TrimSpace(in.OutputDir) != "" {
		args = append(args, "--output", strings.TrimSpace(in.OutputDir))
		if in.Force {
			args = append(args, "--force")
		}
	}
	if command == "build" {
		args = appendBuildArgs(args, in)
	}
	return runProcessJSON(executable, args, projectRoot, 30*time.Minute)
}

func appendBuildArgs(args []string, in input) []string {
	add := func(flag, value string) {
		if strings.TrimSpace(value) != "" {
			args = append(args, flag, strings.TrimSpace(value))
		}
	}
	add("--target", in.Target)
	add("--output", in.OutputDir)
	add("--core", in.CorePath)
	add("--launcher", in.LauncherPath)
	add("--iodevice-root", in.IoDeviceRoot)
	add("--bridge-transport", in.BridgeTransport)
	add("--bridge-endpoint", in.BridgeEndpoint)
	if in.AndroidWebViewDebug {
		args = append(args, "--android-webview-debug")
	}
	return args
}

func resolveAPKPath(in input) (string, error) {
	path := strings.TrimSpace(in.ApkPath)
	if path == "" && strings.TrimSpace(in.OutputDir) != "" {
		matches, _ := filepath.Glob(filepath.Join(strings.TrimSpace(in.OutputDir), "android-arm64", "*.apk"))
		if len(matches) > 0 {
			path = matches[0]
		}
	}
	if path == "" {
		return "", errors.New("apk_path is required or output_dir must contain android-arm64/*.apk")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if !pathIsFile(abs) {
		return "", fmt.Errorf("APK was not found: %s", abs)
	}
	return abs, nil
}

func runTool(executable string, args []string, workingDirectory string, timeout time.Duration) map[string]any {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = workingDirectory
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	result := map[string]any{
		"executable": executable,
		"args":       args,
		"exit_code":  exitCode,
		"stdout":     strings.TrimSpace(stdout.String()),
		"stderr":     strings.TrimSpace(stderr.String()),
		"ok":         err == nil && exitCode == 0,
	}
	if ctx.Err() == context.DeadlineExceeded {
		result["timed_out"] = true
	}
	if err != nil {
		result["error"] = err.Error()
	}
	return result
}

func verifyAPK(in input) (map[string]any, error) {
	apk, err := resolveAPKPath(in)
	if err != nil {
		return nil, err
	}
	reader, err := zip.OpenReader(apk)
	if err != nil {
		return nil, fmt.Errorf("APK archive is invalid: %w", err)
	}
	defer reader.Close()
	entries := map[string]bool{}
	for _, file := range reader.File {
		entries[file.Name] = true
	}
	digest, err := fileSHA256(apk)
	if err != nil {
		return nil, err
	}
	info, _ := os.Stat(apk)
	hasIODevicePayload := entries["lib/arm64-v8a/iodevice.so"] || entries["assets/iodevice/iodevice.so"]
	hasSimulatorPlugin := entries["assets/iodevice/ExternalLibraries/IOUI-ANDROID-SIMULATOR.so"]
	result := map[string]any{
		"schema_version":       "exhibit-apk-verification.v1",
		"apk_path":             apk,
		"size_bytes":           info.Size(),
		"sha256":               digest,
		"has_manifest":         entries["AndroidManifest.xml"],
		"has_classes_dex":      entries["classes.dex"],
		"has_arm64_iodevice":   hasIODevicePayload,
		"has_simulator_plugin": hasSimulatorPlugin,
		"has_exhibit_runtime":  entries["assets/exhibit/myriadbox.runtime.json"],
		"has_target_manifest":  entries["assets/exhibit/target.manifest.json"],
		"package_id":           strings.TrimSpace(in.PackageID),
	}
	blockers := make([]string, 0)
	if !entries["AndroidManifest.xml"] || !entries["classes.dex"] {
		blockers = append(blockers, "APK 缺少 AndroidManifest.xml 或 classes.dex")
	}
	if !hasIODevicePayload {
		blockers = append(blockers, "APK 缺少 assets/iodevice/iodevice.so")
	}
	if !entries["assets/exhibit/myriadbox.runtime.json"] || !entries["assets/exhibit/target.manifest.json"] {
		blockers = append(blockers, "APK 缺少展项 runtime 或 target manifest")
	}
	if pathIsFile(strings.TrimSpace(in.AaptPath)) {
		aapt := runTool(strings.TrimSpace(in.AaptPath), []string{"dump", "badging", apk}, filepath.Dir(apk), 30*time.Second)
		result["aapt"] = aapt
		if text, ok := aapt["stdout"].(string); ok {
			for _, line := range strings.Split(text, "\n") {
				if strings.HasPrefix(line, "package:") || strings.HasPrefix(line, "launchable-activity:") || strings.HasPrefix(line, "sdkVersion:") || strings.HasPrefix(line, "targetSdkVersion:") {
					existing, _ := result["badging"].([]string)
					result["badging"] = append(existing, line)
				}
			}
		}
	}
	if pathIsFile(strings.TrimSpace(in.ApkSignerPath)) {
		signer := runTool(strings.TrimSpace(in.ApkSignerPath), []string{"verify", "--verbose", "--print-certs", apk}, filepath.Dir(apk), 30*time.Second)
		result["apksigner"] = signer
		output, _ := signer["stdout"].(string)
		result["signature_verified"] = signer["ok"] == true && strings.Contains(output, "Verifies")
	}
	result["blockers"] = blockers
	result["ok"] = len(blockers) == 0
	return result, nil
}

func adbExecutable(in input) string {
	if value := strings.TrimSpace(in.AdbPath); value != "" {
		return value
	}
	return "adb"
}

func adbDeviceArgs(in input, args []string) []string {
	if serial := strings.TrimSpace(in.DeviceSerial); serial != "" {
		return append([]string{"-s", serial}, args...)
	}
	return args
}

func androidInstall(in input) (map[string]any, error) {
	apk, err := resolveAPKPath(in)
	if err != nil {
		return nil, err
	}
	result := runTool(adbExecutable(in), adbDeviceArgs(in, []string{"install", "-r", apk}), projectWorkingDirectory(in), 5*time.Minute)
	result["schema_version"] = "exhibit-android-install.v1"
	result["apk_path"] = apk
	return result, nil
}

func androidLaunch(in input) (map[string]any, error) {
	pkg := strings.TrimSpace(in.PackageID)
	if pkg == "" {
		return nil, errors.New("package_id is required")
	}
	activity := strings.TrimSpace(in.Activity)
	var args []string
	if activity != "" {
		args = []string{"shell", "am", "start", "-W", "-n", pkg + "/" + activity}
	} else {
		args = []string{"shell", "monkey", "-p", pkg, "-c", "android.intent.category.LAUNCHER", "1"}
	}
	result := runTool(adbExecutable(in), adbDeviceArgs(in, args), projectWorkingDirectory(in), time.Minute)
	result["schema_version"] = "exhibit-android-launch.v1"
	result["package_id"] = pkg
	result["activity"] = activity
	return result, nil
}

func androidLogs(in input) (map[string]any, error) {
	limit := in.TailLines
	if limit <= 0 || limit > 5000 {
		limit = 300
	}
	args := []string{"logcat", "-d", "-v", "time", "-t", fmt.Sprintf("%d", limit)}
	result := runTool(adbExecutable(in), adbDeviceArgs(in, args), projectWorkingDirectory(in), time.Minute)
	result["schema_version"] = "exhibit-android-logs.v1"
	result["tail_lines"] = limit
	return result, nil
}

func androidCDP(in input) (map[string]any, error) {
	pkg := strings.TrimSpace(in.PackageID)
	processID := in.ProcessID
	if processID <= 0 && pkg != "" {
		pidResult := runTool(adbExecutable(in), adbDeviceArgs(in, []string{"shell", "pidof", pkg}), projectWorkingDirectory(in), 30*time.Second)
		if text, ok := pidResult["stdout"].(string); ok {
			fields := strings.Fields(text)
			if len(fields) > 0 {
				_, _ = fmt.Sscanf(fields[0], "%d", &processID)
			}
		}
	}
	if processID <= 0 {
		return nil, errors.New("process_id or package_id with a running process is required")
	}
	port := in.Port
	if port <= 0 {
		port = 9222
	}
	socketName := fmt.Sprintf("localabstract:webview_devtools_remote_%d", processID)
	forward := runTool(adbExecutable(in), adbDeviceArgs(in, []string{"forward", fmt.Sprintf("tcp:%d", port), socketName}), projectWorkingDirectory(in), 30*time.Second)
	if forward["ok"] != true {
		return map[string]any{"schema_version": "exhibit-android-cdp.v1", "ok": false, "process_id": processID, "forward": forward}, nil
	}
	client := http.Client{Timeout: 10 * time.Second}
	response, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
	if err != nil {
		return map[string]any{"schema_version": "exhibit-android-cdp.v1", "ok": false, "process_id": processID, "forward": forward, "error": err.Error()}, nil
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	var targets any
	if err := json.Unmarshal(body, &targets); err != nil {
		return nil, fmt.Errorf("CDP target list is invalid: %w", err)
	}
	return map[string]any{"schema_version": "exhibit-android-cdp.v1", "ok": response.StatusCode == http.StatusOK, "process_id": processID, "port": port, "socket": socketName, "forward": forward, "targets": targets}, nil
}
func smokeRuntime(in input) (map[string]any, error) {
	script := strings.TrimSpace(in.SmokeScript)
	projectRoot := strings.TrimSpace(in.ProjectRoot)
	bridgePath := strings.TrimSpace(in.BridgePath)
	if script == "" || !pathIsFile(script) {
		return nil, errors.New("smoke_script_path is required and must exist")
	}
	if projectRoot == "" || bridgePath == "" || !pathIsFile(bridgePath) {
		return nil, errors.New("project_root and bridge_path are required")
	}
	port := in.Port
	if port <= 0 {
		port = 19198
	}
	args := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", script, "-ProjectRoot", projectRoot, "-BridgePath", bridgePath, "-Port", fmt.Sprintf("%d", port)}
	if in.Mock {
		args = append(args, "-Mock")
	}
	return runProcessJSON("powershell", args, projectRoot, 2*time.Minute)
}

func runProcessJSON(executable string, args []string, workingDirectory string, timeout time.Duration) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = workingDirectory
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, errors.New("command timed out")
	}
	text := strings.TrimSpace(stdout.String())
	if text == "" {
		return nil, fmt.Errorf("command produced no JSON output: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		lines := strings.Split(text, "\n")
		if lastErr := json.Unmarshal([]byte(lines[len(lines)-1]), &result); lastErr != nil {
			return nil, fmt.Errorf("command output is not JSON: %w: %s", err, text)
		}
	}
	if err != nil {
		result["exit_code"] = cmd.ProcessState.ExitCode()
		result["stderr"] = strings.TrimSpace(stderr.String())
	}
	return result, nil
}
func rpcResult(result any, err error) (any, *jsonrpc.Error) {
	if err != nil {
		return nil, jsonrpc.InternalError(err.Error())
	}
	return result, nil
}

func parseScript(path string) (map[string]any, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("script_path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("script cannot be read: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxScriptBytes {
		return nil, errors.New("script file is empty, not regular, or too large")
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".doc" {
		return nil, errors.New("legacy .doc requires conversion to .docx before deterministic parsing")
	}
	var blocks []documentBlock
	switch ext {
	case ".docx":
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("docx cannot be read: %w", err)
		}
		blocks, err = parseDocx(data)
		if err != nil {
			return nil, err
		}
	case ".txt", ".md", ".json":
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("script cannot be read: %w", err)
		}
		blocks = []documentBlock{{Type: "paragraph", Text: string(data), Index: 0}}
	default:
		return nil, fmt.Errorf("unsupported script extension: %s", ext)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("script cannot be read: %w", err)
	}
	digest := sha256.Sum256(data)
	return map[string]any{
		"schema_version": "exhibit-script-document.v1",
		"source_path":    path,
		"file_name":      filepath.Base(path),
		"content_type":   scriptContentType(ext),
		"size_bytes":     info.Size(),
		"sha256":         hex.EncodeToString(digest[:]),
		"blocks":         blocks,
		"warnings":       scriptWarnings(ext),
	}, nil
}

func parseDocx(data []byte) ([]documentBlock, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("docx archive is invalid: %w", err)
	}
	var documentXML []byte
	for _, file := range reader.File {
		if file.Name != "word/document.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		documentXML, err = io.ReadAll(io.LimitReader(rc, maxScriptBytes+1))
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		break
	}
	if len(documentXML) == 0 {
		return nil, errors.New("docx does not contain word/document.xml")
	}
	return parseWordDocumentXML(documentXML)
}

func parseWordDocumentXML(data []byte) ([]documentBlock, error) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	stack := make([]string, 0, 32)
	blocks := make([]documentBlock, 0)
	var paragraph strings.Builder
	paragraphOpen := false
	var table [][]string
	var row []string
	var cell strings.Builder
	cellOpen := false
	tableOpen := false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			name := value.Name.Local
			stack = append(stack, name)
			switch name {
			case "p":
				paragraph.Reset()
				paragraphOpen = true
			case "tbl":
				table = nil
				tableOpen = true
			case "tr":
				row = nil
			case "tc":
				cell.Reset()
				cellOpen = true
			case "drawing", "pict":
				paragraph.WriteString("[图片]")
			}
		case xml.CharData:
			if paragraphOpen {
				paragraph.Write(value)
			}
			if cellOpen {
				cell.Write(value)
			}
		case xml.EndElement:
			name := value.Name.Local
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			switch name {
			case "p":
				text := strings.TrimSpace(paragraph.String())
				if cellOpen {
					if text != "" {
						if cell.Len() > 0 {
							cell.WriteString(" ")
						}
						cell.WriteString(text)
					}
				} else if text != "" {
					blocks = append(blocks, documentBlock{Type: "paragraph", Text: text, Index: len(blocks)})
				}
				paragraphOpen = false
			case "tc":
				row = append(row, strings.TrimSpace(cell.String()))
				cellOpen = false
			case "tr":
				if len(row) > 0 {
					table = append(table, append([]string(nil), row...))
				}
			case "tbl":
				if len(table) > 0 {
					blocks = append(blocks, documentBlock{Type: "table", Rows: table, Index: len(blocks)})
				}
				tableOpen = false
			}
		}
	}
	_ = tableOpen
	return blocks, nil
}

func scriptContentType(ext string) string {
	switch ext {
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".json":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

func scriptWarnings(ext string) []string {
	if ext == ".docx" {
		return []string{"段落和表格已提取；图片仅保留占位标记，需要从 Word 原文件或附件资源中单独导出。"}
	}
	return nil
}

func validateSpec(in input) (map[string]any, error) {
	content := in.ContentJSON
	if content == nil {
		path := strings.TrimSpace(in.SpecPath)
		if path == "" {
			return nil, errors.New("content_json or spec_path is required")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("spec cannot be read: %w", err)
		}
		if err := json.Unmarshal(data, &content); err != nil {
			return nil, fmt.Errorf("spec JSON is invalid: %w", err)
		}
	}
	blockers := make([]string, 0)
	if value, ok := content["schema_version"].(string); !ok || strings.TrimSpace(value) == "" {
		blockers = append(blockers, "schema_version is required")
	}
	if requirements, ok := content["requirements"].([]any); !ok || len(requirements) == 0 {
		blockers = append(blockers, "requirements must be a non-empty array")
	}
	return map[string]any{
		"schema_version": "exhibit-development-spec-validation.v1",
		"valid":          len(blockers) == 0,
		"blockers":       blockers,
	}, nil
}

func inspectProject(root string) (map[string]any, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("project_root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return nil, errors.New("project_root is not a readable directory")
	}
	manifestPath := filepath.Join(abs, "Packages", "manifest.json")
	versionPath := filepath.Join(abs, "ProjectSettings", "ProjectVersion.txt")
	unihperAssemblies := filepath.Join(abs, "Assets", "Resources", "UNIHper", "assemblies.json")
	unihperResources := filepath.Join(abs, "Assets", "Resources", "UNIHper", "resources.json")
	assetsDir := filepath.Join(abs, "Assets")
	projectSettingsDir := filepath.Join(abs, "ProjectSettings")
	packagesDir := filepath.Join(abs, "Packages")
	checks := map[string]bool{
		"assets_dir":           pathIsDir(assetsDir),
		"project_settings_dir": pathIsDir(projectSettingsDir),
		"packages_dir":         pathIsDir(packagesDir),
		"package_manifest":     pathIsFile(manifestPath),
		"project_version":      pathIsFile(versionPath),
		"unihper_assemblies":   pathIsFile(unihperAssemblies),
		"unihper_resources":    pathIsFile(unihperResources),
	}
	version := ""
	if data, err := os.ReadFile(versionPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "m_EditorVersion:") {
				version = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "m_EditorVersion:"))
			}
		}
	}
	manifest := map[string]any{}
	if data, err := os.ReadFile(manifestPath); err == nil {
		_ = json.Unmarshal(data, &manifest)
	}
	unihper := checks["unihper_assemblies"] || checks["unihper_resources"]
	blockers := make([]string, 0)
	if !checks["assets_dir"] || !checks["project_settings_dir"] {
		blockers = append(blockers, "project does not look like a Unity project")
	}
	if !checks["package_manifest"] {
		blockers = append(blockers, "Packages/manifest.json is missing")
	}
	if !unihper {
		blockers = append(blockers, "UNIHper configuration was not detected")
	}
	return map[string]any{
		"schema_version":   "exhibit-unity-project-inspection.v1",
		"project_root":     abs,
		"is_unity":         checks["assets_dir"] && checks["project_settings_dir"],
		"unity_version":    version,
		"unihper":          unihper,
		"mcp":              inspectUnityMCP(abs, version),
		"checks":           checks,
		"package_manifest": manifest,
		"blockers":         blockers,
	}, nil
}

func packEvidence(root string, items []evidenceItem) (map[string]any, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("workspace_root is required")
	}
	if len(items) == 0 {
		return nil, errors.New("items must not be empty")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(rootAbs); err != nil || !info.IsDir() {
		return nil, errors.New("workspace_root is not a readable directory")
	}
	artifacts := make([]evidenceArtifact, 0, len(items))
	var total int64
	for _, item := range items {
		path := strings.TrimSpace(item.Path)
		if path == "" {
			return nil, errors.New("evidence path is required")
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(rootAbs, path)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(rootAbs, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("evidence path escapes workspace root: %s", item.Path)
		}
		info, err := os.Stat(abs)
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("evidence file is not readable: %s", item.Path)
		}
		ext := strings.ToLower(filepath.Ext(abs))
		if !evidenceExtensionAllowed(ext) {
			return nil, fmt.Errorf("unsupported evidence extension: %s", ext)
		}
		digest, err := fileSHA256(abs)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, evidenceArtifact{
			Path:           abs,
			RelativePath:   filepath.ToSlash(rel),
			ArtifactKind:   item.ArtifactKind,
			FileName:       filepath.Base(abs),
			SizeBytes:      info.Size(),
			SHA256:         digest,
			RequirementIDs: unique(item.RequirementIDs),
			Metadata:       item.Metadata,
		})
		total += info.Size()
	}
	sort.SliceStable(artifacts, func(i, j int) bool { return artifacts[i].RelativePath < artifacts[j].RelativePath })
	return map[string]any{
		"schema_version": "exhibit-evidence-manifest.v1",
		"generated_at":   time.Now().UTC().Format(time.RFC3339Nano),
		"workspace_root": rootAbs,
		"artifacts":      artifacts,
		"artifact_count": len(artifacts),
		"total_bytes":    total,
	}, nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func evidenceExtensionAllowed(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".mp4", ".webm", ".mov", ".mkv", ".pdf", ".txt", ".md", ".json", ".log", ".zip":
		return true
	default:
		return false
	}
}

func unique(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func pathIsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func pathIsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
