package utils

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Renderer преобразует JSON-снимок черновика storage в HTML.
type Renderer interface {
	Render(ctx context.Context, draftJSON []byte) ([]byte, error)
}

// CLIRenderer вызывает landing-builder-cli (generate.py).
type CLIRenderer struct {
	CLIPath string
}

// NewCLIRenderer создаёт рендерер с путём к CLI-скрипту.
func NewCLIRenderer(cliPath string) *CLIRenderer {
	return &CLIRenderer{CLIPath: cliPath}
}

func (r *CLIRenderer) Render(ctx context.Context, draftJSON []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "publish-render-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	jsonPath := filepath.Join(tmpDir, "draft.json")
	htmlPath := filepath.Join(tmpDir, "index.html")

	if err := os.WriteFile(jsonPath, draftJSON, 0o600); err != nil {
		return nil, fmt.Errorf("write draft json: %w", err)
	}

	cmd := exec.CommandContext(ctx, "python3", r.CLIPath, "--draft", jsonPath, "--output", htmlPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("cli render failed: %w: %s", err, string(out))
	}

	html, err := os.ReadFile(htmlPath)
	if err != nil {
		return nil, fmt.Errorf("read rendered html: %w", err)
	}
	return html, nil
}
