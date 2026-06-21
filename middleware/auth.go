package middleware

import (
	"context"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserContextKey contextKey = "user"

type JWTClaims struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

var JWTSecret []byte

func SetJWTSecret(secret string) {
	JWTSecret = []byte(secret)
}

func AuthRequired() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			c.Abort()
			return
		}

		claims := &JWTClaims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
			return JWTSecret, nil
		})
		if err != nil || !token.Valid {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "登录已过期"})
			c.Abort()
			return
		}

		c.Set(string(UserContextKey), claims)
		c.Next(ctx)
	}
}

func AdminRequired() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		claims, ok := c.Get(string(UserContextKey))
		if !ok {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			c.Abort()
			return
		}
		userClaims, ok := claims.(*JWTClaims)
		if !ok || userClaims.Role != "admin" {
			c.JSON(consts.StatusForbidden, map[string]string{"error": "需要管理员权限"})
			c.Abort()
			return
		}
		c.Next(ctx)
	}
}

func GetUser(c *app.RequestContext) *JWTClaims {
	claims, ok := c.Get(string(UserContextKey))
	if !ok {
		return nil
	}
	userClaims, ok := claims.(*JWTClaims)
	if !ok {
		return nil
	}
	return userClaims
}

func extractToken(c *app.RequestContext) string {
	auth := string(c.GetHeader("Authorization"))
	if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
		return after
	}
	return string(c.Cookie("auth_token"))
}
