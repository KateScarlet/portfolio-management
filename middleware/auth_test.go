package middleware

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/golang-jwt/jwt/v5"
)

func init() {
	SetJWTSecret("test-secret-key-for-unit-tests")
}

func makeToken(userID, username, role string, expired bool) string {
	claims := &JWTClaims{
		UserID:   userID,
		Username: username,
		Role:     role,
	}
	if expired {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-1 * time.Hour))
	} else {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(1 * time.Hour))
	}
	claims.IssuedAt = jwt.NewNumericDate(time.Now())
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := token.SignedString(JWTSecret)
	return s
}

func newRequestContext() *app.RequestContext {
	c := app.NewContext(1)
	c.Request.SetRequestURI("/api/test")
	return c
}

// --- AuthRequired tests ---

func TestAuthRequired_ValidToken(t *testing.T) {
	c := newRequestContext()
	token := makeToken("u1", "alice", "user", false)
	c.Request.Header.Set("Authorization", "Bearer "+token)

	handler := AuthRequired()
	handler(context.Background(), c)

	if c.Response.StatusCode() != http.StatusOK {
		t.Errorf("expected 200, got %d", c.Response.StatusCode())
	}
	claims, ok := c.Get(string(UserContextKey))
	if !ok || claims == nil {
		t.Fatal("expected claims to be set in context")
	}
	jwtClaims, ok := claims.(*JWTClaims)
	if !ok {
		t.Fatal("expected *JWTClaims type")
	}
	if jwtClaims.UserID != "u1" {
		t.Errorf("expected UserID=u1, got %q", jwtClaims.UserID)
	}
	if jwtClaims.Username != "alice" {
		t.Errorf("expected Username=alice, got %q", jwtClaims.Username)
	}
	if jwtClaims.Role != "user" {
		t.Errorf("expected Role=user, got %q", jwtClaims.Role)
	}
}

func TestAuthRequired_NoToken(t *testing.T) {
	c := newRequestContext()

	handler := AuthRequired()
	handler(context.Background(), c)

	if c.Response.StatusCode() != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", c.Response.StatusCode())
	}
}

func TestAuthRequired_InvalidToken(t *testing.T) {
	c := newRequestContext()
	c.Request.Header.Set("Authorization", "Bearer invalid.jwt.token")

	handler := AuthRequired()
	handler(context.Background(), c)

	if c.Response.StatusCode() != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", c.Response.StatusCode())
	}
}

func TestAuthRequired_ExpiredToken(t *testing.T) {
	c := newRequestContext()
	token := makeToken("u1", "alice", "user", true)
	c.Request.Header.Set("Authorization", "Bearer "+token)

	handler := AuthRequired()
	handler(context.Background(), c)

	if c.Response.StatusCode() != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", c.Response.StatusCode())
	}
}

func TestAuthRequired_WrongSecret(t *testing.T) {
	c := newRequestContext()
	// Generate token with a different secret
	claims := &JWTClaims{
		UserID:   "u1",
		Username: "alice",
		Role:     "user",
	}
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(1 * time.Hour))
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := token.SignedString([]byte("wrong-secret"))
	c.Request.Header.Set("Authorization", "Bearer "+s)

	handler := AuthRequired()
	handler(context.Background(), c)

	if c.Response.StatusCode() != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", c.Response.StatusCode())
	}
}

func TestAuthRequired_FromCookie(t *testing.T) {
	c := newRequestContext()
	token := makeToken("u1", "alice", "user", false)
	c.Request.Header.Set("Cookie", "auth_token="+token)

	handler := AuthRequired()
	handler(context.Background(), c)

	if c.Response.StatusCode() != http.StatusOK {
		t.Errorf("expected 200, got %d", c.Response.StatusCode())
	}
}

func TestAuthRequired_BearerPrefixRequired(t *testing.T) {
	c := newRequestContext()
	token := makeToken("u1", "alice", "user", false)
	// Set without "Bearer " prefix
	c.Request.Header.Set("Authorization", token)

	handler := AuthRequired()
	handler(context.Background(), c)

	// Should fall through to cookie check, which also has no token
	if c.Response.StatusCode() != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", c.Response.StatusCode())
	}
}

// --- AdminRequired tests ---

func TestAdminRequired_AdminRole(t *testing.T) {
	c := newRequestContext()
	c.Set(string(UserContextKey), &JWTClaims{
		UserID: "u1", Username: "admin", Role: "admin",
	})

	handler := AdminRequired()
	handler(context.Background(), c)

	if c.Response.StatusCode() != http.StatusOK {
		t.Errorf("expected 200, got %d", c.Response.StatusCode())
	}
}

func TestAdminRequired_NonAdminRole(t *testing.T) {
	c := newRequestContext()
	c.Set(string(UserContextKey), &JWTClaims{
		UserID: "u1", Username: "alice", Role: "user",
	})

	handler := AdminRequired()
	handler(context.Background(), c)

	if c.Response.StatusCode() != http.StatusForbidden {
		t.Errorf("expected 403, got %d", c.Response.StatusCode())
	}
}

func TestAdminRequired_NoClaimsInContext(t *testing.T) {
	c := newRequestContext()

	handler := AdminRequired()
	handler(context.Background(), c)

	if c.Response.StatusCode() != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", c.Response.StatusCode())
	}
}

func TestAdminRequired_WrongType(t *testing.T) {
	c := newRequestContext()
	c.Set(string(UserContextKey), "not-a-jwt-claims")

	handler := AdminRequired()
	handler(context.Background(), c)

	if c.Response.StatusCode() != http.StatusForbidden {
		t.Errorf("expected 403, got %d", c.Response.StatusCode())
	}
}

// --- GetUser tests ---

func TestGetUser_Exists(t *testing.T) {
	c := newRequestContext()
	expected := &JWTClaims{UserID: "u1", Username: "alice", Role: "user"}
	c.Set(string(UserContextKey), expected)

	result := GetUser(c)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.UserID != expected.UserID {
		t.Errorf("expected UserID=%q, got %q", expected.UserID, result.UserID)
	}
}

func TestGetUser_NotExists(t *testing.T) {
	c := newRequestContext()

	result := GetUser(c)
	if result != nil {
		t.Errorf("expected nil, got %+v", result)
	}
}

func TestGetUser_WrongType(t *testing.T) {
	c := newRequestContext()
	c.Set(string(UserContextKey), "not-a-jwt-claims")

	result := GetUser(c)
	if result != nil {
		t.Errorf("expected nil, got %+v", result)
	}
}

// --- extractToken tests ---

func TestExtractToken_FromBearerHeader(t *testing.T) {
	c := newRequestContext()
	c.Request.Header.Set("Authorization", "Bearer my-token-123")

	token := extractToken(c)
	if token != "my-token-123" {
		t.Errorf("expected 'my-token-123', got %q", token)
	}
}

func TestExtractToken_FromCookie(t *testing.T) {
	c := newRequestContext()
	c.Request.Header.Set("Cookie", "auth_token=cookie-token-456")

	token := extractToken(c)
	if token != "cookie-token-456" {
		t.Errorf("expected 'cookie-token-456', got %q", token)
	}
}

func TestExtractToken_NoAuth(t *testing.T) {
	c := newRequestContext()

	token := extractToken(c)
	if token != "" {
		t.Errorf("expected empty string, got %q", token)
	}
}

func TestExtractToken_HeaderTakesPrecedence(t *testing.T) {
	c := newRequestContext()
	c.Request.Header.Set("Authorization", "Bearer header-token")
	c.Request.Header.Set("Cookie", "auth_token=cookie-token")

	token := extractToken(c)
	if token != "header-token" {
		t.Errorf("expected 'header-token', got %q", token)
	}
}

func TestExtractToken_BearerEmpty(t *testing.T) {
	c := newRequestContext()
	c.Request.Header.Set("Authorization", "Bearer ")

	token := extractToken(c)
	if token != "" {
		t.Errorf("expected empty string after Bearer, got %q", token)
	}
}

// --- Full flow: AuthRequired -> AdminRequired ---

func TestFullFlow_AuthThenAdmin(t *testing.T) {
	c := newRequestContext()
	token := makeToken("u1", "admin", "admin", false)
	c.Request.Header.Set("Authorization", "Bearer "+token)

	// Step 1: AuthRequired
	authHandler := AuthRequired()
	authHandler(context.Background(), c)
	if c.Response.StatusCode() != http.StatusOK {
		t.Fatalf("AuthRequired failed: %d", c.Response.StatusCode())
	}

	// Step 2: AdminRequired
	adminHandler := AdminRequired()
	adminHandler(context.Background(), c)
	if c.Response.StatusCode() != http.StatusOK {
		t.Errorf("AdminRequired failed: %d", c.Response.StatusCode())
	}
}

func TestFullFlow_AuthThenAdmin_NonAdmin(t *testing.T) {
	c := newRequestContext()
	token := makeToken("u1", "alice", "user", false)
	c.Request.Header.Set("Authorization", "Bearer "+token)

	// Step 1: AuthRequired passes
	authHandler := AuthRequired()
	authHandler(context.Background(), c)
	if c.Response.StatusCode() != http.StatusOK {
		t.Fatalf("AuthRequired failed: %d", c.Response.StatusCode())
	}

	// Step 2: AdminRequired rejects
	adminHandler := AdminRequired()
	adminHandler(context.Background(), c)
	if c.Response.StatusCode() != http.StatusForbidden {
		t.Errorf("expected 403, got %d", c.Response.StatusCode())
	}
}

func TestFullFlow_AuthFailsThenAdminNeverRuns(t *testing.T) {
	c := newRequestContext()

	// Step 1: AuthRequired rejects
	authHandler := AuthRequired()
	authHandler(context.Background(), c)
	if c.Response.StatusCode() != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", c.Response.StatusCode())
	}

	// Step 2: AdminRequired should not be reached, but if called:
	adminHandler := AdminRequired()
	adminHandler(context.Background(), c)
	if c.Response.StatusCode() != http.StatusUnauthorized {
		t.Errorf("expected 401 from AdminRequired (no claims), got %d", c.Response.StatusCode())
	}
}

// --- Edge cases ---

func TestAuthRequired_MalformedBearerFormat(t *testing.T) {
	tests := []struct {
		name  string
		auth  string
		code  int
	}{
		{"lowercase bearer", "bearer token123", http.StatusUnauthorized},
		{"no space", "Bearertoken123", http.StatusUnauthorized},
		{"extra space", "Bearer  token123", http.StatusUnauthorized},
		{"basic auth", "Basic dXNlcjpwYXNz", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newRequestContext()
			c.Request.Header.Set("Authorization", tt.auth)
			AuthRequired()(context.Background(), c)
			if c.Response.StatusCode() != tt.code {
				t.Errorf("expected %d, got %d", tt.code, c.Response.StatusCode())
			}
		})
	}
}

func TestJWTClaims_Fields(t *testing.T) {
	token := makeToken("u123", "bob", "admin", false)
	claims := &JWTClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		return JWTSecret, nil
	})
	if err != nil {
		t.Fatalf("failed to parse token: %v", err)
	}
	c := parsed.Claims.(*JWTClaims)
	if c.UserID != "u123" {
		t.Errorf("expected UserID=u123, got %q", c.UserID)
	}
	if c.Username != "bob" {
		t.Errorf("expected Username=bob, got %q", c.Username)
	}
	if c.Role != "admin" {
		t.Errorf("expected Role=admin, got %q", c.Role)
	}
	if c.ExpiresAt == nil || c.ExpiresAt.Before(time.Now()) {
		t.Error("expected valid ExpiresAt")
	}
	fmt.Println("all fields validated")
}
