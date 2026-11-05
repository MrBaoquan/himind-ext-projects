package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const maxExperienceSourceBytes = int64(2 << 20)

type experienceFinding struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type experienceCheck struct {
	ID       string              `json:"id"`
	Status   string              `json:"status"`
	Summary  string              `json:"summary"`
	Count    int                 `json:"count"`
	Findings []experienceFinding `json:"findings,omitempty"`
}

type experienceSourceFile struct {
	RelativePath string
	Lines        []string
}

type forbiddenCopyRule struct {
	Token  string
	Advice string
}

var experienceSourceExtensions = map[string]bool{
	".html": true, ".htm": true, ".css": true, ".scss": true,
	".js": true, ".mjs": true, ".cjs": true, ".ts": true,
	".jsx": true, ".tsx": true, ".vue": true, ".svelte": true,
	".json": true,
}

var experienceSkippedDirectories = map[string]bool{
	".git": true, ".svn": true, ".himind": true, ".tmp": true,
	"node_modules": true, "dist": true, "build": true, "artifacts": true,
	"coverage": true, "logs": true, "media": true, "providers": true, "bin": true, "obj": true,
}

var forbiddenExperienceCopy = []forbiddenCopyRule{
	{Token: "开启一段", Advice: "改成游客可以立即执行的动作"},
	{Token: "奇妙旅程", Advice: "删除空泛旅程比喻，补充具体对象和动作"},
	{Token: "无限可能", Advice: "删除不可验收的宏大表述"},
	{Token: "未来已来", Advice: "删除口号，写本展项实际发生的事"},
	{Token: "科技与艺术", Advice: "删除放之四海皆准的并列套话"},
	{Token: "赋能", Advice: "改成具体能力或直接删除"},
	{Token: "重新定义", Advice: "删除营销承诺，写可观察结果"},
	{Token: "解锁专属", Advice: "改成明确操作与结果"},
	{Token: "敬请期待", Advice: "写明当前不可用原因和下一步"},
	{Token: "choose your route", Advice: "改成直接的中文动作"},
	{Token: "crafting your memory", Advice: "改成真实进度说明"},
	{Token: "your postcard is ready", Advice: "改成结果与下一步"},
}

var (
	buttonPattern       = regexp.MustCompile(`(?is)<button\b([^>]*)>(.*?)</button>`)
	buttonAriaPattern   = regexp.MustCompile(`(?i)\baria-label\s*=\s*["']([^"']+)["']`)
	tagPattern          = regexp.MustCompile(`(?s)<[^>]+>`)
	genericButtonLabels = map[string]bool{
		"go": true, "start": true, "continue": true, "next": true,
		"enter": true, "begin": true, "submit": true, "ok": true,
	}
)

func inspectWebExperience(root string) (map[string]any, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("project_root is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("project_root cannot be read: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("project_root must be a directory")
	}

	sources, err := scanExperienceSources(abs)
	if err != nil {
		return nil, err
	}
	checks := []experienceCheck{
		checkForbiddenExperienceCopy(sources),
		checkGenericExperienceButtons(sources),
		checkGenericExperienceErrors(sources),
		checkMotionsDoNotDependOnAnimationEvents(sources),
		checkExperienceTransitionTiming(sources),
		checkReducedMotion(sources),
		checkExperienceDynamicSignals(sources),
	}

	failed, warnings := 0, 0
	nextActions := make([]string, 0)
	for _, check := range checks {
		switch check.Status {
		case "fail":
			failed++
		case "warn":
			warnings++
		}
	}
	if failed > 0 {
		nextActions = append(nextActions,
			"按 findings 修改文案、转场或 reduced-motion 后重新运行检查",
			"不要通过删除检查或隐藏违规文件绕过门禁",
		)
	}
	if warnings > 0 {
		nextActions = append(nextActions, "复核警告项，并在体验证据中说明保留理由")
	}

	return map[string]any{
		"schema_version": "exhibit-web-experience-inspection.v1",
		"project_root":   abs,
		"ok":             failed == 0,
		"scanned_files":  len(sources),
		"summary": map[string]any{
			"passed":   len(checks) - failed - warnings,
			"warnings": warnings,
			"failed":   failed,
		},
		"checks":       checks,
		"next_actions": nextActions,
	}, nil
}

func scanExperienceSources(root string) ([]experienceSourceFile, error) {
	sources := make([]experienceSourceFile, 0)
	hasFrameworkSource := pathIsDir(filepath.Join(root, "src"))
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path == root {
				return nil
			}
			name := strings.ToLower(entry.Name())
			if experienceSkippedDirectories[name] || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			relative, _ := filepath.Rel(root, path)
			relativeSlash := filepath.ToSlash(relative)
			if relativeSlash == "config/private" {
				return filepath.SkipDir
			}
			if hasFrameworkSource && (relativeSlash == "wwwroot/assets" || relativeSlash == "public/sdk") {
				return filepath.SkipDir
			}
			return nil
		}
		lowerName := strings.ToLower(entry.Name())
		if !experienceSourceExtensions[strings.ToLower(filepath.Ext(lowerName))] ||
			strings.Contains(lowerName, ".min.") ||
			strings.Contains(lowerName, ".spec.") ||
			strings.Contains(lowerName, ".test.") ||
			strings.HasSuffix(lowerName, "-lock.json") ||
			strings.HasSuffix(lowerName, ".map") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() <= 0 || info.Size() > maxExperienceSourceBytes {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		normalized := strings.ReplaceAll(string(content), "\r\n", "\n")
		normalized = strings.ReplaceAll(normalized, "\r", "\n")
		sources = append(sources, experienceSourceFile{
			RelativePath: filepath.ToSlash(relative),
			Lines:        strings.Split(normalized, "\n"),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan exhibit sources: %w", err)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].RelativePath < sources[j].RelativePath })
	return sources, nil
}

func checkForbiddenExperienceCopy(sources []experienceSourceFile) experienceCheck {
	findings := make([]experienceFinding, 0)
	advice := make([]string, 0)
	for _, source := range sources {
		for lineNumber, line := range source.Lines {
			lower := strings.ToLower(line)
			for _, rule := range forbiddenExperienceCopy {
				if strings.Contains(lower, strings.ToLower(rule.Token)) {
					findings = appendExperienceFinding(findings, experienceFinding{
						Path: source.RelativePath,
						Line: lineNumber + 1,
						Text: truncateExperienceLine(line),
					})
					if !containsString(advice, rule.Advice) {
						advice = append(advice, rule.Advice)
					}
					break
				}
			}
		}
	}
	status := "pass"
	summary := "未发现禁用套话"
	if len(findings) > 0 {
		status = "fail"
		summary = "发现空泛或替代操作说明的文案；" + strings.Join(advice, "；")
	}
	return experienceCheck{
		ID:       "copy.forbidden",
		Status:   status,
		Summary:  summary,
		Count:    len(findings),
		Findings: findings,
	}
}

func checkGenericExperienceButtons(sources []experienceSourceFile) experienceCheck {
	findings := make([]experienceFinding, 0)
	for _, source := range sources {
		content := strings.Join(source.Lines, "\n")
		for _, match := range buttonPattern.FindAllStringSubmatchIndex(content, -1) {
			attributes := content[match[2]:match[3]]
			inner := content[match[4]:match[5]]
			visible := strings.TrimSpace(tagPattern.ReplaceAllString(inner, " "))
			if visible == "" || strings.Contains(visible, "{{") || strings.Contains(strings.ToLower(attributes), "v-html") {
				if buttonAriaPattern.MatchString(attributes) {
					continue
				}
				findings = appendExperienceFinding(findings, experienceFinding{
					Path: source.RelativePath,
					Line: lineNumberAt(content, match[0]),
					Text: "<button> 缺少可读文字或 aria-label",
				})
				continue
			}
			if genericButtonLabels[strings.ToLower(visible)] {
				findings = appendExperienceFinding(findings, experienceFinding{
					Path: source.RelativePath,
					Line: lineNumberAt(content, match[0]),
					Text: fmt.Sprintf("按钮文案“%s”只写方向，未说明动作", truncateExperienceLine(visible)),
				})
			}
		}
	}
	status := "pass"
	summary := "按钮文案包含可读动作"
	if len(findings) > 0 {
		status = "fail"
		summary = "存在缺少动作或不可读的按钮"
	}
	return experienceCheck{ID: "copy.buttons", Status: status, Summary: summary, Count: len(findings), Findings: findings}
}

func checkGenericExperienceErrors(sources []experienceSourceFile) experienceCheck {
	genericErrors := []string{"系统异常", "操作失败", "未知错误", "发生错误", "出错了"}
	findings := make([]experienceFinding, 0)
	for _, source := range sources {
		for lineNumber, line := range source.Lines {
			lower := strings.ToLower(line)
			for _, phrase := range genericErrors {
				if strings.Contains(lower, strings.ToLower(phrase)) {
					findings = appendExperienceFinding(findings, experienceFinding{
						Path: source.RelativePath,
						Line: lineNumber + 1,
						Text: truncateExperienceLine(line),
					})
					break
				}
			}
		}
	}
	status := "pass"
	summary := "错误文案未使用无原因的通用套话"
	if len(findings) > 0 {
		status = "fail"
		summary = "错误文案只有泛化结论；补充原因、内容状态和下一步"
	}
	return experienceCheck{ID: "copy.error-states", Status: status, Summary: summary, Count: len(findings), Findings: findings}
}

func checkExperienceTransitionTiming(sources []experienceSourceFile) experienceCheck {
	transitionPattern := regexp.MustCompile(`(?i)transition(?:-duration)?\s*:\s*([^;{}\n]+)`)
	durationPattern := regexp.MustCompile(`(?i)([0-9]+(?:\.[0-9]+)?)(ms|s)`)
	findings := make([]experienceFinding, 0)
	for _, source := range sources {
		for lineNumber, line := range source.Lines {
			match := transitionPattern.FindStringSubmatch(line)
			if len(match) < 2 {
				continue
			}
			maxDuration := 0.0
			for _, duration := range durationPattern.FindAllStringSubmatch(match[1], -1) {
				value := 0.0
				_, _ = fmt.Sscanf(duration[1], "%f", &value)
				if strings.EqualFold(duration[2], "s") {
					value *= 1000
				}
				if value > maxDuration {
					maxDuration = value
				}
			}
			if maxDuration > 800 {
				findings = appendExperienceFinding(findings, experienceFinding{
					Path: source.RelativePath,
					Line: lineNumber + 1,
					Text: truncateExperienceLine(line),
				})
			}
		}
	}
	status := "pass"
	summary := "未发现超过 800ms 的 CSS 转场"
	if len(findings) > 0 {
		status = "warn"
		summary = "发现超过 800ms 的 CSS 转场；确认它传达了必要信息并补充证据"
	}
	return experienceCheck{ID: "motion.transition-timing", Status: status, Summary: summary, Count: len(findings), Findings: findings}
}
func checkMotionsDoNotDependOnAnimationEvents(sources []experienceSourceFile) experienceCheck {
	needles := []string{"transitionend", "animationend", "webkittransitionend", "webkitAnimationEnd"}
	return checkExperienceNeedles(sources, "motion.animation-events", needles,
		"未用动画结束事件推进业务", "发现以动画结束事件推进流程的风险；业务必须用显式状态或计时器")
}

func checkReducedMotion(sources []experienceSourceFile) experienceCheck {
	found := false
	for _, source := range sources {
		if strings.Contains(strings.ToLower(strings.Join(source.Lines, "\n")), "prefers-reduced-motion") {
			found = true
			break
		}
	}
	if found {
		return experienceCheck{ID: "motion.reduced-motion", Status: "pass", Summary: "存在 prefers-reduced-motion 分支"}
	}
	return experienceCheck{
		ID:       "motion.reduced-motion",
		Status:   "fail",
		Summary:  "未找到 prefers-reduced-motion；必须为低动态偏好提供可见且可操作的替代",
		Count:    1,
		Findings: []experienceFinding{{Path: ".", Line: 0, Text: "missing prefers-reduced-motion"}},
	}
}

func checkExperienceDynamicSignals(sources []experienceSourceFile) experienceCheck {
	signals := map[string]string{
		"css-keyframes":           "@keyframes",
		"css-transition":          "transition:",
		"request-animation-frame": "requestanimationframe",
		"timed-interval":          "setinterval(",
		"web-animation":           ".animate(",
	}
	found := make([]string, 0)
	all := strings.ToLower(joinExperienceSources(sources))
	for name, needle := range signals {
		if strings.Contains(all, needle) {
			found = append(found, name)
		}
	}
	sort.Strings(found)
	status := "pass"
	summary := "检测到至少两类动态信号"
	if len(found) < 2 {
		status = "fail"
		summary = "动态信号少于两类；检查是否缺少环境、状态或交互反馈"
	}
	return experienceCheck{
		ID:      "motion.live-signals",
		Status:  status,
		Summary: summary,
		Count:   len(found),
		Findings: []experienceFinding{{
			Path: ".", Line: 0, Text: "signals=" + strings.Join(found, ","),
		}},
	}
}

func checkExperienceNeedles(sources []experienceSourceFile, id string, needles []string, passSummary, failSummary string) experienceCheck {
	findings := make([]experienceFinding, 0)
	for _, source := range sources {
		for lineNumber, line := range source.Lines {
			lower := strings.ToLower(line)
			for _, needle := range needles {
				if strings.Contains(lower, strings.ToLower(needle)) {
					findings = appendExperienceFinding(findings, experienceFinding{
						Path: source.RelativePath,
						Line: lineNumber + 1,
						Text: truncateExperienceLine(line),
					})
					break
				}
			}
		}
	}
	status := "pass"
	summary := passSummary
	if len(findings) > 0 {
		status = "fail"
		summary = failSummary
	}
	return experienceCheck{ID: id, Status: status, Summary: summary, Count: len(findings), Findings: findings}
}

func appendExperienceFinding(findings []experienceFinding, finding experienceFinding) []experienceFinding {
	if len(findings) >= 100 {
		return findings
	}
	for _, existing := range findings {
		if existing.Path == finding.Path && existing.Line == finding.Line && existing.Text == finding.Text {
			return findings
		}
	}
	return append(findings, finding)
}

func joinExperienceSources(sources []experienceSourceFile) string {
	var builder strings.Builder
	for _, source := range sources {
		builder.WriteString(strings.Join(source.Lines, "\n"))
		builder.WriteByte('\n')
	}
	return builder.String()
}

func lineNumberAt(content string, offset int) int {
	if offset <= 0 {
		return 1
	}
	return strings.Count(content[:offset], "\n") + 1
}

func truncateExperienceLine(line string) string {
	trimmed := strings.TrimSpace(line)
	runes := []rune(trimmed)
	if len(runes) <= 240 {
		return trimmed
	}
	return string(runes[:240]) + "…"
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
