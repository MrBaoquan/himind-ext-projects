package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Only MyriadBox's exact generated startup wrapper is removed. Business HTML,
// JS and resources are copied unchanged and still pass ordinary AndX checks.
var myriadStartup = regexp.MustCompile(`(?s)<script>window\.__MYRIADBOX_STARTUP__=(\{[^<]*\});</script><script src="([^"<>]+)"></script>`)

func prepareMyriadBoxStatic(project, source string) (string, error) {
	inventory, err := collectSourceFiles(source)
	if err != nil {
		return "", err
	}
	var entry []byte
	for _, file := range inventory.Files {
		if file.Path == "index.html" {
			entry = file.Body
		}
	}
	match := myriadStartup.FindSubmatchIndex(entry)
	if match == nil {
		return source, nil
	}
	if len(myriadStartup.FindAllIndex(entry, -1)) != 1 {
		return "", errors.New("MyriadBox 启动包装重复，请检查构建产物")
	}
	var config struct {
		Name           string `json:"name"`
		ExplicitReady  bool   `json:"explicitReady"`
		TimeoutSeconds int    `json:"timeoutSeconds"`
		Motion         string `json:"motion"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(entry[match[2]:match[3]])))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&config) != nil || config.TimeoutSeconds < 1 || config.TimeoutSeconds > 600 {
		return "", errors.New("MyriadBox 启动包装不匹配受支持契约")
	}
	startupPath := string(entry[match[4]:match[5]])
	if startupPath != "../sdk/brand/startup.js" {
		return "", errors.New("MyriadBox 启动脚本路径未识别，未自动删除")
	}
	entry = append(append([]byte{}, entry[:match[0]]...), entry[match[1]:]...)
	identity := sha256.New()
	for i := range inventory.Files {
		file := &inventory.Files[i]
		if file.Path == "index.html" {
			file.Body = entry
		}
		identity.Write([]byte(file.Path))
		identity.Write([]byte{0})
		identity.Write(file.Body)
		identity.Write([]byte{0})
	}
	if err := validateEntryAndContent(inventory.Files); err != nil {
		return "", err
	}
	digest := hex.EncodeToString(identity.Sum(nil))
	root := filepath.Join(project, "artifacts", "andx-static")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil || !pathWithin(project, root) {
		return "", errors.New("AndX 适配输出必须位于当前工程")
	}
	stage, err := os.MkdirTemp(root, ".candidate-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	for _, file := range inventory.Files {
		path := filepath.Join(stage, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, file.Body, 0o644); err != nil {
			return "", err
		}
	}
	output := filepath.Join(root, digest)
	if _, err := os.Lstat(output); err == nil {
		actual, err := filepath.EvalSymlinks(output)
		if err != nil || actual != output {
			return "", errors.New("AndX 适配输出冲突")
		}
		for _, file := range inventory.Files {
			body, err := os.ReadFile(filepath.Join(actual, filepath.FromSlash(file.Path)))
			if err != nil || string(body) != string(file.Body) {
				return "", errors.New("AndX 适配输出已改变，请使用新构建目录")
			}
		}
		return actual, nil
	}
	if err := os.Rename(stage, output); err != nil {
		return "", err
	}
	return output, nil
}
