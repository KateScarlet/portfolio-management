package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"portfolio-management/marketsource"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"strconv"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"log/slog"
)

type httpError struct {
	status int
	msg    string
}

func (e *httpError) Error() string { return e.msg }

func convertHoldingsCurrency(holdings []models.Holding, targetCurrency string, router *marketsource.Router, userID string) error {
	for i := range holdings {
		h := &holdings[i]
		if h.Currency == "" || h.Currency == targetCurrency {
			continue
		}
		pair := h.Currency + targetCurrency
		rate, err := router.ExchangeRate(userID, pair)
		if err != nil {
			return fmt.Errorf("获取 %s 汇率失败: %w", pair, err)
		}
		h.Value *= rate
		h.Cost *= rate
		h.CostPrice *= rate
		for j := range h.Lots {
			h.Lots[j].Fee *= rate
			h.Lots[j].Cost *= rate
			h.Lots[j].CostPrice *= rate
			h.Lots[j].ValueAdded *= rate
		}
		h.Currency = targetCurrency
	}
	return nil
}

func ListHoldings(db *gorm.DB, router *marketsource.Router) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID := c.Param("pid")
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

		var holdings []models.Holding
		if err := db.Where("portfolio_id = ?", portfolioID).Order("asset_id").Find(&holdings).Error; err != nil {
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

type CreateHoldingInput struct {
	models.Holding
}

func CreateHolding(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID := c.Param("pid")
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

		var input CreateHoldingInput
		if err := c.BindJSON(&input); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		// Normalize symbol to canonical format (e.g., 600519.SH, AAPL.US)
		if input.Symbol != "" && input.Market != "" {
			input.Symbol = marketsource.NormalizeSymbol(input.Symbol, input.Market)
		}

		if input.AssetId == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "assetId is required"})
			return
		}
		validAssets := map[string]bool{"stocks": true, "bonds": true, "cash": true, "commodities": true}
		if !validAssets[input.AssetId] {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid assetId"})
			return
		}
		if input.Shares < 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "shares cannot be negative"})
			return
		}
		if input.Cost < 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "cost cannot be negative"})
			return
		}
		if input.CostPrice < 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "costPrice cannot be negative"})
			return
		}
		if input.Fee < 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "fee cannot be negative"})
			return
		}

		isRegisterOnly := input.Shares == 0 && input.Cost == 0

		var newLot *models.HoldingLot
		if !isRegisterOnly {
			lot := models.HoldingLot{
				ID:         uuid.New().String(),
				Date:       input.Date,
				Shares:     input.Shares,
				CostPrice:  input.CostPrice,
				Cost:       input.Cost,
				ValueAdded: input.Value,
				Fee:        input.Fee,
			}
			if lot.Date == 0 {
				lot.Date = time.Now().UnixMilli()
			}
			newLot = &lot
		}

		var created bool
		var result models.Holding
		err = db.Transaction(func(tx *gorm.DB) error {
			var existing models.Holding
			var res *gorm.DB
			if input.Symbol != "" {
				res = tx.Where("portfolio_id = ? AND symbol = ? AND symbol != ''", portfolioID, input.Symbol).First(&existing)
			} else {
				res = tx.Where("portfolio_id = ? AND name = ? AND asset_id = ? AND symbol = ''", portfolioID, input.Name, input.AssetId).First(&existing)
			}

			if res.Error == nil {
				if isRegisterOnly {
					return &httpError{status: consts.StatusBadRequest, msg: "该资产已存在"}
				}
				existing.Lots = append(existing.Lots, *newLot)
				if existing.Symbol != "" && input.Price > 0 {
					existing.Price = input.Price
				}
				existing.RecalcFromLots()
				created = false
				result = existing
				if err := tx.Save(&existing).Error; err != nil {
					return err
				}
			} else {
				if !errors.Is(res.Error, gorm.ErrRecordNotFound) {
					return res.Error
				}

				input.ID = uuid.New().String()
				input.UserID = user.UserID
				input.PortfolioID = portfolioID
				if newLot != nil {
					input.Lots = models.JSONColumn{*newLot}
				} else {
					input.Lots = models.JSONColumn{}
				}
				input.RecalcFromLots()
				created = true
				result = input.Holding
				if err := tx.Create(&input.Holding).Error; err != nil {
					return err
				}
			}

			if newLot != nil {
				addedCost := newLot.Cost + input.Fee
				if addedCost > 0 {
					holdingCurrency := input.Currency
					if holdingCurrency == "" {
						holdingCurrency = "CNY"
					}

					var af models.AvailableFund
					err := tx.Where("user_id = ? AND portfolio_id = ? AND currency = ?", user.UserID, portfolioID, holdingCurrency).First(&af).Error
					fundsAmount := 0.0
					if err == nil {
						fundsAmount = af.Amount
					} else if !errors.Is(err, gorm.ErrRecordNotFound) {
						return err
					}

					if math.Round(fundsAmount*100) < math.Round(addedCost*100) {
						return &httpError{status: consts.StatusBadRequest, msg: fmt.Sprintf("可用资金不足: %s 可用 %.2f, 需要 %.2f", holdingCurrency, fundsAmount, addedCost)}
					}

					newAmount := fundsAmount - addedCost
					if err == nil {
						if err := tx.Model(&af).Update("amount", newAmount).Error; err != nil {
							return err
						}
					} else {
						if newAmount > 0 {
							if err := tx.Create(&models.AvailableFund{
								ID:          uuid.New().String(),
								UserID:      user.UserID,
								PortfolioID: portfolioID,
								Currency:    holdingCurrency,
								Amount:      newAmount,
							}).Error; err != nil {
								return err
							}
						}
					}

					if err := tx.Create(&models.FundTransaction{
						ID:          uuid.New().String(),
						UserID:      user.UserID,
						PortfolioID: portfolioID,
						Type:        "buy",
						Amount:      addedCost,
						Currency:    holdingCurrency,
						HoldingID:   result.ID,
						CreatedAt:   time.Now().UnixMilli(),
					}).Error; err != nil {
						return err
					}
				}
			}

			return nil
		})
		if err != nil {
			var he *httpError
			if errors.As(err, &he) {
				c.JSON(he.status, map[string]string{"error": he.msg})
			} else {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return
		}
		if created {
			c.JSON(consts.StatusCreated, result)
		} else {
			c.JSON(consts.StatusOK, result)
		}
	}
}

func UpdateHolding(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID := c.Param("pid")
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

		id := c.Param("id")
		var holding models.Holding
		if err := db.Where("portfolio_id = ?", portfolioID).First(&holding, "id = ?", id).Error; err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "Holding not found"})
			return
		}

		var updates map[string]any
		if err := c.BindJSON(&updates); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		allowedFields := map[string]bool{
			"name": true, "symbol": true, "market": true, "price": true,
			"date": true, "lots": true, "value": true,
		}
		safeUpdates := make(map[string]any)
		for k, v := range updates {
			if allowedFields[k] {
				safeUpdates[k] = v
			}
		}

		if _, ok := updates["assetId"]; ok {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "assetId cannot be changed"})
			return
		}

		if _, ok := safeUpdates["value"]; ok && holding.Symbol != "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "cannot directly update value for symbol-based holding"})
			return
		}

		if lotsRaw, ok := safeUpdates["lots"]; ok {
			if lotsBytes, err := json.Marshal(lotsRaw); err == nil {
				var lots []models.HoldingLot
				if json.Unmarshal(lotsBytes, &lots) == nil {
					for i := range lots {
						if lots[i].ID == "" {
							lots[i].ID = uuid.New().String()
						}
						if lots[i].Shares < 0 {
							c.JSON(consts.StatusBadRequest, map[string]string{"error": "lot shares cannot be negative"})
							return
						}
						if lots[i].Cost < 0 {
							c.JSON(consts.StatusBadRequest, map[string]string{"error": "lot cost cannot be negative"})
							return
						}
						if lots[i].Fee < 0 {
							c.JSON(consts.StatusBadRequest, map[string]string{"error": "lot fee cannot be negative"})
							return
						}
						if lots[i].Type != "" && lots[i].Type != "buy" && lots[i].Type != "sell" {
							c.JSON(consts.StatusBadRequest, map[string]string{"error": "lot type must be 'buy' or 'sell'"})
							return
						}
					}
					holding.Lots = lots
					priceBefore := holding.Price
					holding.RecalcFromLots()
					if holding.Symbol != "" && priceBefore > 0 {
						holding.Price = priceBefore
						holding.Value = holding.Shares * holding.Price
					}
					if err := db.Save(&holding).Error; err != nil {
						c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
						return
					}
					c.JSON(consts.StatusOK, holding)
					return
				}
			}
		}

		if newValue, ok := safeUpdates["value"]; ok && holding.Symbol == "" {
			newVal, err := strconv.ParseFloat(fmt.Sprint(newValue), 64)
			if err != nil {
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid value"})
				return
			}
			oldVal := holding.Value
			if math.Abs(newVal-oldVal) > 1e-9 && len(holding.Lots) > 0 {
				diff := newVal - oldVal
				lastBuyIdx := -1
				for i := len(holding.Lots) - 1; i >= 0; i-- {
					if holding.Lots[i].Type != "sell" {
						lastBuyIdx = i
						break
					}
				}
				if lastBuyIdx < 0 {
					c.JSON(consts.StatusBadRequest, map[string]string{"error": "没有买入记录，无法更新价值"})
					return
				}
				err := db.Transaction(func(tx *gorm.DB) error {
					holding.Lots[lastBuyIdx].ValueAdded += diff
					holding.RecalcFromLots()
					return tx.Save(&holding).Error
				})
				if err != nil {
					c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}
				c.JSON(consts.StatusOK, holding)
				return
			}
		}

		if len(safeUpdates) == 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "no valid fields to update"})
			return
		}

		if err := db.Model(&holding).Updates(safeUpdates).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if err := db.Where("portfolio_id = ?", portfolioID).First(&holding, "id = ?", id).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(consts.StatusOK, holding)
	}
}

func DeleteHolding(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID := c.Param("pid")
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

		id := c.Param("id")

		err = db.Transaction(func(tx *gorm.DB) error {
			var holding models.Holding
			if err := tx.Where("portfolio_id = ?", portfolioID).First(&holding, "id = ?", id).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return &httpError{status: consts.StatusNotFound, msg: "Holding not found"}
				}
				return err
			}

			refundAmount := holding.Cost + holding.BuyFees()
			if refundAmount > 0 {
				currency := holding.Currency
				if currency == "" {
					currency = "CNY"
				}
				if err := addAvailableFund(tx, user.UserID, portfolioID, currency, refundAmount); err != nil {
					return err
				}
				if err := tx.Create(&models.FundTransaction{
					ID:          uuid.New().String(),
					UserID:      user.UserID,
					PortfolioID: portfolioID,
					Type:        "delete",
					Amount:      refundAmount,
					Currency:    currency,
					HoldingID:   holding.ID,
					CreatedAt:   time.Now().UnixMilli(),
				}).Error; err != nil {
					return err
				}
			}

			if err := tx.Delete(&holding).Error; err != nil {
				return err
			}

			return nil
		})
		if err != nil {
			var he *httpError
			if errors.As(err, &he) {
				c.JSON(he.status, map[string]string{"error": he.msg})
			} else {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return
		}
		c.JSON(consts.StatusOK, map[string]bool{"success": true})
	}
}

func userOwnsPortfolio(db *gorm.DB, userID, portfolioID string) (bool, error) {
	var count int64
	if err := db.Model(&models.Portfolio{}).Where("id = ? AND user_id = ?", portfolioID, userID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
