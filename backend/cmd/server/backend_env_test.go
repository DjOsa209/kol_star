package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackendEnvLoadsKeyAndPreservesProcessEnvironment(t *testing.T) {
	file := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(file, []byte("IP_WEB_SEARCH_PROVIDER=tavily\nTAVILY_API_KEY=\"test-only-file-key\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENV_FILE", file)
	t.Setenv("IP_WEB_SEARCH_PROVIDER", "")
	t.Setenv("TAVILY_API_KEY", "")
	_ = os.Unsetenv("IP_WEB_SEARCH_PROVIDER")
	_ = os.Unsetenv("TAVILY_API_KEY")
	if err := loadBackendEnvFile(); err != nil || os.Getenv("TAVILY_API_KEY") != "test-only-file-key" {
		t.Fatal("env file was not loaded")
	}
	t.Setenv("TAVILY_API_KEY", "test-only-deployment-key")
	if err := loadBackendEnvFile(); err != nil || os.Getenv("TAVILY_API_KEY") != "test-only-deployment-key" {
		t.Fatal("process key was overwritten")
	}
}

func TestBackendEnvMissingOptionalAndRedactedErrors(t *testing.T) {
	t.Setenv("ENV_FILE", "")
	if err := loadBackendEnvFileFrom(filepath.Join(t.TempDir(), ".env"), false); err != nil {
		t.Fatal("missing optional .env blocked startup")
	}
	t.Setenv("ENV_FILE", filepath.Join(t.TempDir(), "missing.env"))
	if err := loadBackendEnvFile(); err == nil {
		t.Fatal("explicit missing env path ignored")
	}
	file := filepath.Join(t.TempDir(), "invalid.env")
	if err := os.WriteFile(file, []byte("TAVILY_API_KEY='SECRET_PARSER_LINE\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ENV_FILE", file)
	err := loadBackendEnvFile()
	if err == nil || strings.Contains(err.Error(), "SECRET_PARSER_LINE") {
		t.Fatal("invalid env accepted or secret leaked")
	}
}
