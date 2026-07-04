package handlers

import (
	"context"
	"portfolio-management/marketsource"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func ListAccounts(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		accounts, err := gorm.G[models.Account](db).Where("user_id = ?", user.UserID).Order("created_at ASC").Find(ctx)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(consts.StatusOK, accounts)
	}
}

func CreateAccount(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		var body struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Broker      string `json:"broker"`
		}
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		if body.Name == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "账户名称不能为空"})
			return
		}

		account := models.Account{
			ID:          uuid.New().String(),
			UserID:      user.UserID,
			Name:        body.Name,
			Description: body.Description,
			Broker:      body.Broker,
			CreatedAt:   time.Now().UnixMilli(),
		}

		if err := gorm.G[models.Account](db).Create(ctx, &account); err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusCreated, account)
	}
}

func UpdateAccount(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		id := c.Param("id")
		account, err := gorm.G[models.Account](db).Where("user_id = ? AND id = ?", user.UserID, id).First(ctx)
		if err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "Account not found"})
			return
		}

		var body struct {
			Name        *string `json:"name"`
			Description *string `json:"description"`
			Broker      *string `json:"broker"`
		}
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		updates := map[string]any{}
		if body.Name != nil {
			updates["name"] = *body.Name
		}
		if body.Description != nil {
			updates["description"] = *body.Description
		}
		if body.Broker != nil {
			updates["broker"] = *body.Broker
		}

		if len(updates) == 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "no fields to update"})
			return
		}

		if _, err := gorm.G[map[string]any](db).Table("accounts").Where("user_id = ? AND id = ?", user.UserID, id).Updates(ctx, updates); err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		account, err = gorm.G[models.Account](db).Where("user_id = ? AND id = ?", user.UserID, id).First(ctx)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(consts.StatusOK, account)
	}
}

func DeleteAccount(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		id := c.Param("id")

		_, err := gorm.G[models.Account](db).Where("user_id = ? AND id = ?", user.UserID, id).First(ctx)
		if err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "Account not found"})
			return
		}

		err = db.Transaction(func(tx *gorm.DB) error {
			// Clear account_id on holdings belonging to this account
			if _, err := gorm.G[map[string]any](tx).Table("holdings").Where("user_id = ? AND account_id = ?", user.UserID, id).Updates(ctx, map[string]any{"account_id": ""}); err != nil {
				return err
			}
			// Delete the account
			if _, err := gorm.G[models.Account](tx).Where("id = ?", id).Delete(ctx); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusOK, map[string]bool{"success": true})
	}
}

// HoldingWithAccount is a Holding enriched with account name for the account view.
type HoldingWithAccount struct {
	models.Holding
	AccountName string `json:"accountName"`
}

// ListAllAccountHoldings returns all holdings across all portfolios with account names.
func ListAllAccountHoldings(db *gorm.DB, router *marketsource.Router) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		// Load all accounts for name lookup
		accounts, err := gorm.G[models.Account](db).Where("user_id = ?", user.UserID).Find(ctx)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		accountNameMap := make(map[string]string, len(accounts))
		for _, a := range accounts {
			accountNameMap[a.ID] = a.Name
		}

		// Load all holdings across all portfolios
		var holdings []models.Holding
		if err := db.Where("user_id = ?", user.UserID).Order("asset_id").Find(&holdings).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// Filter by account_id if specified
		filterAccountID := c.Query("account_id")
		if filterAccountID != "" {
			filtered := make([]models.Holding, 0)
			for i := range holdings {
				if holdings[i].AccountID == filterAccountID {
					filtered = append(filtered, holdings[i])
				}
			}
			holdings = filtered
		}

		// Currency conversion
		if displayCurrency := c.Query("currency"); displayCurrency != "" {
			if err := convertHoldingsCurrency(holdings, displayCurrency, router, user.UserID); err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}

		// Enrich with account names
		result := make([]HoldingWithAccount, len(holdings))
		for i := range holdings {
			result[i] = HoldingWithAccount{
				Holding:     holdings[i],
				AccountName: accountNameMap[holdings[i].AccountID],
			}
		}

		c.JSON(consts.StatusOK, result)
	}
}

// ListAccountHoldings returns raw holdings for a specific account.
func ListAccountHoldings(db *gorm.DB, router *marketsource.Router) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		accountID := c.Param("id")

		// Verify account ownership
		_, err := gorm.G[models.Account](db).Where("user_id = ? AND id = ?", user.UserID, accountID).First(ctx)
		if err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "Account not found"})
			return
		}

		var holdings []models.Holding
		if err := db.Where("user_id = ? AND account_id = ?", user.UserID, accountID).Order("asset_id").Find(&holdings).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if displayCurrency := c.Query("currency"); displayCurrency != "" {
			if err := convertHoldingsCurrency(holdings, displayCurrency, router, user.UserID); err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}

		c.JSON(consts.StatusOK, holdings)
	}
}
