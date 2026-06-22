package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"portfolio-management/db"
	"portfolio-management/models"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"gorm.io/gorm"
)

func getProvider(ctx context.Context, cfg *db.Config) (*oidc.Provider, error) {
	return oidc.NewProvider(ctx, cfg.OIDC.Issuer)
}

func newOAuth2Config(ctx context.Context, cfg *db.Config) (*oauth2.Config, error) {
	provider, err := getProvider(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &oauth2.Config{
		ClientID:     cfg.OIDC.ClientID,
		ClientSecret: cfg.OIDC.ClientSecret,
		RedirectURL:  cfg.OIDC.RedirectURL,
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		Endpoint:     provider.Endpoint(),
	}, nil
}

func OIDCLogin(cfg *db.Config) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if !cfg.OIDC.Enabled {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "SSO未启用"})
			return
		}

		state := generateState()
		c.SetCookie("oidc_state", state, 600, "/", "", 0, false, true)

		oauth2Config, err := newOAuth2Config(ctx, cfg)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "连接OIDC Provider失败"})
			return
		}
		authURL := oauth2Config.AuthCodeURL(state)
		c.Redirect(consts.StatusFound, []byte(authURL))
	}
}

func OIDCCallback(db *gorm.DB, cfg *db.Config) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if !cfg.OIDC.Enabled {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "SSO未启用"})
			return
		}

		state := string(c.Cookie("oidc_state"))
		c.SetCookie("oidc_state", "", -1, "/", "", 0, false, true)

		queryState := c.Query("state")
		if state == "" || state != queryState {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "state参数无效"})
			return
		}

		code := c.Query("code")
		if code == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "缺少授权码"})
			return
		}

		oauth2Config, err := newOAuth2Config(ctx, cfg)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "连接OIDC Provider失败"})
			return
		}
		token, err := oauth2Config.Exchange(ctx, code)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "换取token失败"})
			return
		}

		provider, err := getProvider(ctx, cfg)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "连接OIDC Provider失败"})
			return
		}

		oidcConfig := &oidc.Config{
			ClientID: cfg.OIDC.ClientID,
		}
		verifier := provider.Verifier(oidcConfig)
		rawIDToken, ok := token.Extra("id_token").(string)
		if !ok {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "缺少id_token"})
			return
		}

		idToken, err := verifier.Verify(ctx, rawIDToken)
		if err != nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "验证id_token失败"})
			return
		}

		var claims struct {
			Sub           string `json:"sub"`
			Name          string `json:"name"`
			Email         string `json:"email"`
			PreferredUser string `json:"preferred_username"`
		}
		if err := idToken.Claims(&claims); err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "解析用户信息失败"})
			return
		}

		sub := claims.Sub
		username := claims.PreferredUser
		if username == "" {
			username = claims.Email
		}
		if username == "" {
			username = claims.Name
		}
		if username == "" {
			username = sub
		}

		var user models.User
		err = db.Where("sso_provider = ? AND sso_id = ?", "oidc", sub).First(&user).Error
		if err == gorm.ErrRecordNotFound {
			user = models.User{
				ID:          uuid.New().String(),
				Username:    username,
				Password:    "",
				Role:        "user",
				SSOProvider: "oidc",
				SSOId:       sub,
				CreatedAt:   time.Now().Unix(),
			}
			if err := db.Create(&user).Error; err != nil {
				c.JSON(consts.StatusConflict, map[string]string{"error": "创建用户失败: " + err.Error()})
				return
			}
		} else if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "查询用户失败"})
			return
		}

		tokenStr, err := generateToken(&user)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "生成token失败"})
			return
		}

		c.SetCookie("auth_token", tokenStr, cookieMaxAge, "/", "", 0, false, true)
		c.Redirect(consts.StatusFound, []byte("/"))
	}
}

func OIDCStatus(cfg *db.Config) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.JSON(consts.StatusOK, map[string]bool{"enabled": cfg.OIDC.Enabled})
	}
}

func GetOIDCConfig(cfg *db.Config) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.JSON(consts.StatusOK, map[string]any{
			"enabled":      cfg.OIDC.Enabled,
			"issuer":       cfg.OIDC.Issuer,
			"clientID":     cfg.OIDC.ClientID,
			"clientSecret": cfg.OIDC.ClientSecret,
			"redirectURL":  cfg.OIDC.RedirectURL,
		})
	}
}

func UpdateOIDCConfig(cfg *db.Config) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var body struct {
			Enabled      *bool  `json:"enabled"`
			Issuer       string `json:"issuer"`
			ClientID     string `json:"clientID"`
			ClientSecret string `json:"clientSecret"`
			RedirectURL  string `json:"redirectURL"`
		}
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		if body.Enabled != nil {
			cfg.OIDC.Enabled = *body.Enabled
		}
		cfg.OIDC.Issuer = body.Issuer
		cfg.OIDC.ClientID = body.ClientID
		cfg.OIDC.ClientSecret = body.ClientSecret
		cfg.OIDC.RedirectURL = body.RedirectURL

		if err := db.SaveConfig(cfg); err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "保存配置失败"})
			return
		}

		c.JSON(consts.StatusOK, map[string]any{
			"enabled":      cfg.OIDC.Enabled,
			"issuer":       cfg.OIDC.Issuer,
			"clientID":     cfg.OIDC.ClientID,
			"clientSecret": cfg.OIDC.ClientSecret,
			"redirectURL":  cfg.OIDC.RedirectURL,
		})
	}
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func GetWebAuthnConfig(cfg *db.Config) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.JSON(consts.StatusOK, map[string]any{
			"enabled":   cfg.WebAuthn.Enabled,
			"rpid":      cfg.WebAuthn.RPID,
			"rpOrigins": cfg.WebAuthn.RPOrigins,
		})
	}
}

func UpdateWebAuthnConfig(cfg *db.Config) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var body struct {
			Enabled   *bool    `json:"enabled"`
			RPID      string   `json:"rpid"`
			RPOrigins []string `json:"rpOrigins"`
		}
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		if body.Enabled != nil {
			cfg.WebAuthn.Enabled = *body.Enabled
		}
		cfg.WebAuthn.RPID = body.RPID
		cfg.WebAuthn.RPOrigins = body.RPOrigins

		if err := db.SaveConfig(cfg); err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "保存配置失败"})
			return
		}

		c.JSON(consts.StatusOK, map[string]any{
			"enabled":   cfg.WebAuthn.Enabled,
			"rpid":      cfg.WebAuthn.RPID,
			"rpOrigins": cfg.WebAuthn.RPOrigins,
		})
	}
}
