package utils

import (
	"context"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Renderer преобразует JSON-снимок черновика storage в bundle статических файлов.
type Renderer interface {
	Render(ctx context.Context, draftJSON []byte) ([]Blob, error)
}

// CLIRenderer вызывает landing-builder-cli v2 (Go + Astro).
type CLIRenderer struct {
	CLIPath string
}

// NewCLIRenderer создаёт рендерер с путём к CLI-бинарнику.
func NewCLIRenderer(cliPath string) *CLIRenderer {
	return &CLIRenderer{CLIPath: cliPath}
}

func (r *CLIRenderer) Render(ctx context.Context, draftJSON []byte) ([]Blob, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "publish-render-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	jsonPath := filepath.Join(tmpDir, "draft.json")
	outputDir := filepath.Join(tmpDir, "dist")

	if err := os.WriteFile(jsonPath, draftJSON, 0o600); err != nil {
		return nil, fmt.Errorf("failed to write draft json: %w", err)
	}

	cmd := exec.CommandContext(ctx, r.CLIPath, "build", "--draft", jsonPath, "--output", outputDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to run cli render: %w: %s", err, string(out))
	}

	blobs, err := collectOutputBlobs(outputDir)
	if err != nil {
		return nil, err
	}
	if len(blobs) == 0 {
		return nil, fmt.Errorf("cli render produced no files in %s", outputDir)
	}

	hasIndex := false
	for _, blob := range blobs {
		if blob.Path == "index.html" {
			hasIndex = true
			break
		}
	}
	if !hasIndex {
		return nil, fmt.Errorf("cli render output missing index.html")
	}

	return blobs, nil
}

func collectOutputBlobs(rootDir string) ([]Blob, error) {
	var blobs []Blob

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return fmt.Errorf("failed to resolve output path: %w", err)
		}
		relPath = filepath.ToSlash(relPath)

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read rendered file %s: %w", relPath, err)
		}

		blobs = append(blobs, Blob{
			Path:        relPath,
			Content:     content,
			ContentType: contentTypeForPath(relPath),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to collect rendered files: %w", err)
	}

	return blobs, nil
}

func contentTypeForPath(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return "application/octet-stream"
	}

	contentType := mime.TypeByExtension(ext)
	if contentType != "" {
		return contentType
	}

	switch strings.ToLower(ext) {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js", ".mjs":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".woff2":
		return "font/woff2"
	case ".woff":
		return "font/woff"
	case ".ttf":
		return "font/ttf"
	default:
		return "application/octet-stream"
	}
}
