package deploy

import (
	"fmt"
	"testing"

	"github.com/neurosamAI/tow-cli/internal/config"
)

func TestRunParallelSingleServer(t *testing.T) {
	cfg := testConfig()
	d := New(cfg, nil)

	servers := []config.Server{
		{Number: 1, Host: "10.0.1.10"},
	}

	callCount := 0
	results := d.RunParallel(servers, nil, func(srv config.Server) error {
		callCount++
		return nil
	})

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
	if results[0].Error != nil {
		t.Errorf("expected no error, got %v", results[0].Error)
	}
}

func TestRunParallelMultipleServers(t *testing.T) {
	cfg := testConfig()
	d := New(cfg, nil)

	servers := []config.Server{
		{Number: 1, Host: "10.0.1.10"},
		{Number: 2, Host: "10.0.1.11"},
		{Number: 3, Host: "10.0.1.12"},
	}

	results := d.RunParallel(servers, nil, func(srv config.Server) error {
		return nil
	})

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	err := CheckParallelResults(results)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestRunParallelWithFailure(t *testing.T) {
	cfg := testConfig()
	d := New(cfg, nil)

	servers := []config.Server{
		{Number: 1, Host: "10.0.1.10"},
		{Number: 2, Host: "10.0.1.11"},
	}

	results := d.RunParallel(servers, nil, func(srv config.Server) error {
		if srv.Number == 2 {
			return fmt.Errorf("server 2 failed")
		}
		return nil
	})

	err := CheckParallelResults(results)
	if err == nil {
		t.Error("expected error for failed server")
	}

	// Server 1 should succeed
	if results[0].Error != nil {
		t.Errorf("server 1 should succeed, got %v", results[0].Error)
	}

	// Server 2 should fail
	if results[1].Error == nil {
		t.Error("server 2 should fail")
	}
}

func TestCheckParallelResultsAllSuccess(t *testing.T) {
	results := []ParallelResult{
		{Host: "host1", Error: nil},
		{Host: "host2", Error: nil},
	}

	if err := CheckParallelResults(results); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestCheckParallelResultsPartialFailure(t *testing.T) {
	results := []ParallelResult{
		{Host: "host1", Error: nil},
		{Host: "host2", Error: fmt.Errorf("connection refused")},
		{Host: "host3", Error: fmt.Errorf("timeout")},
	}

	err := CheckParallelResults(results)
	if err == nil {
		t.Error("expected error")
	}
	if !contains(err.Error(), "2 server(s) failed") {
		t.Errorf("expected '2 server(s) failed', got: %s", err.Error())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
