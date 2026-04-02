package deploy

import (
	"fmt"
	"strings"

	"github.com/neurosamAI/tow-cli/internal/logger"
)

// Rollback switches the current symlink to a previous deployment
func (d *Deployer) Rollback(envName, moduleName string, serverNum int, target string) error {
	servers, env, err := d.cfg.GetServersForModule(envName, moduleName, serverNum)
	if err != nil {
		return err
	}

	for _, srv := range servers {
		baseDir := d.RemoteBaseDirForServer(moduleName, srv)
		logger.ServerAction(srv.Host, "Rolling back %s", moduleName)

		var rollbackCmd string
		if target != "" {
			rollbackCmd = fmt.Sprintf(`
set -e
if [ ! -d "%s/deploy/%s" ]; then
    echo "ERROR: deployment %s not found"
    exit 1
fi
cd %s
rm -f current
ln -s deploy/%s current
echo "ROLLBACK_OK to %s"
`, baseDir, target, target, baseDir, target, target)
		} else {
			rollbackCmd = fmt.Sprintf(`
set -e
CURRENT=$(readlink %s/current 2>/dev/null | xargs basename)
PREVIOUS=$(ls -1t %s/deploy/ | grep -v "^$CURRENT$" | head -1)

if [ -z "$PREVIOUS" ]; then
    echo "ERROR: no previous deployment found"
    exit 1
fi

cd %s
rm -f current
ln -s deploy/$PREVIOUS current
echo "ROLLBACK_OK from $CURRENT to $PREVIOUS"
`, baseDir, baseDir, baseDir)
		}

		result, err := d.ssh.Exec(env, srv.Host, rollbackCmd)
		if err != nil {
			return fmt.Errorf("[%s] rollback failed: %w", srv.Host, err)
		}
		if strings.Contains(result.Stdout, "ERROR:") {
			return fmt.Errorf("[%s] %s", srv.Host, strings.TrimSpace(result.Stdout))
		}

		logger.Success("[%s] %s", srv.Host, strings.TrimSpace(result.Stdout))
	}

	// Restart after rollback
	logger.Info("Restarting after rollback...")
	return d.Restart(envName, moduleName, serverNum)
}
