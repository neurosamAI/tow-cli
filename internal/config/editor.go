// editor.go — programmatic YAML config editing
// Modifies tow.yaml and tow.local.yaml without destroying comments or formatting.

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// AddModule adds a module definition to tow.yaml
func AddModule(configPath, name, moduleType string, port int, version string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", configPath, err)
	}

	content := string(data)

	// Check if module already exists
	if strings.Contains(content, fmt.Sprintf("  %s:", name)) {
		return fmt.Errorf("module %q already exists in %s", name, configPath)
	}

	// Build module YAML block
	var block strings.Builder
	block.WriteString(fmt.Sprintf("\n  %s:\n", name))
	block.WriteString(fmt.Sprintf("    type: %s\n", moduleType))
	if port > 0 {
		block.WriteString(fmt.Sprintf("    port: %d\n", port))
	}
	if version != "" {
		block.WriteString(fmt.Sprintf("    version: \"%s\"\n", version))
	}

	// Find "modules:" section and append
	idx := strings.Index(content, "modules:")
	if idx < 0 {
		// No modules section — add one
		content += "\nmodules:\n" + block.String()
	} else {
		// Append after last module entry
		content += block.String()
	}

	return os.WriteFile(configPath, []byte(content), 0644)
}

// RemoveModule removes a module definition from tow.yaml
func RemoveModule(configPath, name string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var result []string
	skip := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect module start: "  modulename:" at exactly 2-space indent
		if !skip && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") &&
			strings.HasSuffix(trimmed, ":") && strings.TrimSuffix(trimmed, ":") == name {
			skip = true
			continue
		}

		// Stop skipping when we hit next module (same indent level) or section
		if skip {
			if (strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && trimmed != "" && !strings.HasPrefix(trimmed, "#")) ||
				(!strings.HasPrefix(line, " ") && trimmed != "" && !strings.HasPrefix(trimmed, "#")) {
				skip = false
			} else {
				continue
			}
		}

		result = append(result, line)
	}

	return os.WriteFile(configPath, []byte(strings.Join(result, "\n")), 0644)
}

// AddServer adds a server to tow.local.yaml under the specified environment
func AddServer(localPath, envName, serverName, host string, number int, modules []string) error {
	// Ensure file exists
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		dir := filepath.Dir(localPath)
		os.MkdirAll(dir, 0755)
		initial := "# tow.local.yaml — server IPs and SSH keys (do not commit to git)\n\nenvironments:\n"
		os.WriteFile(localPath, []byte(initial), 0644)
	}

	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}

	content := string(data)

	// Build server entry
	var block strings.Builder
	block.WriteString(fmt.Sprintf("      - name: %s\n", serverName))
	if number > 0 {
		block.WriteString(fmt.Sprintf("        number: %d\n", number))
	}
	block.WriteString(fmt.Sprintf("        host: %s\n", host))
	if len(modules) > 0 {
		block.WriteString(fmt.Sprintf("        modules: [%s]\n", strings.Join(modules, ", ")))
	}

	// Find environment section
	envHeader := fmt.Sprintf("  %s:", envName)
	idx := strings.Index(content, envHeader)

	if idx < 0 {
		// Environment doesn't exist — add it
		content += fmt.Sprintf("\n  %s:\n    servers:\n%s", envName, block.String())
	} else {
		// Find "servers:" under this environment
		serversIdx := strings.Index(content[idx:], "servers:")
		if serversIdx < 0 {
			// No servers section — add one
			insertAt := idx + len(envHeader)
			// Find end of line
			nlIdx := strings.Index(content[insertAt:], "\n")
			if nlIdx >= 0 {
				insertAt += nlIdx + 1
			}
			content = content[:insertAt] + "    servers:\n" + block.String() + content[insertAt:]
		} else {
			// Find insertion point: after last server entry in this env
			absServersIdx := idx + serversIdx
			afterServers := content[absServersIdx:]

			// Find the next environment or end of file
			nextEnvIdx := -1
			lines := strings.Split(afterServers, "\n")
			lineOffset := 0
			for i, line := range lines {
				if i == 0 {
					lineOffset += len(line) + 1
					continue
				}
				trimmed := strings.TrimSpace(line)
				// Next env: line starts with "  " but not "      " (not a server property)
				if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && trimmed != "" && !strings.HasPrefix(trimmed, "#") && strings.HasSuffix(trimmed, ":") {
					nextEnvIdx = lineOffset
					break
				}
				lineOffset += len(line) + 1
			}

			if nextEnvIdx < 0 {
				// Append at end
				content += block.String()
			} else {
				insertAt := absServersIdx + nextEnvIdx
				content = content[:insertAt] + block.String() + content[insertAt:]
			}
		}
	}

	return os.WriteFile(localPath, []byte(content), 0644)
}

// RemoveServer removes a server from tow.local.yaml
func RemoveServer(localPath, envName, serverName string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	var result []string
	skip := false
	inTargetEnv := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track which environment we're in
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(trimmed, ":") {
			envN := strings.TrimSuffix(trimmed, ":")
			inTargetEnv = (envN == envName)
		}

		// Detect server entry: "      - name: xxx"
		if inTargetEnv && !skip && strings.Contains(trimmed, "- name: "+serverName) {
			skip = true
			continue
		}

		// Stop skipping at next server entry or section change
		if skip {
			if strings.HasPrefix(trimmed, "- name:") || strings.HasPrefix(trimmed, "- number:") {
				skip = false
			} else if !strings.HasPrefix(line, "        ") && !strings.HasPrefix(line, "      -") && trimmed != "" {
				skip = false
			} else {
				continue
			}
		}

		result = append(result, line)
	}

	return os.WriteFile(localPath, []byte(strings.Join(result, "\n")), 0644)
}

// AssignModules adds modules to a server's module list in tow.local.yaml
func AssignModules(localPath, envName, serverName string, newModules []string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}

	// Parse to get current modules
	var cfg struct {
		Environments map[string]struct {
			Servers []struct {
				Name    string   `yaml:"name"`
				Modules []string `yaml:"modules"`
			} `yaml:"servers"`
		} `yaml:"environments"`
	}

	expanded := os.Expand(string(data), func(key string) string {
		if val, ok := os.LookupEnv(key); ok {
			return val
		}
		return "${" + key + "}"
	})
	yaml.Unmarshal([]byte(expanded), &cfg)

	env, ok := cfg.Environments[envName]
	if !ok {
		return fmt.Errorf("environment %q not found in %s", envName, localPath)
	}

	var currentModules []string
	found := false
	for _, srv := range env.Servers {
		if srv.Name == serverName {
			currentModules = srv.Modules
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("server %q not found in environment %q", serverName, envName)
	}

	// Merge modules (add new, keep existing)
	moduleSet := make(map[string]bool)
	for _, m := range currentModules {
		moduleSet[m] = true
	}
	for _, m := range newModules {
		moduleSet[m] = true
	}

	var merged []string
	for m := range moduleSet {
		merged = append(merged, m)
	}

	// Replace modules line in the file
	content := string(data)
	// Find the server and replace its modules line
	lines := strings.Split(content, "\n")
	inTargetServer := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "- name: "+serverName) {
			inTargetServer = true
			continue
		}
		if inTargetServer && strings.HasPrefix(trimmed, "modules:") {
			lines[i] = fmt.Sprintf("        modules: [%s]", strings.Join(merged, ", "))
			break
		}
		if inTargetServer && (strings.HasPrefix(trimmed, "- name:") || (!strings.HasPrefix(line, "        ") && trimmed != "")) {
			break // no modules line found — add one
		}
	}

	return os.WriteFile(localPath, []byte(strings.Join(lines, "\n")), 0644)
}

// UnassignModules removes modules from a server's module list
func UnassignModules(localPath, envName, serverName string, removeModules []string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return err
	}

	removeSet := make(map[string]bool)
	for _, m := range removeModules {
		removeSet[m] = true
	}

	// Parse to get current modules
	var cfg struct {
		Environments map[string]struct {
			Servers []struct {
				Name    string   `yaml:"name"`
				Modules []string `yaml:"modules"`
			} `yaml:"servers"`
		} `yaml:"environments"`
	}

	expanded := os.Expand(string(data), func(key string) string {
		if val, ok := os.LookupEnv(key); ok {
			return val
		}
		return "${" + key + "}"
	})
	yaml.Unmarshal([]byte(expanded), &cfg)

	env, ok := cfg.Environments[envName]
	if !ok {
		return fmt.Errorf("environment %q not found", envName)
	}

	var currentModules []string
	for _, srv := range env.Servers {
		if srv.Name == serverName {
			currentModules = srv.Modules
			break
		}
	}

	var remaining []string
	for _, m := range currentModules {
		if !removeSet[m] {
			remaining = append(remaining, m)
		}
	}

	// Replace in file
	lines := strings.Split(string(data), "\n")
	inTargetServer := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "- name: "+serverName) {
			inTargetServer = true
			continue
		}
		if inTargetServer && strings.HasPrefix(trimmed, "modules:") {
			lines[i] = fmt.Sprintf("        modules: [%s]", strings.Join(remaining, ", "))
			break
		}
		if inTargetServer && strings.HasPrefix(trimmed, "- name:") {
			break
		}
	}

	return os.WriteFile(localPath, []byte(strings.Join(lines, "\n")), 0644)
}

// ListServers returns servers for an environment from tow.local.yaml
func ListServers(localPath, envName string) ([]Server, error) {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return nil, err
	}

	expanded := os.Expand(string(data), func(key string) string {
		if val, ok := os.LookupEnv(key); ok {
			return val
		}
		return "${" + key + "}"
	})

	var cfg struct {
		Environments map[string]*Environment `yaml:"environments"`
	}
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, err
	}

	env, ok := cfg.Environments[envName]
	if !ok {
		return nil, fmt.Errorf("environment %q not found", envName)
	}

	return env.Servers, nil
}
