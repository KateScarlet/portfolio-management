package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsSetupMode_NoConfig(t *testing.T) {
	dir := t.TempDir()
	origConfigFile := configFile
	configFile = filepath.Join(dir, "config.yaml")
	defer func() { configFile = origConfigFile }()

	if !IsSetupMode() {
		t.Error("expected setup mode when config file doesn't exist")
	}
}

func TestIsSetupMode_WithConfig(t *testing.T) {
	dir := t.TempDir()
	origConfigFile := configFile
	configFile = filepath.Join(dir, "config.yaml")
	defer func() { configFile = origConfigFile }()

	if err := os.WriteFile(configFile, []byte("jwtSecret: test"), 0o640); err != nil {
		t.Fatal(err)
	}

	if IsSetupMode() {
		t.Error("expected non-setup mode when config file exists")
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	dir := t.TempDir()
	origConfigFile := configFile
	origConfigDir := configDir
	configDir = dir
	configFile = filepath.Join(dir, "config.yaml")
	defer func() {
		configFile = origConfigFile
		configDir = origConfigDir
	}()

	cfg := &Config{
		JWTSecret: "test-secret-123",
	}
	cfg.Database.Type = "sqlite"
	cfg.Database.DSN = filepath.Join(dir, "test.db")

	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	data, err := os.ReadFile(configFile)
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
	if !contains(content, "sqlite") {
		t.Error("config file missing database type")
	}
}

func TestSaveConfig_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	origConfigDir := configDir
	origConfigFile := configFile
	configDir = filepath.Join(dir, "subdir")
	configFile = filepath.Join(configDir, "config.yaml")
	defer func() {
		configDir = origConfigDir
		configFile = origConfigFile
	}()

	cfg := &Config{JWTSecret: "secret"}
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	info, err := os.Stat(configDir)
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
	if len(s1) != 64 {
		t.Errorf("expected 64 char hex string, got %d chars", len(s1))
	}
}

func TestInit_SQLite(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{}
	cfg.Database.Type = "sqlite"
	cfg.Database.DSN = filepath.Join(dir, "test.db")

	db, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
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

func TestInit_NilConfig(t *testing.T) {
	dir := t.TempDir()
	origDataDir := dataDir
	dataDir = dir
	defer func() { dataDir = origDataDir }()

	db, err := Init(nil)
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

func TestDefaultDSN(t *testing.T) {
	origDataDir := dataDir
	dataDir = "/tmp/test-data"
	defer func() { dataDir = origDataDir }()

	dsn := DefaultDSN()
	if dsn != "/tmp/test-data/portfolio.db" {
		t.Errorf("unexpected DSN: %s", dsn)
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
