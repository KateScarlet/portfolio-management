package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"uuid"

	"portfolio-management/internal/marketsource"
	"portfolio-management/middleware"
	"portfolio-management/models"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var currencyConversionTolerance = decimal.RequireFromString("0.01")

func CalcPrincipal(db *gorm.DB, portfolioID uuid.UUID, targetCurrency string, router *marketsource.Router) (decimal.Decimal, error) {
	return calcPrincipalByQuery(db.Where("portfolio_id = ? AND type IN ?", portfolioID, []string{"transfer_in", "transfer_out"}), targetCurrency, router)
}

func CalcPrincipalByUser(db *gorm.DB, userID uuid.UUID, targetCurrency string, router *marketsource.Router) (decimal.Decimal, error) {
	return calcPrincipalByQuery(db.Where("user_id = ? AND type IN ?", userID, []string{"transfer_in", "transfer_out"}), targetCurrency, router)
}

func calcPrincipalByQuery(query *gorm.DB, targetCurrency string, router *marketsource.Router) (decimal.Decimal, error) {
	var txs []models.FundTransaction
	if err := query.Find(&txs).Error; err != nil {
		return decimal.Zero, err
	}

	byCurrency := make(map[string]decimal.Decimal)
	for i := range txs {
		if txs[i].Type == "transfer_in" {
			byCurrency[txs[i].Currency] = byCurrency[txs[i].Currency].Add(txs[i].Amount)
		} else {
			byCurrency[txs[i].Currency] = byCurrency[txs[i].Currency].Sub(txs[i].Amount)
		}
	}

	total := decimal.Zero
	for currency, amount := range byCurrency {
		if currency == targetCurrency || amount.IsZero() {
			total = total.Add(amount)
			continue
		}
		rate, err := router.ExchangeRate(uuid.UUID{}, currency+targetCurrency)
		if err != nil {
			return decimal.Zero, fmt.Errorf("获取 %s 汇率失败: %w", currency+targetCurrency, err)
		}
		total = total.Add(amount.Mul(rate))
	}
	return total, nil
}

func ListFundTransactions(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID, err := uuid.Parse(c.Param("pid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid portfolio id"})
			return
		}
		owns, err := userOwnsPortfolio(db, user.UserID, portfolioID)
		if err != nil {
			slog.Error("failed to check portfolio ownership", "error", err)
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "数据库错误"})
			return
		}
		if !owns {
			c.JSON(consts.StatusForbidden, map[string]string{"error": "无权访问此组合"})
			return
		}

		query := db.Where("portfolio_id = ?", portfolioID)
		if txType := c.Query("type"); txType != "" {
			query = query.Where("type = ?", txType)
		}

		var transactions []models.FundTransaction
		if err := query.Order("created_at DESC").Find(&transactions).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if transactions == nil {
			transactions = []models.FundTransaction{}
		}
		c.JSON(consts.StatusOK, transactions)
	}
}

func TransferIn(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID, err := uuid.Parse(c.Param("pid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid portfolio id"})
			return
		}
		owns, err := userOwnsPortfolio(db, user.UserID, portfolioID)
		if err != nil {
			slog.Error("failed to check portfolio ownership", "error", err)
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "数据库错误"})
			return
		}
		if !owns {
			c.JSON(consts.StatusForbidden, map[string]string{"error": "无权访问此组合"})
			return
		}

		var body struct {
			Currency string          `json:"currency"`
			Amount   decimal.Decimal `json:"amount"`
			Note     string          `json:"note"`
		}
		if err := c.BindJSON(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if body.Currency == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "currency 不能为空"})
			return
		}
		if !body.Amount.IsPositive() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "金额必须大于0"})
			return
		}

		err = db.Transaction(func(tx *gorm.DB) error {
			if err := addAvailableFund(tx, user.UserID, portfolioID, body.Currency, body.Amount); err != nil {
				return err
			}
			return tx.Create(&models.FundTransaction{
				ID:          uuid.New(),
				UserID:      user.UserID,
				PortfolioID: portfolioID,
				Type:        "transfer_in",
				Amount:      body.Amount,
				Currency:    body.Currency,
				Note:        body.Note,
			}).Error
		})
		if err != nil {
			httpErr := &httpError{}
			if errors.As(err, &httpErr) {
				c.JSON(httpErr.status, map[string]string{"error": httpErr.msg})
			} else {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return
		}

		c.JSON(consts.StatusCreated, map[string]string{"status": "ok"})
	}
}

func TransferOut(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID, err := uuid.Parse(c.Param("pid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid portfolio id"})
			return
		}
		owns, err := userOwnsPortfolio(db, user.UserID, portfolioID)
		if err != nil {
			slog.Error("failed to check portfolio ownership", "error", err)
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "数据库错误"})
			return
		}
		if !owns {
			c.JSON(consts.StatusForbidden, map[string]string{"error": "无权访问此组合"})
			return
		}

		var body struct {
			Currency string          `json:"currency"`
			Amount   decimal.Decimal `json:"amount"`
			Note     string          `json:"note"`
		}
		if err := c.BindJSON(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if body.Currency == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "currency 不能为空"})
			return
		}
		if !body.Amount.IsPositive() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "金额必须大于0"})
			return
		}

		err = db.Transaction(func(tx *gorm.DB) error {
			if err := deductAvailableFund(tx, user.UserID, portfolioID, body.Currency, body.Amount); err != nil {
				return err
			}
			return tx.Create(&models.FundTransaction{
				ID:          uuid.New(),
				UserID:      user.UserID,
				PortfolioID: portfolioID,
				Type:        "transfer_out",
				Amount:      body.Amount,
				Currency:    body.Currency,
				Note:        body.Note,
			}).Error
		})
		if err != nil {
			httpErr := &httpError{}
			ok := errors.As(err, &httpErr)
			if ok {
				c.JSON(httpErr.status, map[string]string{"error": httpErr.msg})
			} else {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return
		}

		c.JSON(consts.StatusCreated, map[string]string{"status": "ok"})
	}
}

func TransferBetween(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID, err := uuid.Parse(c.Param("pid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid portfolio id"})
			return
		}
		owns, err := userOwnsPortfolio(db, user.UserID, portfolioID)
		if err != nil {
			slog.Error("failed to check portfolio ownership", "error", err)
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "数据库错误"})
			return
		}
		if !owns {
			c.JSON(consts.StatusForbidden, map[string]string{"error": "无权访问此组合"})
			return
		}

		var body struct {
			Currency          string          `json:"currency"`
			Amount            decimal.Decimal `json:"amount"`
			TargetPortfolioID string          `json:"targetPortfolioId"`
			Note              string          `json:"note"`
		}
		if err := c.BindJSON(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if body.Currency == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "currency 不能为空"})
			return
		}
		if !body.Amount.IsPositive() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "金额必须大于0"})
			return
		}
		if body.TargetPortfolioID == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "targetPortfolioId 不能为空"})
			return
		}
		targetPortfolioID, err := uuid.Parse(body.TargetPortfolioID)
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid target portfolio id"})
			return
		}
		if targetPortfolioID == portfolioID {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "不能划转到同一个组合"})
			return
		}
		ownsTarget, err := userOwnsPortfolio(db, user.UserID, targetPortfolioID)
		if err != nil {
			slog.Error("failed to check target portfolio ownership", "error", err)
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "数据库错误"})
			return
		}
		if !ownsTarget {
			c.JSON(consts.StatusForbidden, map[string]string{"error": "无权访问目标组合"})
			return
		}

		err = db.Transaction(func(tx *gorm.DB) error {
			if err := deductAvailableFund(tx, user.UserID, portfolioID, body.Currency, body.Amount); err != nil {
				return err
			}
			if err := addAvailableFund(tx, user.UserID, targetPortfolioID, body.Currency, body.Amount); err != nil {
				return err
			}

			if err := tx.Create(&models.FundTransaction{
				ID:                uuid.New(),
				UserID:            user.UserID,
				PortfolioID:       portfolioID,
				Type:              "transfer_out",
				Amount:            body.Amount,
				Currency:          body.Currency,
				TargetPortfolioID: &targetPortfolioID,
				Note:              body.Note,
			}).Error; err != nil {
				return err
			}
			return tx.Create(&models.FundTransaction{
				ID:                uuid.New(),
				UserID:            user.UserID,
				PortfolioID:       targetPortfolioID,
				Type:              "transfer_in",
				Amount:            body.Amount,
				Currency:          body.Currency,
				TargetPortfolioID: &portfolioID,
				Note:              "从其他组合划转转入",
			}).Error
		})
		if err != nil {
			httpErr := &httpError{}
			ok := errors.As(err, &httpErr)
			if ok {
				c.JSON(httpErr.status, map[string]string{"error": httpErr.msg})
			} else {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return
		}

		c.JSON(consts.StatusCreated, map[string]string{"status": "ok"})
	}
}

func ConvertCurrency(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID, err := uuid.Parse(c.Param("pid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid portfolio id"})
			return
		}
		owns, err := userOwnsPortfolio(db, user.UserID, portfolioID)
		if err != nil {
			slog.Error("failed to check portfolio ownership", "error", err)
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": "数据库错误"})
			return
		}
		if !owns {
			c.JSON(consts.StatusForbidden, map[string]string{"error": "无权访问此组合"})
			return
		}

		var body struct {
			FromCurrency string          `json:"fromCurrency"`
			ToCurrency   string          `json:"toCurrency"`
			FromAmount   decimal.Decimal `json:"fromAmount"`
			ToAmount     decimal.Decimal `json:"toAmount"`
			ExchangeRate decimal.Decimal `json:"exchangeRate"`
		}
		if err := c.BindJSON(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if body.FromCurrency == "" || body.ToCurrency == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "fromCurrency 和 toCurrency 不能为空"})
			return
		}
		if body.FromCurrency == body.ToCurrency {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "源币种和目标币种不能相同"})
			return
		}
		if !body.FromAmount.IsPositive() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "fromAmount 必须大于0"})
			return
		}
		if !body.ToAmount.IsPositive() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "toAmount 必须大于0"})
			return
		}
		if !body.ExchangeRate.IsPositive() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "exchangeRate 必须大于0"})
			return
		}
		expectedTo := body.FromAmount.Mul(body.ExchangeRate)
		if expectedTo.Sub(body.ToAmount).Abs().GreaterThan(currencyConversionTolerance) {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "汇率与金额不一致"})
			return
		}

		err = db.Transaction(func(tx *gorm.DB) error {
			if err := deductAvailableFund(tx, user.UserID, portfolioID, body.FromCurrency, body.FromAmount); err != nil {
				return err
			}
			if err := addAvailableFund(tx, user.UserID, portfolioID, body.ToCurrency, body.ToAmount); err != nil {
				return err
			}
			return tx.Create(&models.FundTransaction{
				ID:             uuid.New(),
				UserID:         user.UserID,
				PortfolioID:    portfolioID,
				Type:           "convert",
				Amount:         body.FromAmount,
				Currency:       body.FromCurrency,
				TargetAmount:   body.ToAmount,
				TargetCurrency: body.ToCurrency,
				ExchangeRate:   body.ExchangeRate,
			}).Error
		})
		if err != nil {
			httpErr := &httpError{}
			ok := errors.As(err, &httpErr)
			if ok {
				c.JSON(httpErr.status, map[string]string{"error": httpErr.msg})
			} else {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return
		}

		c.JSON(consts.StatusCreated, map[string]string{"status": "ok"})
	}
}

func addAvailableFund(tx *gorm.DB, userID, portfolioID uuid.UUID, currency string, amount decimal.Decimal) error {
	_, err := addAvailableFundReturningBalance(tx, userID, portfolioID, currency, amount)
	return err
}

func addAvailableFundReturningBalance(tx *gorm.DB, userID, portfolioID uuid.UUID, currency string, amount decimal.Decimal) (decimal.Decimal, error) {
	fund := models.AvailableFund{
		ID:          uuid.New(),
		UserID:      userID,
		PortfolioID: portfolioID,
		Currency:    currency,
		Amount:      amount,
	}
	err := tx.Clauses(
		clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "portfolio_id"}, {Name: "currency"}},
			DoUpdates: clause.Assignments(map[string]any{
				"amount": gorm.Expr("available_funds.amount + EXCLUDED.amount"),
			}),
		},
		clause.Returning{Columns: []clause.Column{{Name: "amount"}}},
	).Create(&fund).Error
	return fund.Amount, err
}

func deductAvailableFund(tx *gorm.DB, userID, portfolioID uuid.UUID, currency string, amount decimal.Decimal) error {
	result := tx.Model(&models.AvailableFund{}).
		Where("user_id = ? AND portfolio_id = ? AND currency = ?", userID, portfolioID, currency).
		Where("amount >= ?", amount).
		UpdateColumn("amount", gorm.Expr("amount - ?", amount))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}

	var fund models.AvailableFund
	err := tx.Where("user_id = ? AND portfolio_id = ? AND currency = ?", userID, portfolioID, currency).First(&fund).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &httpError{status: consts.StatusBadRequest, msg: "可用资金不足: " + currency + " 余额为 0"}
	}
	if err != nil {
		return err
	}
	return &httpError{status: consts.StatusBadRequest, msg: "可用资金不足: " + currency + " 可用 " + fund.Amount.StringFixed(2) + ", 需要 " + amount.StringFixed(2)}
}
