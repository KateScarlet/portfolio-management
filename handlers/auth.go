package handlers

import (
	"context"
	"errors"
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

func Login(db *gorm.DB) app.HandlerFunc {
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

		var user models.User
		if err := db.Where("username = ?", body.Username).First(&user).Error; err != nil {
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

		c.SetCookie("auth_token", token, cookieMaxAge, "/", "", 0, false, true)
		c.JSON(consts.StatusOK, map[string]any{
			"user": map[string]any{
				"id":       user.ID,
				"username": user.Username,
				"role":     user.Role,
			},
		})
	}
}

func Logout() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.SetCookie("auth_token", "", -1, "/", "", 0, false, true)
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

		var user models.User
		if err := db.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
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

		if body.Role == "" {
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
			CreatedAt: time.Now().Unix(),
		}

		if err := db.Create(&user).Error; err != nil {
			c.JSON(consts.StatusConflict, map[string]string{"error": "用户名已存在"})
			return
		}

		ensureDefaultPortfolio(db, user.ID)

		c.JSON(consts.StatusOK, map[string]any{
			"id":       user.ID,
			"username": user.Username,
			"role":     user.Role,
		})
	}
}

func ListUsers(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var users []models.User
		if err := db.Find(&users).Error; err != nil {
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
			if err := tx.Where("user_id = ?", id).Delete(&models.Holding{}).Error; err != nil {
				return err
			}
			if err := tx.Where("user_id = ?", id).Delete(&models.PortfolioRecord{}).Error; err != nil {
				return err
			}
			if err := tx.Where("user_id = ?", id).Delete(&models.Setting{}).Error; err != nil {
				return err
			}
			if err := tx.Where("user_id = ?", id).Delete(&models.Portfolio{}).Error; err != nil {
				return err
			}
			if err := tx.Where("user_id = ?", id).Delete(&models.WebAuthnCredential{}).Error; err != nil {
				return err
			}
		result := tx.Delete(&models.User{}, "id = ?", id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
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
		CreatedAt: time.Now().Unix(),
	}

	if err := db.Create(&user).Error; err != nil {
		return err
	}

	ensureDefaultPortfolio(db, user.ID)
	return nil
}

func ensureDefaultPortfolio(db *gorm.DB, userID string) {
	var count int64
	db.Model(&models.Portfolio{}).Where("user_id = ?", userID).Count(&count)
	if count > 0 {
		return
	}
	portfolio := models.Portfolio{
		ID:        uuid.New().String(),
		UserID:    userID,
		Name:      "默认组合",
		IsDefault: true,
		CreatedAt: time.Now().UnixMilli(),
	}
	if err := db.Create(&portfolio).Error; err != nil {
		return
	}
	db.Model(&models.Holding{}).Where("user_id = ? AND (portfolio_id = '' OR portfolio_id IS NULL)", userID).Update("portfolio_id", portfolio.ID)
	db.Model(&models.PortfolioRecord{}).Where("user_id = ? AND (portfolio_id = '' OR portfolio_id IS NULL)", userID).Update("portfolio_id", portfolio.ID)
	db.Model(&models.Setting{}).Where("user_id = ? AND (portfolio_id = '' OR portfolio_id IS NULL)", userID).Update("portfolio_id", portfolio.ID)
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
