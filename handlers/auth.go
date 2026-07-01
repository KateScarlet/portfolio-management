package handlers

import (
	"context"
	"errors"
	"portfolio-management/db"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const cookieMaxAge = 7 * 24 * 3600

var ErrUserNotFound = errors.New("user not found")

func setAuthCookie(c *app.RequestContext, name, value string, maxAge int, cfg *db.Config) {
	secure := false
	if cfg != nil {
		secure = cfg.CookieSecure
	}
	c.SetCookie(name, value, maxAge, "/", "", 2, secure, true)
}

func Login(db *gorm.DB, cfg *db.Config) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"` //nolint:gosec // Request body field
		}
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		if body.Username == "" || body.Password == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "用户名和密码不能为空"})
			return
		}

		user, err := gorm.G[models.User](db).Where("username = ?", body.Username).First(ctx)
		if err != nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "用户名或密码错误"})
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(body.Password)); err != nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "用户名或密码错误"})
			return
		}

		token, err := generateToken(&user)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "生成token失败"})
			return
		}

		setAuthCookie(c, "auth_token", token, cookieMaxAge, cfg)
		c.JSON(consts.StatusOK, map[string]any{
			"user": map[string]any{
				"id":       user.ID,
				"username": user.Username,
				"role":     user.Role,
			},
		})
	}
}

func Logout(cfg *db.Config) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		setAuthCookie(c, "auth_token", "", -1, cfg)
		c.JSON(consts.StatusOK, map[string]bool{"success": true})
	}
}

func Me(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		claims := middleware.GetUser(c)
		if claims == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		user, err := gorm.G[models.User](db).Where("id = ?", claims.UserID).First(ctx)
		if err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "用户不存在"})
			return
		}

		c.JSON(consts.StatusOK, map[string]any{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		})
	}
}

func Register(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"` //nolint:gosec // Request body field
			Role     string `json:"role"`
		}
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		if body.Username == "" || body.Password == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "用户名和密码不能为空"})
			return
		}

		if len(body.Password) < 6 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "密码至少6位"})
			return
		}

		if body.Role != "user" && body.Role != "admin" {
			body.Role = "user"
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "加密密码失败"})
			return
		}

		user := models.User{
			ID:        uuid.New().String(),
			Username:  body.Username,
			Password:  string(hashedPassword),
			Role:      body.Role,
			CreatedAt: time.Now().UnixMilli(),
		}

		if err := gorm.G[models.User](db).Create(ctx, &user); err != nil {
			c.JSON(consts.StatusConflict, map[string]string{"error": "用户名已存在"})
			return
		}

		if err := ensureDefaultPortfolio(db, user.ID); err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "创建默认组合失败"})
			return
		}

		c.JSON(consts.StatusOK, map[string]any{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		})
	}
}

func ListUsers(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		users, err := gorm.G[models.User](db).Find(ctx)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		result := make([]map[string]any, len(users))
		for i, u := range users {
			result[i] = map[string]any{
				"id":       u.ID,
				"username": u.Username,
				"role":     u.Role,
			}
		}
		c.JSON(consts.StatusOK, result)
	}
}

func DeleteUser(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		id := c.Param("id")

		claims := middleware.GetUser(c)
		if claims != nil && claims.UserID == id {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "不能删除自己"})
			return
		}

		err := db.Transaction(func(tx *gorm.DB) error {
			if _, err := gorm.G[models.Holding](tx).Where("user_id = ?", id).Delete(ctx); err != nil {
				return err
			}
			if _, err := gorm.G[models.PortfolioRecord](tx).Where("user_id = ?", id).Delete(ctx); err != nil {
				return err
			}
			if _, err := gorm.G[models.Setting](tx).Where("user_id = ?", id).Delete(ctx); err != nil {
				return err
			}
			if _, err := gorm.G[models.AvailableFund](tx).Where("user_id = ?", id).Delete(ctx); err != nil {
				return err
			}
			if _, err := gorm.G[models.FundTransaction](tx).Where("user_id = ?", id).Delete(ctx); err != nil {
				return err
			}
			if _, err := gorm.G[models.Portfolio](tx).Where("user_id = ?", id).Delete(ctx); err != nil {
				return err
			}
			if _, err := gorm.G[models.WebAuthnCredential](tx).Where("user_id = ?", id).Delete(ctx); err != nil {
				return err
			}
			rows, err := gorm.G[models.User](tx).Where("id = ?", id).Delete(ctx)
			if err != nil {
				return err
			}
			if rows == 0 {
				return ErrUserNotFound
			}
			return nil
		})
		if err != nil {
			if errors.Is(err, ErrUserNotFound) {
				c.JSON(consts.StatusNotFound, map[string]string{"error": "用户不存在"})
			} else {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return
		}

		c.JSON(consts.StatusOK, map[string]bool{"success": true})
	}
}

func CreateUserForSetup(db *gorm.DB, username, password, role string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := models.User{
		ID:        uuid.New().String(),
		Username:  username,
		Password:  string(hashedPassword),
		Role:      role,
		CreatedAt: time.Now().UnixMilli(),
	}

	if err := gorm.G[models.User](db).Create(context.Background(), &user); err != nil {
		return err
	}

	return ensureDefaultPortfolio(db, user.ID)
}

func ensureDefaultPortfolio(db *gorm.DB, userID string) error {
	var count int64
	db.Model(&models.Portfolio{}).Where("user_id = ?", userID).Count(&count)
	if count > 0 {
		return nil
	}
	portfolio := models.Portfolio{
		ID:        uuid.New().String(),
		UserID:    userID,
		Name:      "默认组合",
		IsDefault: true,
		CreatedAt: time.Now().UnixMilli(),
	}
	ctx := context.Background()
	if err := gorm.G[models.Portfolio](db).Create(ctx, &portfolio); err != nil {
		return err
	}
	if _, err := gorm.G[models.Holding](db).Where("user_id = ? AND (portfolio_id = '' OR portfolio_id IS NULL)", userID).Update(ctx, "portfolio_id", portfolio.ID); err != nil {
		return err
	}
	if _, err := gorm.G[models.PortfolioRecord](db).Where("user_id = ? AND (portfolio_id = '' OR portfolio_id IS NULL)", userID).Update(ctx, "portfolio_id", portfolio.ID); err != nil {
		return err
	}
	if _, err := gorm.G[models.Setting](db).Where("user_id = ? AND (portfolio_id = '' OR portfolio_id IS NULL)", userID).Update(ctx, "portfolio_id", portfolio.ID); err != nil {
		return err
	}
	return nil
}

func generateToken(user *models.User) (string, error) {
	claims := &middleware.JWTClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(middleware.JWTSecret)
}
