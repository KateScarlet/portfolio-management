package handlers

import (
	"context"
	"portfolio-management/marketsource"
	"portfolio-management/middleware"
	"portfolio-management/models"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type PortfolioSummaryItem struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	Total     decimal.Decimal       `json:"total"`
	Principal decimal.Decimal       `json:"principal"`
	Assets    models.AssetMapColumn `json:"assets"`
}

type SummaryResponse struct {
	Total      decimal.Decimal        `json:"total"`
	Principal  decimal.Decimal        `json:"principal"`
	Assets     models.AssetMapColumn  `json:"assets"`
	Portfolios []PortfolioSummaryItem `json:"portfolios"`
}

func GetSummary(db *gorm.DB, router *marketsource.Router) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		portfolios, err := gorm.G[models.Portfolio](db).Where("user_id = ?", user.UserID).Find(ctx)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		summary := SummaryResponse{
			Assets:     models.AssetMapColumn{"stocks": decimal.Zero, "bonds": decimal.Zero, "cash": decimal.Zero, "commodities": decimal.Zero},
			Portfolios: make([]PortfolioSummaryItem, 0, len(portfolios)),
		}

		displayCurrency := c.Query("currency")
		if displayCurrency == "" {
			displayCurrency = "CNY"
		}

		for _, p := range portfolios {
			holdings, err := gorm.G[models.Holding](db).Where("portfolio_id = ?", p.ID).Find(ctx)
			if err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}

			// Load lots for currency conversion
			holdingIDs := make([]uuid.UUID, len(holdings))
			for i, h := range holdings {
				holdingIDs[i] = h.ID
			}
			lotsMap, err := models.LoadLotsByHoldingIDs(db, holdingIDs)
			if err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}

			if err := convertHoldingsCurrency(holdings, lotsMap, displayCurrency, router, user.UserID); err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}

			assets := models.AssetMapColumn{"stocks": decimal.Zero, "bonds": decimal.Zero, "cash": decimal.Zero, "commodities": decimal.Zero}
			total := decimal.Zero
			for i := range holdings {
				assets[holdings[i].AssetId] = assets[holdings[i].AssetId].Add(holdings[i].Value)
				total = total.Add(holdings[i].Value)
			}

			fundsTotal := decimal.Zero
			funds, err := gorm.G[models.AvailableFund](db).Where("user_id = ? AND portfolio_id = ?", user.UserID, p.ID).Find(ctx)
			if err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			for _, f := range funds {
				amt := f.Amount
				if f.Currency != displayCurrency && f.Currency != "" {
					pair := f.Currency + displayCurrency
					rate, err := router.ExchangeRate(user.UserID, pair)
					if err != nil {
						c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
						return
					}
					amt = amt.Mul(rate)
				}
				fundsTotal = fundsTotal.Add(amt)
			}

			principal, err := CalcPrincipal(db, p.ID, displayCurrency, router)
			if err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}

			portfolioTotal := total.Add(fundsTotal)
			summary.Total = summary.Total.Add(portfolioTotal)
			summary.Principal = summary.Principal.Add(principal)
			for k, v := range assets {
				summary.Assets[k] = summary.Assets[k].Add(v)
			}

			summary.Portfolios = append(summary.Portfolios, PortfolioSummaryItem{
				ID:        p.ID.String(),
				Name:      p.Name,
				Total:     portfolioTotal,
				Principal: principal,
				Assets:    assets,
			})
		}

		c.JSON(consts.StatusOK, summary)
	}
}
