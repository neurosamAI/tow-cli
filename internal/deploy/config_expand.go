package deploy

import (
	"os"
	"path/filepath"
	"strings"
)

// expandConfigDir copies a config directory to a temp location,
// expanding ${VAR} environment variables in all text files.
// This allows secrets to live in env vars instead of config files committed to git.
func expandConfigDir(srcDir string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "tow-config-*")
	if err != nil {
		return "", err
	}

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(srcDir, path)
		destPath := filepath.Join(tmpDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		// Read file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		content := string(data)

		// Only expand if file contains ${...} patterns (skip binaries)
		if strings.Contains(content, "${") {
			content = os.ExpandEnv(content)
		}

		return os.WriteFile(destPath, []byte(content), info.Mode())
	})

	if err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	return tmpDir, nil
}
