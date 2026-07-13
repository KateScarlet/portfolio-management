package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"portfolio-management/db"
	"testing"

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
