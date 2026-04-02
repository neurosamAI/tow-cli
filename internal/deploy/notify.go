package deploy

import (
	"fmt"
	"os/exec"
	"time"
)

// Notify sends a notification about a deployment event
func (d *Deployer) Notify(envName, moduleName, event, message string) {
	cfg := d.cfg
	if cfg.Notifications == nil {
		return
	}

	for _, n := range cfg.Notifications {
		var curlCmd string
		text := fmt.Sprintf("[%s] %s/%s: %s — %s", cfg.Project.Name, envName, moduleName, event, message)

		switch n.Type {
		case "webhook":
			payload := fmt.Sprintf(`{"project":"%s","environment":"%s","module":"%s","event":"%s","message":"%s","timestamp":"%s"}`,
				cfg.Project.Name, envName, moduleName, event, message, time.Now().Format(time.RFC3339))
			curlCmd = fmt.Sprintf(`curl -sf -X POST -H "Content-Type: application/json" -d '%s' '%s'`, payload, n.URL)
		case "slack":
			payload := fmt.Sprintf(`{"text":"%s"}`, text)
			curlCmd = fmt.Sprintf(`curl -sf -X POST -H "Content-Type: application/json" -d '%s' '%s'`, payload, n.URL)
		case "discord":
			payload := fmt.Sprintf(`{"content":"%s"}`, text)
			curlCmd = fmt.Sprintf(`curl -sf -X POST -H "Content-Type: application/json" -d '%s' '%s'`, payload, n.URL)
		case "telegram":
			// Telegram Bot API: URL format should be https://api.telegram.org/bot<TOKEN>/sendMessage
			// n.URL = bot token, n.ChatID = chat ID
			// Or use full webhook URL: https://api.telegram.org/bot{TOKEN}/sendMessage?chat_id={CHAT_ID}
			payload := fmt.Sprintf(`{"chat_id":"%s","text":"%s","parse_mode":"HTML"}`, n.ChatID, text)
			apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.URL)
			curlCmd = fmt.Sprintf(`curl -sf -X POST -H "Content-Type: application/json" -d '%s' '%s'`, payload, apiURL)
		default:
			continue
		}
		go func(cmd string) {
			_ = runShell(cmd)
		}(curlCmd)
	}
}

// runShell runs a shell command locally (for notifications)
func runShell(command string) error {
	return exec.Command("sh", "-c", command).Run()
}
