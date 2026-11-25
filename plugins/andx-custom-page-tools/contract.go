package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	bridgeVersion        = "1.0"
	maxSourceFiles       = 499
	maxSingleFileBytes   = 50 * 1024 * 1024
	maxUncompressedBytes = 100 * 1024 * 1024
	maxPathLength        = 200
)

var (
	pageKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,63}$`)
	versionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

	allowedExtensions = map[string]struct{}{
		".html": {}, ".htm": {}, ".css": {}, ".js": {}, ".mjs": {}, ".json": {},
		".png": {}, ".jpg": {}, ".jpeg": {}, ".webp": {}, ".gif": {}, ".svg": {}, ".ico": {},
		".woff": {}, ".woff2": {}, ".ttf": {}, ".otf": {},
		".mp3": {}, ".wav": {}, ".ogg": {}, ".mp4": {}, ".webm": {},
		".glb": {}, ".gltf": {}, ".bin": {}, ".txt": {},
	}

	forbiddenDirectories = map[string]struct{}{
		".git": {}, "node_modules": {}, "__MACOSX": {}, ".myriadbox": {}, "bin": {}, "obj": {},
		"providers": {}, "hardware": {},
	}

	myriadBoxRuntimeFiles = map[string]struct{}{
		"myriadbox.runtime.json": {},
		"sdk/myriadbox.js":       {},
		"sdk/myriadbox.d.ts":     {},
	}

	supportedCapabilities = map[string]struct{}{
		"andx.action.request":    {},
		"andx.session.read":      {},
		"andx.status.read":       {},
		"andx.payment.request":   {},
		"andx.payment.events":    {},
		"andx.lifecycle.events":  {},
		"andx.visibility.events": {},
	}

	inlineEventHandler = regexp.MustCompile(`(?i)\son[a-z]+\s*=`)
	scriptBlock        = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)
	scriptSource       = regexp.MustCompile(`(?i)\bsrc\s*=`)
	metaRefresh        = regexp.MustCompile(`(?i)http-equiv\s*=\s*["']?refresh`)
	baseTag            = regexp.MustCompile(`(?i)<base\s`)
	rootAssetReference = regexp.MustCompile(`(?i)(?:src|href)\s*=\s*["']/`)
	cssRootAsset       = regexp.MustCompile(`(?i)url\(\s*["']?/`)
	myriadBoxReference = []*regexp.Regexp{
		regexp.MustCompile(`(?i)window\s*\.\s*MyriadBox`),
		regexp.MustCompile(`(?i)(?:^|[/"'])sdk/myriadbox\.js`),
		regexp.MustCompile(`(?i)myriadbox\.runtime\.json`),
		regexp.MustCompile(`(?i)/providers/`),
	}
)

type sourceFile struct {
	Path   string `json:"path"`
	Body   []byte `json:"-"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type sourceInventory struct {
	Root         string       `json:"root"`
	SourceKind   string       `json:"source_kind"`
	Files        []sourceFile `json:"files"`
	SkippedFiles []string     `json:"skipped_runtime_files"`
	Warnings     []string     `json:"warnings"`
	TotalBytes   int64        `json:"total_source_bytes"`
}

type validatedPage struct {
	PageActions          []string
	Inventory            sourceInventory
	PageKey              string
	Version              string
	DisplayMode          string
	RequiredCapabilities []string
	OptionalCapabilities []string
	BuiltAt              string
}

type manifestFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type bridgeCapabilitiesManifest struct {
	Required []string `json:"required"`
	Optional []string `json:"optional,omitempty"`
}

type bridgeManifest struct {
	Version      string                     `json:"version"`
	Capabilities bridgeCapabilitiesManifest `json:"capabilities"`
}

type pageManifest struct {
	PageActions []string       `json:"pageActions"`
	PageKey     string         `json:"pageKey"`
	Version     string         `json:"version"`
	DisplayMode string         `json:"displayMode"`
	EntryFile   string         `json:"entryFile"`
	BuiltAt     string         `json:"builtAt,omitempty"`
	Bridge      bridgeManifest `json:"bridge"`
	Files       []manifestFile `json:"files"`
}

func inspectSource(in input) (map[string]any, error) {
	fromMyriadBox := in.BuildReceipt != nil
	if in.BuildReceipt != nil {
		if in.BuildReceipt["ok"] != true || in.BuildReceipt["target"] != "web" {
			return nil, errors.New("MyriadBox 构建回执未成功或不是 Web 目标")
		}
		output, _ := in.BuildReceipt["output"].(string)
		artifact, _ := in.BuildReceipt["artifact"].(map[string]any)
		entry, _ := artifact["entrypoint"].(string)
		// CLI versions with PascalCase artifact members are also supported.
		if entry == "" {
			entry, _ = artifact["Entrypoint"].(string)
		}
		if output == "" || entry == "" || filepath.IsAbs(entry) || filepath.Base(entry) != "index.html" || strings.Contains(filepath.ToSlash(entry), "..") {
			return nil, errors.New("MyriadBox 构建回执缺少受限的静态入口")
		}
		in.SourceRoot = filepath.Join(output, filepath.Dir(entry))
	}
	_, sourceRoot, _, err := resolveProjectPaths(in, false)
	if err != nil {
		return nil, err
	}
	if fromMyriadBox {
		project, _, _, err := resolveProjectPaths(in, false)
		if err != nil {
			return nil, err
		}
		sourceRoot, err = prepareMyriadBoxStatic(project, sourceRoot)
		if err != nil {
			return nil, err
		}
	}
	inventory, err := collectSourceFiles(sourceRoot)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"ok":                    true,
		"source_root":           inventory.Root,
		"source_kind":           inventory.SourceKind,
		"has_index":             hasEntryFile(inventory.Files),
		"file_count":            len(inventory.Files),
		"total_source_bytes":    inventory.TotalBytes,
		"skipped_runtime_files": inventory.SkippedFiles,
		"warnings":              inventory.Warnings,
		"files":                 inventory.Files,
	}, nil
}

func validatePageInput(in input) (*validatedPage, error) {
	_, sourceRoot, _, err := resolveProjectPaths(in, false)
	if err != nil {
		return nil, err
	}

	var issues []string
	if !pageKeyPattern.MatchString(strings.TrimSpace(in.PageKey)) {
		issues = append(issues, "page_key 必须为 2-64 位小写字母、数字或连字符")
	}
	if !versionPattern.MatchString(strings.TrimSpace(in.Version)) {
		issues = append(issues, "version 只能包含字母、数字、点、下划线和连字符")
	}
	displayMode := strings.TrimSpace(in.DisplayMode)
	if displayMode == "" {
		displayMode = "embedded"
	}
	if displayMode != "embedded" && displayMode != "immersive" {
		issues = append(issues, "display_mode 只能是 embedded 或 immersive")
	}
	required, optional, capabilityIssues := validateCapabilities(in.RequiredCapabilities, in.OptionalCapabilities)
	issues = append(issues, capabilityIssues...)
	seenActions := map[string]bool{}
	actionPattern := regexp.MustCompile(`^[a-z][a-z0-9]*(?:\.[a-z][a-z0-9-]*)+$`)
	for _, action := range in.PageActions {
		if len(action) > 128 || !actionPattern.MatchString(action) || seenActions[action] {
			issues = append(issues, "page_actions 动作名无效或重复")
		}
		seenActions[action] = true
	}
	if len(in.PageActions) > 0 {
		declared := false
		for _, capability := range append(append([]string{}, required...), optional...) {
			if capability == "andx.action.request" {
				declared = true
			}
		}
		if !declared {
			issues = append(issues, "page_actions 需要声明 andx.action.request")
		}
	}

	inventory, inventoryErr := collectSourceFiles(sourceRoot)
	if inventoryErr != nil {
		issues = append(issues, inventoryErr.Error())
	}
	if len(issues) > 0 {
		warnings := []string{}
		if inventoryErr == nil {
			warnings = inventory.Warnings
		}
		return nil, validationError(issues, warnings)
	}
	if !hasEntryFile(inventory.Files) {
		return nil, validationError([]string{"静态产物根目录必须包含 index.html"}, inventory.Warnings)
	}
	if err := validateEntryAndContent(inventory.Files); err != nil {
		var issues *validationIssues
		if errors.As(err, &issues) {
			issues.Warnings = append(issues.Warnings, inventory.Warnings...)
			return nil, issues
		}
		return nil, err
	}
	if len(inventory.Files)+1 > 500 {
		return nil, validationError([]string{"静态文件数量超过 500 个（含 manifest.json）"}, inventory.Warnings)
	}
	return &validatedPage{
		PageActions:          append([]string{}, in.PageActions...),
		Inventory:            inventory,
		PageKey:              strings.TrimSpace(in.PageKey),
		Version:              strings.TrimSpace(in.Version),
		DisplayMode:          displayMode,
		RequiredCapabilities: required,
		OptionalCapabilities: optional,
		BuiltAt:              strings.TrimSpace(in.BuiltAt),
	}, nil
}

func resolveProjectPaths(in input, requireOutput bool) (string, string, string, error) {
	projectRoot := strings.TrimSpace(in.ProjectRoot)
	sourceRoot := strings.TrimSpace(in.SourceRoot)
	if projectRoot == "" {
		return "", "", "", errors.New("project_root is required")
	}
	if sourceRoot == "" {
		return "", "", "", errors.New("source_root is required")
	}
	projectAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", "", "", fmt.Errorf("无法解析 project_root: %w", err)
	}
	projectAbs, err = filepath.EvalSymlinks(projectAbs)
	if err != nil {
		return "", "", "", fmt.Errorf("project_root 不存在或无法解析: %w", err)
	}
	projectInfo, err := os.Stat(projectAbs)
	if err != nil || !projectInfo.IsDir() {
		return "", "", "", errors.New("project_root 必须是已存在目录")
	}
	if !filepath.IsAbs(sourceRoot) {
		sourceRoot = filepath.Join(projectAbs, sourceRoot)
	}
	sourceAbs, err := filepath.Abs(sourceRoot)
	if err != nil {
		return "", "", "", fmt.Errorf("无法解析 source_root: %w", err)
	}
	sourceAbs, err = filepath.EvalSymlinks(sourceAbs)
	if err != nil {
		return "", "", "", fmt.Errorf("source_root 不存在或无法解析: %w", err)
	}
	if !pathWithin(projectAbs, sourceAbs) {
		return "", "", "", errors.New("source_root 必须位于 project_root 内")
	}
	info, err := os.Stat(sourceAbs)
	if err != nil || !info.IsDir() {
		return "", "", "", errors.New("source_root 必须是已存在目录")
	}

	outputZip := ""
	if requireOutput {
		rawOutput := strings.TrimSpace(in.OutputZip)
		if rawOutput == "" {
			return "", "", "", errors.New("output_zip is required")
		}
		if !filepath.IsAbs(rawOutput) {
			rawOutput = filepath.Join(projectAbs, rawOutput)
		}
		outputZip, err = filepath.Abs(rawOutput)
		if err != nil {
			return "", "", "", fmt.Errorf("无法解析 output_zip: %w", err)
		}
		if !pathWithin(projectAbs, outputZip) {
			return "", "", "", errors.New("output_zip 必须位于 project_root 内")
		}
		if pathWithin(sourceAbs, outputZip) {
			return "", "", "", errors.New("output_zip 不能位于 source_root 内")
		}
		if strings.ToLower(filepath.Ext(outputZip)) != ".zip" {
			return "", "", "", errors.New("output_zip 必须使用 .zip 扩展名")
		}
	}
	return projectAbs, sourceAbs, outputZip, nil
}

func pathWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

func collectSourceFiles(root string) (sourceInventory, error) {
	inventory := sourceInventory{
		Root:         root,
		SourceKind:   "static",
		Files:        []sourceFile{},
		SkippedFiles: []string{},
		Warnings:     []string{},
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		lowerBase := strings.ToLower(entry.Name())
		if entry.IsDir() {
			if _, forbidden := forbiddenDirectories[lowerBase]; forbidden {
				return fmt.Errorf("静态产物包含禁止目录：%s", rel)
			}
			if lowerBase == "__macosx" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("静态产物不允许符号链接：%s", rel)
		}
		if lowerBase == ".ds_store" {
			inventory.SkippedFiles = append(inventory.SkippedFiles, rel)
			return nil
		}
		lowerRel := strings.ToLower(rel)
		if _, runtimeFile := myriadBoxRuntimeFiles[lowerRel]; runtimeFile {
			inventory.SourceKind = "myriadbox-web"
			inventory.SkippedFiles = append(inventory.SkippedFiles, rel)
			inventory.Warnings = append(inventory.Warnings, "已排除 MyriadBox 运行时文件："+rel)
			return nil
		}
		if err := validateRelativePath(rel); err != nil {
			return err
		}
		if lowerRel == "manifest.json" {
			return errors.New("source_root 不能预置 manifest.json，由打包器生成")
		}
		if err := validateAllowedFile(rel); err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxSingleFileBytes {
			return fmt.Errorf("单个文件不能超过 50MB：%s", rel)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		inventory.TotalBytes += int64(len(body))
		if inventory.TotalBytes > maxUncompressedBytes {
			return errors.New("静态产物总大小不能超过 100MB")
		}
		inventory.Files = append(inventory.Files, sourceFile{
			Path:   rel,
			Body:   body,
			Size:   int64(len(body)),
			SHA256: sha256Hex(body),
		})
		return nil
	})
	if err != nil {
		return inventory, err
	}
	sort.Slice(inventory.Files, func(i, j int) bool {
		return inventory.Files[i].Path < inventory.Files[j].Path
	})
	sort.Strings(inventory.SkippedFiles)
	if len(inventory.Files) > maxSourceFiles+1 {
		return inventory, errors.New("静态文件数量超过 500 个（含 manifest.json）")
	}
	return inventory, nil
}

func validateRelativePath(path string) error {
	if path == "" || strings.Contains(path, "\\") || strings.ContainsRune(path, 0) || strings.HasPrefix(path, "/") || len(path) > maxPathLength {
		return fmt.Errorf("部署包包含非法路径：%s", path)
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("部署包包含非法路径：%s", path)
		}
	}
	return nil
}

func validateAllowedFile(path string) error {
	lower := strings.ToLower(path)
	base := filepath.Base(lower)
	if base == ".env" || strings.HasPrefix(base, ".env.") || strings.HasSuffix(base, ".map") {
		return fmt.Errorf("禁止上传敏感或调试文件：%s", path)
	}
	ext := filepath.Ext(base)
	if _, allowed := allowedExtensions[ext]; !allowed {
		return fmt.Errorf("不允许的文件类型：%s", path)
	}
	return nil
}

func hasEntryFile(files []sourceFile) bool {
	for _, file := range files {
		if file.Path == "index.html" {
			return true
		}
	}
	return false
}

func validateEntryAndContent(files []sourceFile) error {
	var issues []string
	for _, file := range files {
		lower := strings.ToLower(file.Path)
		text := string(file.Body)
		if lower == "index.html" {
			issues = append(issues, validateEntryHTML(text)...)
			if rootAssetReference.MatchString(text) {
				issues = append(issues, "index.html 不允许根绝对资源路径，请使用相对路径")
			}
		}
		if strings.HasSuffix(lower, ".css") && cssRootAsset.MatchString(text) {
			issues = append(issues, "CSS 不允许根绝对资源路径："+file.Path)
		}
		if lower == "index.html" || strings.HasSuffix(lower, ".js") || strings.HasSuffix(lower, ".mjs") {
			for _, pattern := range myriadBoxReference {
				if pattern.MatchString(text) {
					issues = append(issues, "AndX 定制页不得依赖 MyriadBox 运行时或 /providers："+file.Path)
					break
				}
			}
		}
	}
	if len(issues) > 0 {
		return validationError(uniqueStrings(issues), nil)
	}
	return nil
}

func validateEntryHTML(html string) []string {
	var issues []string
	if baseTag.MatchString(html) {
		issues = append(issues, "index.html 不允许使用 base 标签")
	}
	if metaRefresh.MatchString(html) {
		issues = append(issues, "index.html 不允许使用 meta refresh 自动跳转")
	}
	if inlineEventHandler.MatchString(html) {
		issues = append(issues, "index.html 不允许内联事件处理器")
	}
	for _, match := range scriptBlock.FindAllStringSubmatch(html, -1) {
		attrs := match[1]
		body := strings.TrimSpace(match[2])
		if !scriptSource.MatchString(attrs) || body != "" {
			issues = append(issues, "index.html 必须使用外部脚本，不允许内联 script")
			break
		}
	}
	return issues
}

func validateCapabilities(requiredInput, optionalInput []string) ([]string, []string, []string) {
	var issues []string
	required, err := normalizeCapabilities(requiredInput, "required")
	if err != nil {
		issues = append(issues, err.Error())
	}
	optional, err := normalizeCapabilities(optionalInput, "optional")
	if err != nil {
		issues = append(issues, err.Error())
	}
	requiredSet := make(map[string]struct{}, len(required))
	for _, capability := range required {
		requiredSet[capability] = struct{}{}
	}
	for _, capability := range optional {
		if _, exists := requiredSet[capability]; exists {
			issues = append(issues, "Bridge 能力不能同时出现在 required 和 optional："+capability)
		}
	}
	if len(required)+len(optional) > 32 {
		issues = append(issues, "Bridge 能力总数不能超过 32 个")
	}
	return required, optional, uniqueStrings(issues)
}

func normalizeCapabilities(values []string, field string) ([]string, error) {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, duplicate := seen[value]; duplicate {
			return nil, fmt.Errorf("bridge.capabilities.%s 不能重复：%s", field, value)
		}
		if _, supported := supportedCapabilities[value]; !supported {
			return nil, fmt.Errorf("不支持的 Bridge 能力：%s", value)
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
