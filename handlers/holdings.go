package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"portfolio-management/internal/marketsource"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"sort"
	"time"

	"log/slog"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type httpError struct {
	status int
	msg    string
}

func (e *httpError) Error() string { return e.msg }

func convertHoldingsCurrency(holdings []models.Holding, lotsMap map[uuid.UUID][]models.HoldingLot, targetCurrency string, router *marketsource.Router, userID uuid.UUID) error {
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
		h.Value = h.Value.Mul(rate)
		h.Cost = h.Cost.Mul(rate)
		h.CostPrice = h.CostPrice.Mul(rate)
		lots := lotsMap[h.ID]
		for j := range lots {
			lots[j].Fee = lots[j].Fee.Mul(rate)
			lots[j].Cost = lots[j].Cost.Mul(rate)
			lots[j].CostPrice = lots[j].CostPrice.Mul(rate)
			lots[j].ValueAdded = lots[j].ValueAdded.Mul(rate)
		}
		h.Currency = targetCurrency
	}
	return nil
}

// MergedHoldingAccount is one account's position within a merged holding.
type MergedHoldingAccount struct {
	HoldingID   string              `json:"holdingId"`
	AccountID   string              `json:"accountId"`
	AccountName string              `json:"accountName"`
	Shares      decimal.Decimal     `json:"shares"`
	Cost        decimal.Decimal     `json:"cost"`
	Value       decimal.Decimal     `json:"value"`
	Lots        []models.HoldingLot `json:"lots"`
}

// MergedHolding is a holding merged across accounts by symbol.
type MergedHolding struct {
	models.Holding
	Accounts []MergedHoldingAccount `json:"accounts"`
	Lots     []models.HoldingLot    `json:"lots"`
}

// HoldingResponse is a holding with lots attached for API responses.
type HoldingResponse struct {
	models.Holding
	Lots []models.HoldingLot `json:"lots"`
}

func ListHoldings(db *gorm.DB, router *marketsource.Router) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioIDStr := c.Param("pid")
		portfolioID, err := uuid.Parse(portfolioIDStr)
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的组合ID"})
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

		var holdings []models.Holding
		if err := db.Where("portfolio_id = ?", portfolioID).Order("asset_id, id").Find(&holdings).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		// Load lots for all holdings
		holdingIDs := make([]uuid.UUID, len(holdings))
		for i := range holdings {
			holdingIDs[i] = holdings[i].ID
		}
		lotsMap, err := models.LoadLotsByHoldingIDs(db, holdingIDs)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if displayCurrency := c.Query("currency"); displayCurrency != "" {
			if err := convertHoldingsCurrency(holdings, lotsMap, displayCurrency, router, user.UserID); err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}

		// Merge mode: group by symbol, aggregate across accounts
		if c.Query("merge") == "true" {
			// Load accounts for name lookup
			accounts, err := gorm.G[models.Account](db).Where("user_id = ?", user.UserID).Find(ctx)
			if err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			accountNameMap := make(map[uuid.UUID]string, len(accounts))
			for _, a := range accounts {
				accountNameMap[a.ID] = a.Name
			}

			type mergeKey struct {
				Symbol  string
				Name    string
				AssetId string
			}
			merged := make(map[mergeKey]*MergedHolding)

			for i := range holdings {
				h := &holdings[i]
				key := mergeKey{Symbol: h.Symbol, Name: h.Name, AssetId: h.AssetId}
				mh, exists := merged[key]
				if !exists {
					mh = &MergedHolding{
						Holding: models.Holding{
							ID:          h.ID,
							UserID:      h.UserID,
							PortfolioID: h.PortfolioID,
							AssetId:     h.AssetId,
							Symbol:      h.Symbol,
							Name:        h.Name,
							Market:      h.Market,
							Currency:    h.Currency,
							Price:       h.Price,
							Date:        h.Date,
						},
						Accounts: make([]MergedHoldingAccount, 0),
					}
					merged[key] = mh
				}

				accountName := accountNameMap[h.AccountID]
				mh.Accounts = append(mh.Accounts, MergedHoldingAccount{
					HoldingID:   h.ID.String(),
					AccountID:   h.AccountID.String(),
					AccountName: accountName,
					Shares:      h.Shares,
					Cost:        h.Cost,
					Value:       h.Value,
					Lots:        lotsMap[h.ID],
				})

				mh.Shares = mh.Shares.Add(h.Shares)
				mh.Cost = mh.Cost.Add(h.Cost)
				mh.Value = mh.Value.Add(h.Value)
			}

			// Compute merged CostPrice and append all lots
			result := make([]MergedHolding, 0, len(merged))
			for _, mh := range merged {
				if mh.Shares.IsPositive() {
					mh.CostPrice = mh.Cost.Div(mh.Shares)
				}
				// Collect all lots from all accounts
				var allLots []models.HoldingLot
				for _, acc := range mh.Accounts {
					allLots = append(allLots, acc.Lots...)
				}
				mh.Lots = allLots
				result = append(result, *mh)
			}
			sort.Slice(result, func(i, j int) bool {
				if result[i].AssetId != result[j].AssetId {
					return result[i].AssetId < result[j].AssetId
				}
				return result[i].ID.String() < result[j].ID.String()
			})

			c.JSON(consts.StatusOK, result)
			return
		}

		// Non-merge mode: attach lots to each holding
		resp := make([]HoldingResponse, len(holdings))
		for i := range holdings {
			resp[i] = HoldingResponse{Holding: holdings[i], Lots: lotsMap[holdings[i].ID]}
		}
		c.JSON(consts.StatusOK, resp)
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

		portfolioIDStr := c.Param("pid")
		portfolioID, err := uuid.Parse(portfolioIDStr)
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的组合ID"})
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

		var input CreateHoldingInput
		if err := c.BindJSON(&input); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		if input.Symbol != "" && input.Market != "" {
			input.Symbol = marketsource.NormalizeSymbol(input.Symbol, input.Market)
		}

		if input.AssetId == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "assetId 不能为空"})
			return
		}
		validAssets := map[string]bool{"stocks": true, "bonds": true, "cash": true, "commodities": true}
		if !validAssets[input.AssetId] {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的 assetId"})
			return
		}
		if input.Shares.IsNegative() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "股数不能为负数"})
			return
		}
		if input.Cost.IsNegative() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "成本不能为负数"})
			return
		}
		if input.CostPrice.IsNegative() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "成本价不能为负数"})
			return
		}
		if input.Fee.IsNegative() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "手续费不能为负数"})
			return
		}

		// 如果没有指定账户，使用默认账户
		if input.AccountID == uuid.Nil {
			var defaultAccount models.Account
			if err := db.Where("user_id = ? AND is_default = ?", user.UserID, true).First(&defaultAccount).Error; err == nil {
				input.AccountID = defaultAccount.ID
			}
		}

		isRegisterOnly := input.Shares.IsZero() && input.Cost.IsZero()

		var created bool
		var result models.Holding
		var resultLots []models.HoldingLot
		err = db.Transaction(func(tx *gorm.DB) error {
			var existing models.Holding
			var res *gorm.DB
			if input.Symbol != "" {
				res = tx.Where("portfolio_id = ? AND symbol = ? AND account_id = ? AND symbol != ''", portfolioID, input.Symbol, input.AccountID).First(&existing)
			} else {
				res = tx.Where("portfolio_id = ? AND name = ? AND asset_id = ? AND account_id = ? AND symbol = ''", portfolioID, input.Name, input.AssetId, input.AccountID).First(&existing)
			}

			if res.Error == nil {
				if isRegisterOnly {
					return &httpError{status: consts.StatusBadRequest, msg: "该资产已存在"}
				}
				// Load existing lots, append new lot, replace all
				existingLots, err := models.LoadLots(tx, existing.ID)
				if err != nil {
					return err
				}
				newLot := models.HoldingLot{
					ID:         uuid.New(),
					HoldingID:  existing.ID,
					Date:       input.Date,
					Shares:     input.Shares,
					CostPrice:  input.CostPrice,
					Cost:       input.Cost,
					ValueAdded: input.Value,
					Fee:        input.Fee,
				}
				if newLot.Date.IsZero() {
					newLot.Date = time.Now()
				}
				existingLots = append(existingLots, newLot)
				if existing.Symbol != "" && input.Price.IsPositive() {
					existing.Price = input.Price
				}
				models.RecalcFromLots(&existing, existingLots)
				created = false
				result = existing
				if err := tx.Save(&existing).Error; err != nil {
					return err
				}
				if err := models.ReplaceLots(tx, existing.ID, existingLots); err != nil {
					return err
				}
				resultLots = existingLots
			} else {
				if !errors.Is(res.Error, gorm.ErrRecordNotFound) {
					return res.Error
				}

				input.ID = uuid.New()
				input.UserID = user.UserID
				input.PortfolioID = portfolioID
				created = true
				result = input.Holding
				if err := tx.Create(&input.Holding).Error; err != nil {
					return err
				}
				if !isRegisterOnly {
					newLot := models.HoldingLot{
						ID:         uuid.New(),
						HoldingID:  input.ID,
						Date:       input.Date,
						Shares:     input.Shares,
						CostPrice:  input.CostPrice,
						Cost:       input.Cost,
						ValueAdded: input.Value,
						Fee:        input.Fee,
					}
					if newLot.Date.IsZero() {
						newLot.Date = time.Now()
					}
					if err := models.CreateLot(tx, &newLot); err != nil {
						return err
					}
					resultLots = []models.HoldingLot{newLot}
					models.RecalcFromLots(&result, resultLots)
					if err := tx.Save(&result).Error; err != nil {
						return err
					}
				} else {
					resultLots = nil
				}
			}

			if !isRegisterOnly {
				addedCost := input.Cost.Add(input.Fee)
				if addedCost.IsPositive() {
					holdingCurrency := input.Currency
					if holdingCurrency == "" {
						holdingCurrency = "CNY"
					}

					var af models.AvailableFund
					err := tx.Where("user_id = ? AND portfolio_id = ? AND currency = ?", user.UserID, portfolioID, holdingCurrency).First(&af).Error
					fundsAmount := decimal.Zero
					if err == nil {
						fundsAmount = af.Amount
					} else if !errors.Is(err, gorm.ErrRecordNotFound) {
						return err
					}

					if fundsAmount.LessThan(addedCost) {
						return &httpError{status: consts.StatusBadRequest, msg: fmt.Sprintf("可用资金不足: %s 可用 %s, 需要 %s", holdingCurrency, fundsAmount.StringFixed(2), addedCost.StringFixed(2))}
					}

					newAmount := fundsAmount.Sub(addedCost)
					if err == nil {
						if err := tx.Model(&af).Update("amount", newAmount).Error; err != nil {
							return err
						}
					} else {
						if newAmount.IsPositive() {
							if err := tx.Create(&models.AvailableFund{
								ID:          uuid.New(),
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
						ID:          uuid.New(),
						UserID:      user.UserID,
						PortfolioID: portfolioID,
						Type:        "buy",
						Amount:      addedCost,
						Currency:    holdingCurrency,
						HoldingID:   &result.ID,
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
			c.JSON(consts.StatusCreated, HoldingResponse{Holding: result, Lots: resultLots})
		} else {
			c.JSON(consts.StatusOK, HoldingResponse{Holding: result, Lots: resultLots})
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

		portfolioIDStr := c.Param("pid")
		portfolioID, err := uuid.Parse(portfolioIDStr)
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的组合ID"})
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

		id := c.Param("id")
		var holding models.Holding
		if err := db.Where("portfolio_id = ?", portfolioID).First(&holding, "id = ?", id).Error; err != nil {
			c.JSON(consts.StatusNotFound, map[string]string{"error": "持仓不存在"})
			return
		}

		var updates map[string]any
		if err := c.BindJSON(&updates); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		allowedFields := map[string]bool{
			"name": true, "symbol": true, "market": true, "price": true,
			"date": true, "lots": true, "value": true, "accountId": true,
		}
		safeUpdates := make(map[string]any)
		for k, v := range updates {
			if allowedFields[k] {
				safeUpdates[k] = v
			}
		}

		if _, ok := updates["assetId"]; ok {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "不能修改 assetId"})
			return
		}

		if _, ok := safeUpdates["value"]; ok && holding.Symbol != "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "不能直接更新股票类持仓的价值"})
			return
		}

		if lotsRaw, ok := safeUpdates["lots"]; ok {
			if lotsBytes, err := json.Marshal(lotsRaw); err == nil {
				var lots []models.HoldingLot
				if json.Unmarshal(lotsBytes, &lots) == nil {
					for i := range lots {
						if lots[i].ID == uuid.Nil {
							lots[i].ID = uuid.New()
						}
						lots[i].HoldingID = holding.ID
						if lots[i].Shares.IsNegative() {
							c.JSON(consts.StatusBadRequest, map[string]string{"error": "交易记录股数不能为负数"})
							return
						}
						if lots[i].Cost.IsNegative() {
							c.JSON(consts.StatusBadRequest, map[string]string{"error": "交易记录成本不能为负数"})
							return
						}
						if lots[i].Fee.IsNegative() {
							c.JSON(consts.StatusBadRequest, map[string]string{"error": "交易记录手续费不能为负数"})
							return
						}
						if lots[i].Type != "" && lots[i].Type != "buy" && lots[i].Type != "sell" {
							c.JSON(consts.StatusBadRequest, map[string]string{"error": "交易记录类型必须为 'buy' 或 'sell'"})
							return
						}
					}
					priceBefore := holding.Price
					models.RecalcFromLots(&holding, lots)
					if holding.Symbol != "" && priceBefore.IsPositive() {
						holding.Price = priceBefore
						holding.Value = holding.Shares.Mul(holding.Price)
					}
					if err := db.Transaction(func(tx *gorm.DB) error {
						if err := tx.Save(&holding).Error; err != nil {
							return err
						}
						return models.ReplaceLots(tx, holding.ID, lots)
					}); err != nil {
						c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
						return
					}
					c.JSON(consts.StatusOK, HoldingResponse{Holding: holding, Lots: lots})
					return
				}
			}
		}

		if newValue, ok := safeUpdates["value"]; ok && holding.Symbol == "" {
			var newVal decimal.Decimal
			var parseErr error
			switch v := newValue.(type) {
			case string:
				newVal, parseErr = decimal.NewFromString(v)
			case float64:
				newVal = decimal.NewFromFloat(v)
			case json.Number:
				newVal, parseErr = decimal.NewFromString(v.String())
			default:
				newVal, parseErr = decimal.NewFromString(fmt.Sprint(newValue))
			}
			if parseErr != nil {
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的数值格式"})
				return
			}
			oldVal := holding.Value
			lots, err := models.LoadLots(db, holding.ID)
			if err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			if !newVal.Equal(oldVal) && len(lots) > 0 {
				diff := newVal.Sub(oldVal)
				lastBuyIdx := -1
				for i := len(lots) - 1; i >= 0; i-- {
					if lots[i].Type != "sell" {
						lastBuyIdx = i
						break
					}
				}
				if lastBuyIdx < 0 {
					c.JSON(consts.StatusBadRequest, map[string]string{"error": "没有买入记录，无法更新价值"})
					return
				}
				err := db.Transaction(func(tx *gorm.DB) error {
					lots[lastBuyIdx].ValueAdded = lots[lastBuyIdx].ValueAdded.Add(diff)
					models.RecalcFromLots(&holding, lots)
					if err := tx.Save(&holding).Error; err != nil {
						return err
					}
					return models.ReplaceLots(tx, holding.ID, lots)
				})
				if err != nil {
					c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}
				c.JSON(consts.StatusOK, HoldingResponse{Holding: holding, Lots: lots})
				return
			}
		}

		// Remove "lots" from safeUpdates since it's handled separately
		delete(safeUpdates, "lots")

		if len(safeUpdates) == 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "没有可更新的字段"})
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
		lots, err := models.LoadLots(db, holding.ID)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		c.JSON(consts.StatusOK, HoldingResponse{Holding: holding, Lots: lots})
	}
}

func DeleteHolding(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioIDStr := c.Param("pid")
		portfolioID, err := uuid.Parse(portfolioIDStr)
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的组合ID"})
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

		id := c.Param("id")

		err = db.Transaction(func(tx *gorm.DB) error {
			var holding models.Holding
			if err := tx.Where("portfolio_id = ?", portfolioID).First(&holding, "id = ?", id).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return &httpError{status: consts.StatusNotFound, msg: "Holding not found"}
				}
				return err
			}

			lots, err := models.LoadLots(tx, holding.ID)
			if err != nil {
				return err
			}

			var realizedValue decimal.Decimal
			for i := range lots {
				lot := &lots[i]
				if lot.Type == "sell" {
					realizedValue = realizedValue.Add(lot.ValueAdded).Sub(lot.Fee)
				}
			}
			refundAmount := holding.Cost.Add(models.BuyFees(lots)).Sub(realizedValue)
			if refundAmount.IsPositive() {
				currency := holding.Currency
				if currency == "" {
					currency = "CNY"
				}
				if err := addAvailableFund(tx, user.UserID, portfolioID, currency, refundAmount); err != nil {
					return err
				}
				if err := tx.Create(&models.FundTransaction{
					ID:          uuid.New(),
					UserID:      user.UserID,
					PortfolioID: portfolioID,
					Type:        "delete",
					Amount:      refundAmount,
					Currency:    currency,
					HoldingID:   &holding.ID,
				}).Error; err != nil {
					return err
				}
			}

			var dividends []models.Dividend
			if err := tx.Where("holding_id = ? AND user_id = ?", holding.ID, user.UserID).Find(&dividends).Error; err != nil {
				return err
			}
			for i := range dividends {
				if err := tx.Where("id = ? AND user_id = ?", dividends[i].FundTxID, user.UserID).Delete(&models.FundTransaction{}).Error; err != nil {
					return err
				}
			}
			if err := tx.Where("holding_id = ? AND user_id = ?", holding.ID, user.UserID).Delete(&models.Dividend{}).Error; err != nil {
				return err
			}

			if err := models.DeleteLotsByHoldingID(tx, holding.ID); err != nil {
				return err
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

func UpdateLot(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID, err := uuid.Parse(c.Param("pid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的组合ID"})
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

		holdingID, err := uuid.Parse(c.Param("hid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的持仓ID"})
			return
		}
		lotID, err := uuid.Parse(c.Param("lid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的记录ID"})
			return
		}

		var input struct {
			Date       *time.Time       `json:"date"`
			Shares     *decimal.Decimal `json:"shares"`
			CostPrice  *decimal.Decimal `json:"costPrice"`
			Cost       *decimal.Decimal `json:"cost"`
			ValueAdded *decimal.Decimal `json:"valueAdded"`
			Fee        *decimal.Decimal `json:"fee"`
		}
		if err := c.BindJSON(&input); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		if input.Shares != nil && input.Shares.IsNegative() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "交易记录股数不能为负数"})
			return
		}
		if input.Cost != nil && input.Cost.IsNegative() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "交易记录成本不能为负数"})
			return
		}
		if input.Fee != nil && input.Fee.IsNegative() {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "交易记录手续费不能为负数"})
			return
		}

		updates := make(map[string]any)
		if input.Date != nil {
			updates["date"] = *input.Date
		}
		if input.Shares != nil {
			updates["shares"] = *input.Shares
		}
		if input.CostPrice != nil {
			updates["cost_price"] = *input.CostPrice
		}
		if input.Cost != nil {
			updates["cost"] = *input.Cost
		}
		if input.ValueAdded != nil {
			updates["value_added"] = *input.ValueAdded
		}
		if input.Fee != nil {
			updates["fee"] = *input.Fee
		}

		if len(updates) == 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "没有可更新的字段"})
			return
		}

		var holding models.Holding
		var remainingLots []models.HoldingLot
		err = db.Transaction(func(tx *gorm.DB) error {
			// 持仓必须同时属于当前用户和 URL 中的组合，不能只信任持仓 UUID。
			if err := tx.Where("id = ? AND portfolio_id = ? AND user_id = ?", holdingID, portfolioID, user.UserID).First(&holding).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return &httpError{status: consts.StatusNotFound, msg: "持仓不存在"}
				}
				return err
			}

			var existingLot models.HoldingLot
			if err := tx.Where("id = ? AND holding_id = ?", lotID, holding.ID).First(&existingLot).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return &httpError{status: consts.StatusNotFound, msg: "交易记录不存在"}
				}
				return err
			}
			var dividendLotCount int64
			if err := tx.Model(&models.Dividend{}).Where("holding_lot_id = ?", lotID).Count(&dividendLotCount).Error; err != nil {
				return err
			}
			if dividendLotCount > 0 {
				return &httpError{status: consts.StatusConflict, msg: "分红再投资批次只能通过编辑分红记录修改"}
			}

			// 先记录旧值，计算资金差额
			currency := holding.Currency
			if currency == "" {
				currency = "CNY"
			}

			var fundDelta decimal.Decimal
			if existingLot.Type == "sell" {
				oldRealized := existingLot.ValueAdded.Sub(existingLot.Fee)
				newRealized := oldRealized
				if input.ValueAdded != nil {
					newRealized = input.ValueAdded.Sub(existingLot.Fee)
				}
				if input.Fee != nil {
					if input.ValueAdded != nil {
						newRealized = input.ValueAdded.Sub(*input.Fee)
					} else {
						newRealized = existingLot.ValueAdded.Sub(*input.Fee)
					}
				}
				fundDelta = newRealized.Sub(oldRealized)
			} else {
				oldCost := existingLot.Cost.Add(existingLot.Fee)
				newCost := oldCost
				if input.Cost != nil {
					newCost = input.Cost.Add(existingLot.Fee)
				}
				if input.Fee != nil {
					if input.Cost != nil {
						newCost = input.Cost.Add(*input.Fee)
					} else {
						newCost = existingLot.Cost.Add(*input.Fee)
					}
				}
				fundDelta = oldCost.Sub(newCost) // 正数=退款, 负数=补扣
			}

			// 应用资金差额
			if fundDelta.IsPositive() {
				if err := addAvailableFund(tx, user.UserID, portfolioID, currency, fundDelta); err != nil {
					return err
				}
				if err := tx.Create(&models.FundTransaction{
					ID:          uuid.New(),
					UserID:      user.UserID,
					PortfolioID: portfolioID,
					Type:        "delete",
					Amount:      fundDelta,
					Currency:    currency,
					HoldingID:   &holdingID,
				}).Error; err != nil {
					return err
				}
			} else if fundDelta.IsNegative() {
				abs := fundDelta.Abs()
				if err := deductAvailableFund(tx, user.UserID, portfolioID, currency, abs); err != nil {
					return err
				}
				if err := tx.Create(&models.FundTransaction{
					ID:          uuid.New(),
					UserID:      user.UserID,
					PortfolioID: portfolioID,
					Type:        "delete",
					Amount:      abs,
					Currency:    currency,
					HoldingID:   &holdingID,
				}).Error; err != nil {
					return err
				}
			}

			if err := tx.Model(&models.HoldingLot{}).Where("id = ? AND holding_id = ?", lotID, holding.ID).Updates(updates).Error; err != nil {
				return err
			}
			remainingLots, err = models.LoadLots(tx, holding.ID)
			if err != nil {
				return err
			}
			models.RecalcFromLots(&holding, remainingLots)
			return tx.Save(&holding).Error
		})
		if err != nil {
			if he, ok := errors.AsType[*httpError](err); ok {
				c.JSON(he.status, map[string]string{"error": he.msg})
			} else {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return
		}

		c.JSON(consts.StatusOK, HoldingResponse{Holding: holding, Lots: remainingLots})
	}
}

func DeleteLot(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolioID, err := uuid.Parse(c.Param("pid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的组合ID"})
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

		holdingID, err := uuid.Parse(c.Param("hid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的持仓ID"})
			return
		}
		lotID, err := uuid.Parse(c.Param("lid"))
		if err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "无效的记录ID"})
			return
		}

		var holding models.Holding
		var remainingLots []models.HoldingLot
		err = db.Transaction(func(tx *gorm.DB) error {
			// 持仓必须同时属于当前用户和 URL 中的组合，不能只信任持仓 UUID。
			if err := tx.Where("id = ? AND portfolio_id = ? AND user_id = ?", holdingID, portfolioID, user.UserID).First(&holding).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return &httpError{status: consts.StatusNotFound, msg: "持仓不存在"}
				}
				return err
			}

			var lot models.HoldingLot
			if err := tx.Where("id = ? AND holding_id = ?", lotID, holding.ID).First(&lot).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return &httpError{status: consts.StatusNotFound, msg: "交易记录不存在"}
				}
				return err
			}
			var dividendLotCount int64
			if err := tx.Model(&models.Dividend{}).Where("holding_lot_id = ?", lotID).Count(&dividendLotCount).Error; err != nil {
				return err
			}
			if dividendLotCount > 0 {
				return &httpError{status: consts.StatusConflict, msg: "分红再投资批次只能通过删除分红记录撤销"}
			}

			// 回退资金: 买入退回 cost+fee, 卖出扣回 valueAdded-fee
			currency := holding.Currency
			if currency == "" {
				currency = "CNY"
			}
			var fundDelta decimal.Decimal
			if lot.Type == "sell" {
				fundDelta = lot.ValueAdded.Sub(lot.Fee).Neg()
			} else {
				fundDelta = lot.Cost.Add(lot.Fee)
			}
			if fundDelta.IsPositive() {
				if err := addAvailableFund(tx, user.UserID, portfolioID, currency, fundDelta); err != nil {
					return err
				}
				if err := tx.Create(&models.FundTransaction{
					ID:          uuid.New(),
					UserID:      user.UserID,
					PortfolioID: portfolioID,
					Type:        "delete",
					Amount:      fundDelta,
					Currency:    currency,
					HoldingID:   &holdingID,
				}).Error; err != nil {
					return err
				}
			} else if fundDelta.IsNegative() {
				if err := deductAvailableFund(tx, user.UserID, portfolioID, currency, fundDelta.Abs()); err != nil {
					return err
				}
				if err := tx.Create(&models.FundTransaction{
					ID:          uuid.New(),
					UserID:      user.UserID,
					PortfolioID: portfolioID,
					Type:        "delete",
					Amount:      fundDelta.Abs(),
					Currency:    currency,
					HoldingID:   &holdingID,
				}).Error; err != nil {
					return err
				}
			}

			if err := tx.Where("id = ? AND holding_id = ?", lotID, holding.ID).Delete(&models.HoldingLot{}).Error; err != nil {
				return err
			}
			remainingLots, err = models.LoadLots(tx, holding.ID)
			if err != nil {
				return err
			}
			models.RecalcFromLots(&holding, remainingLots)
			return tx.Save(&holding).Error
		})
		if err != nil {
			if he, ok := errors.AsType[*httpError](err); ok {
				c.JSON(he.status, map[string]string{"error": he.msg})
			} else {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return
		}

		c.JSON(consts.StatusOK, HoldingResponse{Holding: holding, Lots: remainingLots})
	}
}

func userOwnsPortfolio(db *gorm.DB, userID, portfolioID uuid.UUID) (bool, error) {
	var count int64
	if err := db.Model(&models.Portfolio{}).Where("id = ? AND user_id = ?", portfolioID, userID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
