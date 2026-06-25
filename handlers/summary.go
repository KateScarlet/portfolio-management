package handlers

import (
	"context"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"portfolio-management/yahoo"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"gorm.io/gorm"
)

type PortfolioSummaryItem struct {
	ID        string                `json:"id"`
	Name      string                `json:"name"`
	Total     float64               `json:"total"`
	Principal float64               `json:"principal"`
	Assets    models.AssetMapColumn `json:"assets"`
}

type SummaryResponse struct {
	Total      float64                `json:"total"`
	Principal  float64                `json:"principal"`
	Assets     models.AssetMapColumn  `json:"assets"`
	Portfolios []PortfolioSummaryItem `json:"portfolios"`
}

func GetSummary(db *gorm.DB) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		var portfolios []models.Portfolio
		if err := db.Where("user_id = ?", user.UserID).Find(&portfolios).Error; err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		summary := SummaryResponse{
			Assets:     models.AssetMapColumn{"stocks": 0, "bonds": 0, "cash": 0, "commodities": 0},
			Portfolios: make([]PortfolioSummaryItem, 0, len(portfolios)),
		}

		displayCurrency := c.Query("currency")
		if displayCurrency == "" {
			displayCurrency = "CNY"
		}

		for _, p := range portfolios {
			var holdings []models.Holding
			if err := db.Where("portfolio_id = ?", p.ID).Find(&holdings).Error; err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}

			if err := convertHoldingsCurrency(holdings, displayCurrency); err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}

			assets := models.AssetMapColumn{"stocks": 0, "bonds": 0, "cash": 0, "commodities": 0}
			var total float64
			for i := range holdings {
				assets[holdings[i].AssetId] += holdings[i].Value
				total += holdings[i].Value
			}

			var fundsTotal float64
			var funds []models.AvailableFund
			if err := db.Where("user_id = ? AND portfolio_id = ?", user.UserID, p.ID).Find(&funds).Error; err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			for _, f := range funds {
				amt := f.Amount
				if f.Currency != displayCurrency && f.Currency != "" {
					pair := f.Currency + displayCurrency
					rate, err := yahoo.FetchExchangeRate(pair)
					if err != nil {
						c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
						return
					}
					amt *= rate
				}
				fundsTotal += amt
			}

			principal, err := CalcPrincipal(db, p.ID, displayCurrency)
			if err != nil {
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}

			portfolioTotal := total + fundsTotal
			summary.Total += portfolioTotal
			summary.Principal += principal
			for k, v := range assets {
				summary.Assets[k] += v
			}

			summary.Portfolios = append(summary.Portfolios, PortfolioSummaryItem{
				ID:        p.ID,
				Name:      p.Name,
				Total:     portfolioTotal,
				Principal: principal,
				Assets:    assets,
			})
		}

		c.JSON(consts.StatusOK, summary)
	}
}
