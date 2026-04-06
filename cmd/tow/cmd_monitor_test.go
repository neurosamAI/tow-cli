package main

import (
	"strings"
	"testing"
)

func TestPresetsPath(t *testing.T) {
	path := presetsPath()
	if path == "" {
		t.Error("expected non-empty presets path")
	}
	if !strings.Contains(path, ".tow") {
		t.Errorf("expected '.tow' in path, got %q", path)
	}
	if !strings.Contains(path, "presets.yaml") {
		t.Errorf("expected 'presets.yaml' in path, got %q", path)
	}
}

func TestLoadPresetsEmpty(t *testing.T) {
	// When no presets file exists, should return empty map without error
	pf, err := loadPresets()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pf.Presets == nil {
		t.Error("expected non-nil Presets map")
	}
}

func TestShowPresetsNoPanic(t *testing.T) {
	// Should not panic even if presets file doesn't exist
	err := showPresets()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
