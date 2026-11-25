package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxArchiveBytes = 50 * 1024 * 1024

var fixedZipTime = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

func packagePage(in input) (map[string]any, error) {
	_, _, outputZip, err := resolveProjectPaths(in, true)
	if err != nil {
		return nil, err
	}
	page, err := validatePageInput(in)
	if err != nil {
		return nil, err
	}
	if page.BuiltAt == "" {
		page.BuiltAt = time.Now().UTC().Format(time.RFC3339)
	} else if _, err := time.Parse(time.RFC3339, page.BuiltAt); err != nil {
		return nil, fmt.Errorf("built_at 必须是 RFC3339 时间：%w", err)
	}
	manifest := buildPageManifest(page)
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("生成 manifest.json 失败：%w", err)
	}
	manifestBytes = append(manifestBytes, '\n')
	if page.Inventory.TotalBytes+int64(len(manifestBytes)) > maxUncompressedBytes {
		return nil, errors.New("静态产物加 manifest 后不能超过 100MB")
	}
	if len(page.Inventory.Files)+1 > 500 {
		return nil, errors.New("静态文件数量超过 500 个（含 manifest.json）")
	}

	if err := os.MkdirAll(filepath.Dir(outputZip), 0o755); err != nil {
		return nil, fmt.Errorf("创建输出目录失败：%w", err)
	}
	temp, err := os.CreateTemp(filepath.Dir(outputZip), ".andx-page-*.zip")
	if err != nil {
		return nil, fmt.Errorf("创建临时 ZIP 失败：%w", err)
	}
	tempPath := temp.Name()
	keepTemp := true
	defer func() {
		_ = temp.Close()
		if keepTemp {
			_ = os.Remove(tempPath)
		}
	}()

	zipWriter := zip.NewWriter(temp)
	for _, file := range page.Inventory.Files {
		if err := writeZipEntry(zipWriter, file.Path, file.Body); err != nil {
			_ = zipWriter.Close()
			return nil, err
		}
	}
	if err := writeZipEntry(zipWriter, "manifest.json", manifestBytes); err != nil {
		_ = zipWriter.Close()
		return nil, err
	}
	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("写入 ZIP 失败：%w", err)
	}
	if err := temp.Close(); err != nil {
		return nil, fmt.Errorf("关闭 ZIP 失败：%w", err)
	}
	info, err := os.Stat(tempPath)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxArchiveBytes {
		return nil, errors.New("部署包不能超过 50MB")
	}
	if _, err := os.Stat(outputZip); err == nil {
		if !in.Force {
			return nil, errors.New("输出 ZIP 已存在；传入 force=true 覆盖")
		}
		if err := os.Remove(outputZip); err != nil {
			return nil, fmt.Errorf("覆盖输出 ZIP 失败：%w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.Rename(tempPath, outputZip); err != nil {
		return nil, fmt.Errorf("保存输出 ZIP 失败：%w", err)
	}
	keepTemp = false
	archiveBody, err := os.ReadFile(outputZip)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(archiveBody)
	return map[string]any{
		"ok":                    true,
		"output_zip":            outputZip,
		"sha256":                hex.EncodeToString(sum[:]),
		"artifact_bytes":        len(archiveBody),
		"file_count":            len(page.Inventory.Files) + 1,
		"source_root":           page.Inventory.Root,
		"source_kind":           page.Inventory.SourceKind,
		"page_key":              page.PageKey,
		"version":               page.Version,
		"display_mode":          page.DisplayMode,
		"bridge_version":        bridgeVersion,
		"required_capabilities": page.RequiredCapabilities,
		"optional_capabilities": page.OptionalCapabilities,
		"skipped_runtime_files": page.Inventory.SkippedFiles,
		"warnings":              page.Inventory.Warnings,
		"manifest":              manifest,
	}, nil
}

func buildPageManifest(page *validatedPage) pageManifest {
	files := make([]manifestFile, 0, len(page.Inventory.Files))
	for _, file := range page.Inventory.Files {
		files = append(files, manifestFile{
			Path:   file.Path,
			Size:   file.Size,
			SHA256: file.SHA256,
		})
	}
	return pageManifest{
		PageActions: page.PageActions,
		PageKey:     page.PageKey,
		Version:     page.Version,
		DisplayMode: page.DisplayMode,
		EntryFile:   "index.html",
		BuiltAt:     page.BuiltAt,
		Bridge: bridgeManifest{
			Version: bridgeVersion,
			Capabilities: bridgeCapabilitiesManifest{
				Required: page.RequiredCapabilities,
				Optional: page.OptionalCapabilities,
			},
		},
		Files: files,
	}
}

func writeZipEntry(writer *zip.Writer, name string, body []byte) error {
	name = strings.ReplaceAll(name, "\\", "/")
	header := &zip.FileHeader{
		Name:     name,
		Method:   zip.Deflate,
		Modified: fixedZipTime,
	}
	header.SetMode(0o644)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("创建 ZIP 条目失败 %s：%w", name, err)
	}
	if _, err := bytes.NewReader(body).WriteTo(entry); err != nil {
		return fmt.Errorf("写入 ZIP 条目失败 %s：%w", name, err)
	}
	return nil
}
