package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//go:embed harness/run.mjs
var bridgeHarnessSource []byte

func runBridgeFixture(in input) (map[string]any, error) {
	projectRoot, sdkRoot, outputDir, err := resolveBridgeTestPaths(in)
	if err != nil {
		return nil, err
	}
	nodePath := strings.TrimSpace(in.NodePath)
	if nodePath == "" {
		nodePath, err = exec.LookPath("node")
		if err != nil {
			return nil, errors.New("node 不可用；请传入 node_path")
		}
	} else if _, err := exec.LookPath(nodePath); err != nil {
		return nil, fmt.Errorf("node_path 不可用：%w", err)
	}
	timeoutSeconds := in.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 120
	}
	if timeoutSeconds > 600 {
		return nil, errors.New("timeout_seconds 不能超过 600")
	}

	harnessDir := filepath.Join(outputDir, ".harness")
	if err := os.MkdirAll(harnessDir, 0o755); err != nil {
		return nil, fmt.Errorf("创建测试目录失败：%w", err)
	}
	harnessPath := filepath.Join(harnessDir, "run.mjs")
	if err := os.WriteFile(harnessPath, bridgeHarnessSource, 0o644); err != nil {
		return nil, fmt.Errorf("写入 Bridge 测试脚本失败：%w", err)
	}

	args := []string{
		harnessPath,
		"--sdk-root", sdkRoot,
		"--output-dir", outputDir,
		"--timeout-ms", strconv.Itoa(timeoutSeconds * 1000),
	}
	if pageRoot := strings.TrimSpace(in.PageRoot); pageRoot != "" {
		if !filepath.IsAbs(pageRoot) {
			pageRoot = filepath.Join(projectRoot, pageRoot)
		}
		pageRoot, err = filepath.Abs(pageRoot)
		if err != nil {
			return nil, err
		}
		if !pathWithin(projectRoot, pageRoot) {
			return nil, errors.New("page_root 必须位于 project_root 内")
		}
		args = append(args, "--page-root", pageRoot)
	}
	if publicCode := strings.TrimSpace(in.PublicCode); publicCode != "" {
		args = append(args, "--public-code", publicCode)
	}
	if chromePath := strings.TrimSpace(in.ChromePath); chromePath != "" {
		if info, err := os.Stat(chromePath); err != nil || info.IsDir() {
			return nil, errors.New("chrome_path 必须指向可执行文件")
		}
		args = append(args, "--chrome", chromePath)
	}

	// Let the harness report its deadline and close Chrome before hard cancellation.
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds+10)*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, nodePath, args...)
	command.Cancel = func() error { return stopBridgeProcess(command) }
	command.WaitDelay = 3 * time.Second
	command.Dir = projectRoot
	var stdout strings.Builder
	var stderr strings.Builder
	command.Stdout = &stdout
	command.Stderr = &stderr
	runErr := command.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, errors.New("Bridge 测试超时")
	}
	output := strings.TrimSpace(stdout.String())
	var result map[string]any
	if output != "" {
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			return nil, fmt.Errorf("解析 Bridge 测试结果失败：%w; stderr=%s", err, compactTail(stderr.String(), 1000))
		}
	}
	if result == nil {
		return nil, fmt.Errorf("Bridge 测试未返回 JSON：%v; stderr=%s", runErr, compactTail(stderr.String(), 1000))
	}
	if runErr != nil {
		result["exit_error"] = runErr.Error()
	}
	result["node_path"] = nodePath
	result["output_dir"] = outputDir
	return result, nil
}

func resolveBridgeTestPaths(in input) (string, string, string, error) {
	projectInput := strings.TrimSpace(in.ProjectRoot)
	sdkInput := strings.TrimSpace(in.SdkRoot)
	outputInput := strings.TrimSpace(in.OutputDir)
	if projectInput == "" {
		return "", "", "", errors.New("project_root is required")
	}
	if outputInput == "" {
		return "", "", "", errors.New("output_dir is required")
	}
	projectRoot, err := filepath.Abs(projectInput)
	if err != nil {
		return "", "", "", err
	}
	projectRoot, err = filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return "", "", "", fmt.Errorf("project_root 不存在或无法解析：%w", err)
	}
	info, err := os.Stat(projectRoot)
	if err != nil || !info.IsDir() {
		return "", "", "", errors.New("project_root 必须是已存在目录")
	}
	if sdkInput == "" {
		for current := projectRoot; ; current = filepath.Dir(current) {
			candidate := filepath.Join(current, "node_modules", "@mrbaoquan", "andx")
			if _, err := os.Stat(filepath.Join(candidate, "dist", "index.js")); err == nil {
				sdkInput = candidate
				break
			}
			if filepath.Dir(current) == current {
				break
			}
		}
		if sdkInput == "" {
			return "", "", "", errors.New("未找到 @mrbaoquan/andx；请先在工程安装真实 SDK，或指定 sdk_root")
		}
	} else if !filepath.IsAbs(sdkInput) {
		sdkInput = filepath.Join(projectRoot, sdkInput)
	}
	sdkRoot, err := filepath.Abs(sdkInput)
	if err != nil {
		return "", "", "", err
	}
	sdkRoot, err = filepath.EvalSymlinks(sdkRoot)
	if err != nil {
		return "", "", "", fmt.Errorf("sdk_root 不存在或无法解析：%w", err)
	}
	if _, err := os.Stat(filepath.Join(sdkRoot, "dist", "index.js")); err != nil {
		if _, rootErr := os.Stat(filepath.Join(sdkRoot, "index.js")); rootErr != nil {
			return "", "", "", errors.New("sdk_root 必须指向 @mrbaoquan/andx 包目录或其 dist 目录")
		}
	}
	if !filepath.IsAbs(outputInput) {
		outputInput = filepath.Join(projectRoot, outputInput)
	}
	outputDir, err := filepath.Abs(outputInput)
	if err != nil {
		return "", "", "", err
	}
	if !pathWithin(projectRoot, outputDir) {
		return "", "", "", errors.New("output_dir 必须位于 project_root 内")
	}
	return projectRoot, sdkRoot, outputDir, nil
}

func compactTail(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[len(value)-limit:]
}
