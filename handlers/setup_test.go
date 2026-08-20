package handlers

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"testing"

	"portfolio-management/db"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestSetupComplete_InitFailureKeepsSetupMode(t *testing.T) {
	dir := t.TempDir()
	db.SetBaseDir(dir)
	t.Cleanup(func() { db.SetBaseDir("") })

	c := app.NewContext(1)
	c.Request.SetRequestURI("/api/setup/complete")
	c.Request.Header.SetMethod("POST")
	c.Request.Header.SetContentTypeBytes([]byte("application/json"))
	body, err := json.Marshal(map[string]string{
		"databaseType": "unsupported",
		"databaseDsn":  "unused",
		"username":     "admin",
		"password":     "password123",
	})
	if err != nil {
		t.Fatal(err)
	}
	c.Request.SetBodyStream(bytes.NewReader(body), len(body))

	SetupComplete(nil)(context.Background(), c)

	if c.Response.StatusCode() != consts.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", c.Response.StatusCode(), c.Response.Body())
	}
	if !db.IsSetupMode() {
		t.Fatal("initialization failure must leave the application in setup mode")
	}
}

func TestBuildPostgresDSN(t *testing.T) {
	dsn, err := buildPostgresDSN("db.internal", "5433", "my portfolio", "app@example.com", "p@ss:/word", "require")
	if err != nil {
		t.Fatal(err)
	}
	want := "postgres://app%40example.com:p%40ss%3A%2Fword@db.internal:5433/my%20portfolio?sslmode=require"
	if dsn != want {
		t.Fatalf("expected %q, got %q", want, dsn)
	}
}

func TestBuildPostgresDSNDefaults(t *testing.T) {
	dsn, err := buildPostgresDSN("", "", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	want := "postgres://localhost:5432/portfolio?sslmode=disable"
	if dsn != want {
		t.Fatalf("expected %q, got %q", want, dsn)
	}
}

func TestBuildPostgresDSNRejectsInvalidPort(t *testing.T) {
	if _, err := buildPostgresDSN("localhost", "70000", "portfolio", "", "", "disable"); err == nil {
		t.Fatal("expected an invalid port error")
	}
}
