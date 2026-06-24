package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route/param"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func setupAuthTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := setupTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.PortfolioRecord{}, &models.WebAuthnCredential{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func createTestUser(t *testing.T, db *gorm.DB, username, role string) string {
	t.Helper()
	err := CreateUserForSetup(db, username, "password123", role)
	if err != nil {
		t.Fatal(err)
	}
	var user models.User
	db.Where("username = ?", username).First(&user)
	return user.ID
}

func jsonBody(body any) []byte {
	b, _ := json.Marshal(body)
	return b
}

func authReq(method, path string, body any) *app.RequestContext {
	b := jsonBody(body)
	c := app.NewContext(1)
	c.Request.SetRequestURI(path)
	c.Request.Header.SetMethod(method)
	c.Request.Header.SetContentTypeBytes([]byte("application/json"))
	c.Request.SetBodyStream(bytes.NewReader(b), len(b))
	return c
}

// --- Register ---

func TestRegister_Success(t *testing.T) {
	db := setupAuthTestDB(t)

	c := authReq("POST", "/api/users", map[string]string{
		"username": "newuser",
		"password": "password123",
		"role":     "user",
	})

	Register(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d: %s", c.Response.StatusCode(), string(c.Response.Body()))
	}

	var resp map[string]any
	if err := json.Unmarshal(c.Response.Body(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["username"] != "newuser" {
		t.Errorf("expected username 'newuser', got %v", resp["username"])
	}
	if resp["role"] != "user" {
		t.Errorf("expected role 'user', got %v", resp["role"])
	}
}

func TestRegister_EmptyUsername(t *testing.T) {
	db := setupAuthTestDB(t)

	c := authReq("POST", "/api/users", map[string]string{
		"username": "",
		"password": "password123",
	})

	Register(db)(context.Background(), c)

	if c.Response.StatusCode() != 400 {
		t.Errorf("expected 400, got %d", c.Response.StatusCode())
	}
}

func TestRegister_ShortPassword(t *testing.T) {
	db := setupAuthTestDB(t)

	c := authReq("POST", "/api/users", map[string]string{
		"username": "newuser",
		"password": "12345",
	})

	Register(db)(context.Background(), c)

	if c.Response.StatusCode() != 400 {
		t.Errorf("expected 400, got %d", c.Response.StatusCode())
	}
}

func TestRegister_DuplicateUsername(t *testing.T) {
	db := setupAuthTestDB(t)
	createTestUser(t, db, "existing", "user")

	c := authReq("POST", "/api/users", map[string]string{
		"username": "existing",
		"password": "password123",
	})

	Register(db)(context.Background(), c)

	if c.Response.StatusCode() != 409 {
		t.Errorf("expected 409, got %d: %s", c.Response.StatusCode(), string(c.Response.Body()))
	}
}

// --- Login ---

func TestLogin_Success(t *testing.T) {
	db := setupAuthTestDB(t)
	createTestUser(t, db, "loginuser", "user")

	c := authReq("POST", "/api/auth/login", map[string]string{
		"username": "loginuser",
		"password": "password123",
	})

	Login(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d: %s", c.Response.StatusCode(), string(c.Response.Body()))
	}

	var resp map[string]any
	if err := json.Unmarshal(c.Response.Body(), &resp); err != nil {
		t.Fatal(err)
	}
	user, ok := resp["user"].(map[string]any)
	if !ok {
		t.Fatal("expected user in response")
	}
	if user["username"] != "loginuser" {
		t.Errorf("expected username 'loginuser', got %v", user["username"])
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	db := setupAuthTestDB(t)
	createTestUser(t, db, "loginuser2", "user")

	c := authReq("POST", "/api/auth/login", map[string]string{
		"username": "loginuser2",
		"password": "wrongpassword",
	})

	Login(db)(context.Background(), c)

	if c.Response.StatusCode() != 401 {
		t.Errorf("expected 401, got %d", c.Response.StatusCode())
	}
}

func TestLogin_NonexistentUser(t *testing.T) {
	db := setupAuthTestDB(t)

	c := authReq("POST", "/api/auth/login", map[string]string{
		"username": "nobody",
		"password": "password123",
	})

	Login(db)(context.Background(), c)

	if c.Response.StatusCode() != 401 {
		t.Errorf("expected 401, got %d", c.Response.StatusCode())
	}
}

func TestLogin_EmptyCredentials(t *testing.T) {
	db := setupAuthTestDB(t)

	c := authReq("POST", "/api/auth/login", map[string]string{
		"username": "",
		"password": "",
	})

	Login(db)(context.Background(), c)

	if c.Response.StatusCode() != 400 {
		t.Errorf("expected 400, got %d", c.Response.StatusCode())
	}
}

// --- Me ---

func TestMe_Authenticated(t *testing.T) {
	db := setupAuthTestDB(t)
	uid := createTestUser(t, db, "meuser", "user")

	c := app.NewContext(1)
	c.Request.SetRequestURI("/api/auth/me")
	c.Request.Header.SetMethod("GET")
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{
		UserID:   uid,
		Username: "meuser",
		Role:     "user",
	})

	Me(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", c.Response.StatusCode())
	}

	var resp map[string]any
	if err := json.Unmarshal(c.Response.Body(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["username"] != "meuser" {
		t.Errorf("expected 'meuser', got %v", resp["username"])
	}
}

func TestMe_Unauthenticated(t *testing.T) {
	db := setupAuthTestDB(t)

	c := app.NewContext(1)
	c.Request.SetRequestURI("/api/auth/me")

	Me(db)(context.Background(), c)

	if c.Response.StatusCode() != 401 {
		t.Errorf("expected 401, got %d", c.Response.StatusCode())
	}
}

// --- ListUsers ---

func TestListUsers_ReturnsAll(t *testing.T) {
	db := setupAuthTestDB(t)
	createTestUser(t, db, "user1", "user")
	createTestUser(t, db, "user2", "admin")

	c := app.NewContext(1)
	c.Request.SetRequestURI("/api/users")

	ListUsers(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d", c.Response.StatusCode())
	}

	var users []map[string]any
	json.Unmarshal(c.Response.Body(), &users)
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

// --- DeleteUser ---

func TestDeleteUser_CleansUpRelatedData(t *testing.T) {
	db := setupAuthTestDB(t)
	uid := createTestUser(t, db, "deleteme", "user")

	portfolioID := uuid.New().String()
	db.Create(&models.Portfolio{ID: portfolioID, UserID: uid, Name: "Test", IsDefault: true, CreatedAt: 1000})
	db.Create(&models.Holding{ID: uuid.New().String(), UserID: uid, PortfolioID: portfolioID, AssetId: "stocks", Symbol: "AAPL"})
	db.Create(&models.PortfolioRecord{ID: uuid.New().String(), UserID: uid, PortfolioID: portfolioID, Timestamp: 1000, Assets: models.AssetMapColumn{"stocks": 100}, Total: 100})
	db.Create(&models.Setting{Key: "syncInterval", Value: "5", UserID: uid, PortfolioID: portfolioID})
	db.Create(&models.WebAuthnCredential{ID: uuid.New().String(), UserID: uid, CredentialID: []byte("test"), PublicKey: []byte("test")})

	adminID := createTestUser(t, db, "admin", "admin")

	c := app.NewContext(1)
	c.Params = param.Params{{Key: "id", Value: uid}}
	c.Request.SetRequestURI("/api/users/" + uid)
	c.Request.Header.SetMethod("DELETE")
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{
		UserID:   adminID,
		Username: "admin",
		Role:     "admin",
	})

	DeleteUser(db)(context.Background(), c)

	if c.Response.StatusCode() != 200 {
		t.Fatalf("expected 200, got %d: %s", c.Response.StatusCode(), string(c.Response.Body()))
	}

	var userCount int64
	db.Model(&models.User{}).Where("id = ?", uid).Count(&userCount)
	if userCount != 0 {
		t.Error("expected user to be deleted")
	}

	var holdingCount int64
	db.Model(&models.Holding{}).Where("user_id = ?", uid).Count(&holdingCount)
	if holdingCount != 0 {
		t.Error("expected holdings to be deleted")
	}

	var recordCount int64
	db.Model(&models.PortfolioRecord{}).Where("user_id = ?", uid).Count(&recordCount)
	if recordCount != 0 {
		t.Error("expected records to be deleted")
	}

	var settingCount int64
	db.Model(&models.Setting{}).Where("user_id = ?", uid).Count(&settingCount)
	if settingCount != 0 {
		t.Error("expected settings to be deleted")
	}

	var credCount int64
	db.Model(&models.WebAuthnCredential{}).Where("user_id = ?", uid).Count(&credCount)
	if credCount != 0 {
		t.Error("expected webauthn credentials to be deleted")
	}
}

func TestDeleteUser_CannotDeleteSelf(t *testing.T) {
	db := setupAuthTestDB(t)
	uid := createTestUser(t, db, "selfdelete", "admin")

	c := app.NewContext(1)
	c.Params = param.Params{{Key: "id", Value: uid}}
	c.Request.SetRequestURI("/api/users/" + uid)
	c.Request.Header.SetMethod("DELETE")
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{
		UserID:   uid,
		Username: "selfdelete",
		Role:     "admin",
	})

	DeleteUser(db)(context.Background(), c)

	if c.Response.StatusCode() != 400 {
		t.Errorf("expected 400, got %d", c.Response.StatusCode())
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	db := setupAuthTestDB(t)
	adminID := createTestUser(t, db, "admin", "admin")

	c := app.NewContext(1)
	c.Params = param.Params{{Key: "id", Value: "nonexistent"}}
	c.Request.SetRequestURI("/api/users/nonexistent")
	c.Request.Header.SetMethod("DELETE")
	c.Set(string(middleware.UserContextKey), &middleware.JWTClaims{
		UserID:   adminID,
		Username: "admin",
		Role:     "admin",
	})

	DeleteUser(db)(context.Background(), c)

	if c.Response.StatusCode() != 404 {
		t.Errorf("expected 404, got %d", c.Response.StatusCode())
	}
}
