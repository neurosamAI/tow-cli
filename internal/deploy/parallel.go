package deploy

import (
	"fmt"
	"sync"

	"github.com/neurosamAI/tow-cli/internal/config"
	"github.com/neurosamAI/tow-cli/internal/logger"
)

// ParallelResult holds the result of a parallel operation on a single server
type ParallelResult struct {
	Host  string
	Error error
}

// RunParallel executes fn on multiple servers concurrently
func (d *Deployer) RunParallel(servers []config.Server, env *config.Environment, fn func(srv config.Server) error) []ParallelResult {
	if len(servers) <= 1 {
		// Single server, run directly
		results := make([]ParallelResult, len(servers))
		for i, srv := range servers {
			results[i] = ParallelResult{
				Host:  srv.Host,
				Error: fn(srv),
			}
		}
		return results
	}

	logger.Info("Running on %d servers in parallel...", len(servers))

	var wg sync.WaitGroup
	results := make([]ParallelResult, len(servers))

	for i, srv := range servers {
		wg.Add(1)
		go func(idx int, s config.Server) {
			defer wg.Done()
			results[idx] = ParallelResult{
				Host:  s.Host,
				Error: fn(s),
			}
		}(i, srv)
	}

	wg.Wait()
	return results
}

// CheckParallelResults evaluates results and returns an error if any server failed
func CheckParallelResults(results []ParallelResult) error {
	var failed []string
	for _, r := range results {
		if r.Error != nil {
			failed = append(failed, fmt.Sprintf("[%s] %v", r.Host, r.Error))
			logger.Error("[%s] Failed: %v", r.Host, r.Error)
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("%d server(s) failed:\n  %s", len(failed), joinLines(failed))
	}

	return nil
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n  "
		}
		result += line
	}
	return result
}
