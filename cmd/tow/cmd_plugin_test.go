package main

import (
	"strings"
	"testing"
)

func TestResolvePluginSourceFullURL(t *testing.T) {
	url, fileName := resolvePluginSource("https://example.com/plugins/kafka.yaml")
	if url != "https://example.com/plugins/kafka.yaml" {
		t.Errorf("expected full URL unchanged, got %q", url)
	}
	if fileName != "kafka.yaml" {
		t.Errorf("expected 'kafka.yaml', got %q", fileName)
	}
}

func TestResolvePluginSourceFullURLNoExtension(t *testing.T) {
	url, fileName := resolvePluginSource("https://example.com/plugins/kafka")
	if url != "https://example.com/plugins/kafka" {
		t.Errorf("expected full URL unchanged, got %q", url)
	}
	if fileName != "kafka.yaml" {
		t.Errorf("expected 'kafka.yaml' with extension added, got %q", fileName)
	}
}

func TestResolvePluginSourceUserRepo(t *testing.T) {
	url, fileName := resolvePluginSource("user/tow-plugin-kafka")
	if !strings.Contains(url, "githubusercontent.com") {
		t.Errorf("expected github URL, got %q", url)
	}
	if fileName != "kafka.yaml" {
		t.Errorf("expected 'kafka.yaml' (tow-plugin- stripped), got %q", fileName)
	}
}

func TestResolvePluginSourceUserRepoPath(t *testing.T) {
	url, fileName := resolvePluginSource("user/repo/plugins/redis")
	if !strings.Contains(url, "githubusercontent.com") {
		t.Errorf("expected github URL, got %q", url)
	}
	if !strings.Contains(url, "plugins/redis.yaml") {
		t.Errorf("expected path in URL, got %q", url)
	}
	if fileName != "redis.yaml" {
		t.Errorf("expected 'redis.yaml', got %q", fileName)
	}
}

func TestResolvePluginSourceBareName(t *testing.T) {
	url, fileName := resolvePluginSource("myservice")
	if url != "myservice" {
		t.Errorf("expected bare name as URL, got %q", url)
	}
	if fileName != "myservice.yaml" {
		t.Errorf("expected 'myservice.yaml', got %q", fileName)
	}
}

func TestMaxHelper(t *testing.T) {
	if max(3, 5) != 5 {
		t.Errorf("expected 5")
	}
	if max(10, 2) != 10 {
		t.Errorf("expected 10")
	}
	if max(4, 4) != 4 {
		t.Errorf("expected 4")
	}
}
