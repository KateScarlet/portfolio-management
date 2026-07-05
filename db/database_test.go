package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsSetupMode_NoConfig(t *testing.T) {
	dir := t.TempDir()
	SetBaseDir(dir)
	defer SetBaseDir("")

	if !IsSetupMode() {
		t.Error("expected setup mode when config file doesn't exist")
	}
}

func TestIsSetupMode_WithConfig(t *testing.T) {
	dir := t.TempDir()
	SetBaseDir(dir)
	defer SetBaseDir("")

	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config", "config.yaml"), []byte("jwtSecret: test"), 0o640); err != nil {
		t.Fatal(err)
	}

	if IsSetupMode() {
		t.Error("expected non-setup mode when config file exists")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	dir := t.TempDir()
	SetBaseDir(dir)
	defer SetBaseDir("")

	cfg := &Config{
		JWTSecret: "test-secret-123",
	}
	cfg.Database.Type = "postgres"
	cfg.Database.DSN = "postgres://localhost:5432/test?sslmode=disable"

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	data, err := os.ReadFile(ConfigFile())
	if err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	if len(data) == 0 {
		t.Error("config file is empty")
	}

	content := string(data)
	if !contains(content, "test-secret-123") {
		t.Error("config file missing jwtSecret")
	}
	if !contains(content, "postgres") {
		t.Error("config file missing database type")
	}
}

func TestSaveConfig_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	SetBaseDir(dir)
	defer SetBaseDir("")

	cfg := &Config{JWTSecret: "secret"}
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "config"))
	if err != nil {
		t.Fatal("config directory not created")
	}
	if !info.IsDir() {
		t.Error("expected config to be a directory")
	}
}

func TestGenerateJWTSecret_Unique(t *testing.T) {
	s1 := generateJWTSecret()
	s2 := generateJWTSecret()

	if s1 == s2 {
		t.Error("expected unique secrets")
	}
	if len(s1) != 64 { // 32 bytes hex-encoded
		t.Errorf("expected 64 char hex string, got %d chars", len(s1))
	}
}

func TestInit_UnsupportedDB(t *testing.T) {
	cfg := &Config{}
	cfg.Database.Type = "sqlite"
	cfg.Database.DSN = "test.db"

	_, err := Init(cfg)
	if err == nil {
		t.Fatal("expected error for unsupported database type")
	}
}

func TestInit_NilConfig(t *testing.T) {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		dsn = "postgres://localhost:5432/portfolio_test?sslmode=disable"
	}

	cfg := &Config{}
	cfg.Database.Type = "postgres"
	cfg.Database.DSN = dsn

	db, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init with nil config failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close() //nolint:errcheck // test cleanup

	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("database not reachable: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s != "" && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
