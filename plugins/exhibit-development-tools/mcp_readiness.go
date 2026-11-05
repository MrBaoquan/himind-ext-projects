package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// unityMCPPackageName 是 Unity 侧包名，用于从 Packages/manifest.json 读取工程 pin 的版本或引用。
const unityMCPPackageName = "com.coplaydev.unity-mcp"

// unityMCPStatusSchemaVersion 标记本段输出的结构，便于调用方区分版本。
const unityMCPStatusSchemaVersion = "exhibit-unity-mcp-status.v1"

// unityMCPHeartbeat 对应 MCP for Unity 写入的状态文件字段。
type unityMCPHeartbeat struct {
	UnityPort    int    `json:"unity_port"`
	Reloading    bool   `json:"reloading"`
	Reason       string `json:"reason"`
	Seq          int    `json:"seq"`
	ProjectPath  string `json:"project_path"`
	ProjectName  string `json:"project_name"`
	UnityVersion string `json:"unity_version"`
	LastHeartbeat string `json:"last_heartbeat"`
}

// unityMCPStatus 是本插件对 MCP 就绪情况的结构化结论。
type unityMCPStatus struct {
	SchemaVersion  string   `json:"schema_version"`
	Ready          bool     `json:"ready"`
	State          string   `json:"state"`
	PackageRef     string   `json:"package_ref,omitempty"`
	PackageVersion string   `json:"package_version,omitempty"`
	BridgePort     int      `json:"bridge_port,omitempty"`
	HeartbeatFile  string   `json:"heartbeat_file,omitempty"`
	HeartbeatAgeS  float64  `json:"heartbeat_age_seconds,omitempty"`
	ProjectPath    string   `json:"project_path,omitempty"`
	UnityVersion   string   `json:"unity_version,omitempty"`
	VersionMatch   *bool    `json:"version_match,omitempty"`
	Notes          []string `json:"notes,omitempty"`
	Blockers       []string `json:"blockers,omitempty"`
}

// statusDir 返回 MCP for Unity 的状态文件目录，遵循其自身的环境变量覆盖规则。
func statusDir() string {
	if dir := strings.TrimSpace(os.Getenv("UNITY_MCP_STATUS_DIR")); dir != "" {
		return dir
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".unity-mcp")
}

// readUnityMCPPackageRef 从 Packages/manifest.json 读取工程 pin 的 MCP 包引用。
// 返回原始引用字符串（Git URL 时含 tag）与可比较的版本号。
func readUnityMCPPackageRef(projectRoot string) (string, string) {
	manifestPath := filepath.Join(projectRoot, "Packages", "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", ""
	}
	var manifest struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", ""
	}
	ref := strings.TrimSpace(manifest.Dependencies[unityMCPPackageName])
	if ref == "" {
		return "", ""
	}
	return ref, versionFromPackageRef(ref)
}

// versionFromPackageRef 从 `10.2.0`、`https://...#v10.2.0`、`...#<sha>` 中提取版本号。
// 无法解析出版本时返回空串，调用方据此跳过一致性判定。
func versionFromPackageRef(ref string) string {
	if ref == "" {
		return ""
	}
	// 纯版本号，例如 "10.2.0"。
	if !strings.Contains(ref, "/") && !strings.Contains(ref, ":") {
		return strings.TrimPrefix(ref, "v")
	}
	if idx := strings.LastIndex(ref, "#"); idx >= 0 && idx+1 < len(ref) {
		tag := strings.TrimPrefix(ref[idx+1:], "v")
		// 只接受形如 x.y.z 的 tag，commit hash 无法与客户端版本比较。
		if looksLikeSemver(tag) {
			return tag
		}
	}
	return ""
}

// looksLikeSemver 判断是否为形如 10.2.0 的版本串。
func looksLikeSemver(s string) bool {
	if s == "" {
		return false
	}
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, r := range p {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// inspectUnityMCP 汇总 MCP 就绪情况：工程 pin、心跳文件、端口与版本一致性。
//
// 判定口径与 unihper-unity-development 13.1 保持一致：
//   - 先读心跳文件，再谈工具是否可用；
//   - 多个工程的 status 文件同时存在时判定为多实例，要求显式选择实例；
//   - reloading 期间不算就绪。
func inspectUnityMCP(projectRoot string, unityVersion string) unityMCPStatus {
	status := unityMCPStatus{
		SchemaVersion: unityMCPStatusSchemaVersion,
		State:         "unknown",
	}

	ref, version := readUnityMCPPackageRef(projectRoot)
	status.PackageRef = ref
	status.PackageVersion = version
	if ref == "" {
		status.Notes = append(status.Notes, "Packages/manifest.json 未声明 "+unityMCPPackageName+"，MCP 自动化能力不可用")
	}

	dir := statusDir()
	if dir == "" {
		status.State = "unknown"
		status.Blockers = append(status.Blockers, "无法确定 MCP 状态目录")
		return status
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		status.State = "not_running"
		status.Notes = append(status.Notes, "状态目录不可读或不存在："+dir)
		return status
	}

	expectedProjectPath := filepath.ToSlash(filepath.Join(projectRoot, "Assets"))
	matched := make([]unityMCPHeartbeat, 0, 2)
	matchedFiles := make([]string, 0, 2)
	otherProjects := make([]string, 0, 4)

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "unity-mcp-status-") || !strings.HasSuffix(name, ".json") {
			continue
		}
		filePath := filepath.Join(dir, name)
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		var hb unityMCPHeartbeat
		if err := json.Unmarshal(data, &hb); err != nil {
			continue
		}
		if filepath.ToSlash(strings.TrimSpace(hb.ProjectPath)) == expectedProjectPath {
			matched = append(matched, hb)
			matchedFiles = append(matchedFiles, filePath)
			continue
		}
		if hb.ProjectName != "" {
			otherProjects = append(otherProjects, hb.ProjectName)
		}
	}

	switch {
	case len(matched) == 0:
		status.State = "not_running"
		status.Notes = append(status.Notes, "未找到本工程的心跳文件，Unity 可能尚未打开")
		if len(otherProjects) > 0 {
			status.Notes = append(status.Notes, "当前其它已打开工程："+strings.Join(otherProjects, ", "))
		}
	case len(matched) > 1:
		status.State = "multiple_instances"
		status.HeartbeatFile = matchedFiles[0]
		status.Blockers = append(status.Blockers, "同一工程存在多个心跳文件，必须先显式选择目标 Unity 实例")
	default:
		hb := matched[0]
		status.HeartbeatFile = matchedFiles[0]
		status.BridgePort = hb.UnityPort
		status.ProjectPath = hb.ProjectPath
		status.UnityVersion = hb.UnityVersion
		if age, ok := heartbeatAgeSeconds(hb.LastHeartbeat); ok {
			status.HeartbeatAgeS = age
		}
		if hb.Reloading {
			status.State = "reloading"
			status.Notes = append(status.Notes, "Unity 正在域重载，等待就绪后再调用工具")
		} else {
			status.State = "ready"
			status.Ready = true
		}
		if len(otherProjects) > 0 {
			status.Notes = append(status.Notes, "其它工程也在运行，操作前必须指定目标实例："+strings.Join(otherProjects, ", "))
		}
	}

	// 版本一致性：心跳里的 Unity 版本 vs ProjectVersion.txt。
	if status.UnityVersion != "" && unityVersion != "" {
		match := status.UnityVersion == unityVersion
		status.VersionMatch = &match
		if !match {
			status.Ready = false
			status.State = "version_mismatch"
			status.Blockers = append(status.Blockers,
				"engine 版本不一致：心跳 "+status.UnityVersion+" / 工程 "+unityVersion+"，拿错版本会触发不可逆的工程升级")
		}
	}

	return status
}

// heartbeatAgeSeconds 把心跳时间换算成距今秒数，用于判断状态文件是否已过期。
func heartbeatAgeSeconds(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return 0, false
	}
	return time.Since(t).Seconds(), true
}
