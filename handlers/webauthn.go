package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"math"
	"portfolio-management/db"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type webauthnRegisterSession struct {
	SessionData *webauthn.SessionData
	CredName    string
}

func saveSession(database *gorm.DB, id string, data any) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		slog.Error("failed to marshal webauthn session", "error", err)
		return
	}
	slog.Info("saving webauthn session", "id", id, "data_len", len(jsonData))
	session := models.WebAuthnSession{
		ID:        id,
		Data:      string(jsonData),
		ExpiresAt: time.Now().Add(5 * time.Minute).Unix(),
	}
	result := database.Where("id = ?", id).Assign(session).FirstOrCreate(&session)
	slog.Info("session saved", "id", id, "rows_affected", result.RowsAffected, "error", result.Error)
}

func loadSession(database *gorm.DB, id string) any {
	var session models.WebAuthnSession
	if err := database.Where("id = ?", id).First(&session).Error; err != nil {
		slog.Warn("session not found in db", "id", id, "error", err)
		return nil
	}
	slog.Info("session found in db", "id", id, "expires_at", session.ExpiresAt, "now", time.Now().Unix(), "data_len", len(session.Data))
	if time.Now().Unix() > session.ExpiresAt {
		slog.Warn("session expired", "id", id)
		database.Delete(&session)
		return nil
	}

	var regSession webauthnRegisterSession
	if err := json.Unmarshal([]byte(session.Data), &regSession); err == nil && regSession.SessionData != nil {
		slog.Info("session loaded as register session", "id", id, "challenge", regSession.SessionData.Challenge)
		return &regSession
	}

	var sessionData webauthn.SessionData
	if err := json.Unmarshal([]byte(session.Data), &sessionData); err == nil && sessionData.Challenge != "" {
		slog.Info("session loaded as login session", "id", id, "challenge", sessionData.Challenge)
		return &sessionData
	}

	slog.Warn("session data unmarshal failed", "id", id)
	return nil
}

func deleteSession(database *gorm.DB, id string) {
	database.Where("id = ?", id).Delete(&models.WebAuthnSession{})
}

func generateSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate session ID: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

type webauthnUser struct {
	id          []byte
	name        string
	displayName string
	credentials []webauthn.Credential
}

func (u *webauthnUser) WebAuthnID() []byte                         { return u.id }
func (u *webauthnUser) WebAuthnName() string                       { return u.name }
func (u *webauthnUser) WebAuthnDisplayName() string                { return u.displayName }
func (u *webauthnUser) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

func loadUserCredentials(gormDB *gorm.DB, userID string) []webauthn.Credential {
	var creds []models.WebAuthnCredential
	gormDB.Where("user_id = ?", userID).Find(&creds)
	result := make([]webauthn.Credential, len(creds))
	for i := range creds {
		c := &creds[i]
		result[i] = webauthn.Credential{
			ID:        c.CredentialID,
			PublicKey: c.PublicKey,
			Flags:     webauthn.CredentialFlagsFromMsgpByte(c.Flags),
			Authenticator: webauthn.Authenticator{
				SignCount: uint32(min(c.SignCount, math.MaxUint32)), //nolint:gosec // G115: min ensures value fits in uint32
			},
		}
	}
	return result
}

func loadUserByID(gormDB *gorm.DB, id string) (*models.User, error) {
	var user models.User
	err := gormDB.Where("id = ?", id).First(&user).Error
	return &user, err
}

func loadWebAuthnUser(gormDB *gorm.DB, userHandle []byte) (*webauthnUser, error) {
	userID := string(userHandle)
	user, err := loadUserByID(gormDB, userID)
	if err != nil {
		return nil, err
	}
	creds := loadUserCredentials(gormDB, user.ID)
	return &webauthnUser{
		id:          []byte(user.ID),
		name:        user.Username,
		displayName: user.Username,
		credentials: creds,
	}, nil
}

func newWebAuthnInstance(cfg *db.Config) (*webauthn.WebAuthn, error) {
	slog.Info("creating webauthn instance", "rpid", cfg.WebAuthn.RPID, "rpOrigins", cfg.WebAuthn.RPOrigins)
	return webauthn.New(&webauthn.Config{
		RPDisplayName: "Portfolio Management",
		RPID:          cfg.WebAuthn.RPID,
		RPOrigins:     cfg.WebAuthn.RPOrigins,
	})
}

func WebAuthnStatus(cfg *db.Config) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.JSON(consts.StatusOK, map[string]bool{"enabled": cfg.WebAuthn.Enabled})
	}
}

func WebAuthnRegisterStart(gormDB *gorm.DB, cfg *db.Config) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		claims := middleware.GetUser(c)
		if claims == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		if !cfg.WebAuthn.Enabled || cfg.WebAuthn.RPID == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "WebAuthn未启用"})
			return
		}

		w, err := newWebAuthnInstance(cfg)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "初始化WebAuthn失败"})
			return
		}

		user, err := loadUserByID(gormDB, claims.UserID)
		if err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "用户不存在"})
			return
		}

		existingCreds := loadUserCredentials(gormDB, user.ID)
		webUser := &webauthnUser{
			id:          []byte(user.ID),
			name:        user.Username,
			displayName: user.Username,
			credentials: existingCreds,
		}

		body, _ := c.Body()
		var reqBody struct {
			Name string `json:"name"`
		}
		if body != nil {
			_ = json.Unmarshal(body, &reqBody)
		}

		sessionID := generateSessionID()
		credentialOptions, sessionData, err := w.BeginRegistration(webUser,
			webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementPreferred),
			webauthn.WithExclusions(webauthn.Credentials(existingCreds).CredentialDescriptors()),
		)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "开始注册失败"})
			return
		}

		saveSession(gormDB, sessionID, &webauthnRegisterSession{
			SessionData: sessionData,
			CredName:    reqBody.Name,
		})

		setAuthCookie(c, "webauthn_session", sessionID, 300, cfg)

		creation := credentialOptions.Response
		slog.Info("webauthn register start", "sessionID", sessionID)
		c.JSON(consts.StatusOK, map[string]any{
			"challenge":              base64.RawURLEncoding.EncodeToString(creation.Challenge),
			"rp":                     creation.RelyingParty,
			"user":                   creation.User,
			"pubKeyCredParams":       creation.Parameters,
			"timeout":                creation.Timeout,
			"attestation":            creation.Attestation,
			"excludeCredentials":     creation.CredentialExcludeList,
			"authenticatorSelection": creation.AuthenticatorSelection,
			"extensions":             creation.Extensions,
		})
	}
}

func WebAuthnRegisterFinish(gormDB *gorm.DB, cfg *db.Config) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		claims := middleware.GetUser(c)
		if claims == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		if !cfg.WebAuthn.Enabled || cfg.WebAuthn.RPID == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "WebAuthn未启用"})
			return
		}

		sessionID := string(c.Cookie("webauthn_session"))
		slog.Info("webauthn register finish", "sessionID", sessionID, "rpID", cfg.WebAuthn.RPID, "rpOrigins", cfg.WebAuthn.RPOrigins)
		if sessionID == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "会话无效"})
			return
		}
		defer deleteSession(gormDB, sessionID)

		sessionRaw := loadSession(gormDB, sessionID)
		if sessionRaw == nil {
			slog.Warn("webauthn register finish: session not found", "sessionID", sessionID)
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "会话已过期"})
			return
		}
		regSession, ok := sessionRaw.(*webauthnRegisterSession)
		if !ok {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "会话类型错误"})
			return
		}

		w, err := newWebAuthnInstance(cfg)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "初始化WebAuthn失败"})
			return
		}

		user, err := loadUserByID(gormDB, claims.UserID)
		if err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "用户不存在"})
			return
		}

		existingCreds := loadUserCredentials(gormDB, user.ID)
		webUser := &webauthnUser{
			id:          []byte(user.ID),
			name:        user.Username,
			displayName: user.Username,
			credentials: existingCreds,
		}

		body, err := c.Body()
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "读取请求体失败"})
			return
		}

		parsedCreation, err := protocol.ParseCredentialCreationResponseBytes(body)
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "解析注册响应失败: " + err.Error()})
			return
		}

		slog.Info("webauthn parsed creation", "raw_id_len", len(parsedCreation.RawID), "origin", parsedCreation.Response.CollectedClientData.Origin)

		credential, err := w.CreateCredential(webUser, *regSession.SessionData, parsedCreation)
		if err != nil {
			slog.Error("webauthn register validation failed", "error", err, "rpID", cfg.WebAuthn.RPID, "rpOrigins", cfg.WebAuthn.RPOrigins)
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "注册验证失败: " + err.Error()})
			return
		}

		credName := regSession.CredName
		if credName == "" {
			credName = time.Now().Format("2006-01-02 15:04")
		}

		newCred := models.WebAuthnCredential{
			ID:           uuid.New().String(),
			UserID:       user.ID,
			Name:         credName,
			CredentialID: credential.ID,
			PublicKey:    credential.PublicKey,
			Flags:        credential.Flags.MsgpByte(),
			SignCount:    uint64(credential.Authenticator.SignCount),
			CreatedAt:    time.Now().Unix(),
			LastUsedAt:   time.Now().Unix(),
		}

		if err := gormDB.Create(&newCred).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "保存凭证失败"})
			return
		}

		setAuthCookie(c, "webauthn_session", "", -1, cfg)
		c.JSON(consts.StatusOK, map[string]string{"success": "true"})
	}
}

func WebAuthnLoginStart(gormDB *gorm.DB, cfg *db.Config) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if !cfg.WebAuthn.Enabled || cfg.WebAuthn.RPID == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "WebAuthn未启用"})
			return
		}

		w, err := newWebAuthnInstance(cfg)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "初始化WebAuthn失败"})
			return
		}

		assertion, sessionData, err := w.BeginDiscoverableLogin()
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "开始登录失败"})
			return
		}

		sessionID := generateSessionID()
		saveSession(gormDB, sessionID, sessionData)
		setAuthCookie(c, "webauthn_session", sessionID, 300, cfg)

		authOptions := assertion.Response
		c.JSON(consts.StatusOK, map[string]any{
			"challenge":        base64.RawURLEncoding.EncodeToString(authOptions.Challenge),
			"timeout":          authOptions.Timeout,
			"rpId":             authOptions.RelyingPartyID,
			"allowCredentials": authOptions.AllowedCredentials,
			"userVerification": authOptions.UserVerification,
		})
	}
}

func WebAuthnLoginFinish(gormDB *gorm.DB, cfg *db.Config) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if !cfg.WebAuthn.Enabled || cfg.WebAuthn.RPID == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "WebAuthn未启用"})
			return
		}

		sessionID := string(c.Cookie("webauthn_session"))
		if sessionID == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "会话无效"})
			return
		}
		defer deleteSession(gormDB, sessionID)

		sessionRaw := loadSession(gormDB, sessionID)
		if sessionRaw == nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "会话已过期"})
			return
		}
		sessionData, ok := sessionRaw.(*webauthn.SessionData)
		if !ok {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "会话类型错误"})
			return
		}

		w, err := newWebAuthnInstance(cfg)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "初始化WebAuthn失败"})
			return
		}

		body, err := c.Body()
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "读取请求体失败"})
			return
		}

		parsedAssertion, err := protocol.ParseCredentialRequestResponseBytes(body)
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "解析登录响应失败: " + err.Error()})
			return
		}

		handler := func(rawID []byte, userHandle []byte) (webauthn.User, error) {
			return loadWebAuthnUser(gormDB, userHandle)
		}

		user, credential, err := w.ValidatePasskeyLogin(handler, *sessionData, parsedAssertion)
		if err != nil {
			slog.Error("webauthn login validation failed", "error", err, "rpID", cfg.WebAuthn.RPID, "rpOrigins", cfg.WebAuthn.RPOrigins)
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "Passkey验证失败: " + err.Error()})
			return
		}

		var cred models.WebAuthnCredential
		if err := gormDB.Where("credential_id = ?", credential.ID).First(&cred).Error; err == nil {
			gormDB.Model(&cred).Updates(map[string]any{
				"sign_count":   credential.Authenticator.SignCount,
				"last_used_at": time.Now().Unix(),
			})
		}

		var dbUser models.User
		if err := gormDB.Where("id = ?", string(user.WebAuthnID())).First(&dbUser).Error; err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "用户不存在"})
			return
		}

		token, err := generateToken(&dbUser)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "生成token失败"})
			return
		}

		setAuthCookie(c, "webauthn_session", "", -1, cfg)
		setAuthCookie(c, "auth_token", token, cookieMaxAge, cfg)
		c.JSON(consts.StatusOK, map[string]any{
			"user": map[string]any{
				"id":       dbUser.ID,
				"username": dbUser.Username,
				"role":     dbUser.Role,
			},
		})
	}
}

func WebAuthnListCredentials(gormDB *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		claims := middleware.GetUser(c)
		if claims == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		var creds []models.WebAuthnCredential
		gormDB.Where("user_id = ?", claims.UserID).Find(&creds)

		result := make([]map[string]any, len(creds))
		for i := range creds {
			cred := &creds[i]
			result[i] = map[string]any{
				"id":         cred.ID,
				"name":       cred.Name,
				"createdAt":  cred.CreatedAt,
				"lastUsedAt": cred.LastUsedAt,
			}
		}
		c.JSON(consts.StatusOK, result)
	}
}

func WebAuthnDeleteCredential(gormDB *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		claims := middleware.GetUser(c)
		if claims == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		id := c.Param("id")
		result := gormDB.Where("id = ? AND user_id = ?", id, claims.UserID).Delete(&models.WebAuthnCredential{})
		if result.Error != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": result.Error.Error()})
			return
		}
		if result.RowsAffected == 0 {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "凭证不存在"})
			return
		}

		c.JSON(consts.StatusOK, map[string]bool{"success": true})
	}
}
