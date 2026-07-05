package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"portfolio-management/marketsource"
	"portfolio-management/middleware"
	"portfolio-management/models"
	"portfolio-management/scheduler"
	"strconv"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/sse"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func ListSettings(db *gorm.DB) app.HandlerFunc {
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

		settings, err := gorm.G[models.Setting](db).Where("portfolio_id = ?", portfolioID).Find(ctx)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		result := make(map[string]string)
		for _, s := range settings {
			result[s.Key] = s.Value
		}
		c.JSON(consts.StatusOK, result)
	}
}

func UpdateSetting(db *gorm.DB, s *scheduler.PriceScheduler) app.HandlerFunc {
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

		key := c.Param("key")
		var body struct {
			Value string `json:"value"`
		}
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if body.Value == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "value 不能为空"})
			return
		}

		if key == "syncInterval" {
			mins, err := strconv.Atoi(body.Value)
			if err != nil {
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "syncInterval 必须为有效整数"})
				return
			}
			if mins < 0 || mins > 10080 {
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "syncInterval 必须在 0 到 10080 分钟之间（7天）"})
				return
			}
		}

		err = upsertSetting(db, ctx, key, body.Value, user.UserID, portfolioID)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if key == "syncInterval" {
			s.UpdateSchedule(user.UserID, portfolioID)
		}

		c.JSON(consts.StatusOK, map[string]string{"key": key, "value": body.Value})
	}
}

func upsertSetting(db *gorm.DB, ctx context.Context, key, value string, userID, portfolioID uuid.UUID) error {
	_, err := gorm.G[models.Setting](db).Where("key = ? AND user_id = ? AND portfolio_id = ?", key, userID, portfolioID).First(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return gorm.G[models.Setting](db).Create(ctx, &models.Setting{Key: key, Value: value, UserID: userID, PortfolioID: portfolioID})
	}
	if err != nil {
		return err
	}
	_, err = gorm.G[models.Setting](db).Where("key = ? AND user_id = ? AND portfolio_id = ?", key, userID, portfolioID).Update(ctx, "value", value)
	return err
}

func GetAvailableFunds(db *gorm.DB) app.HandlerFunc {
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

		funds, err := gorm.G[models.AvailableFund](db).Where("user_id = ? AND portfolio_id = ?", user.UserID, portfolioID).Find(ctx)
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		result := make([]map[string]any, 0, len(funds))
		for _, f := range funds {
			if !f.Amount.IsZero() {
				result = append(result, map[string]any{
					"currency": f.Currency,
					"amount":   f.Amount,
				})
			}
		}
		if result == nil {
			result = []map[string]any{}
		}
		c.JSON(consts.StatusOK, result)
	}
}

func BatchUpdateSettings(db *gorm.DB, s *scheduler.PriceScheduler) app.HandlerFunc {
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

		var body map[string]string
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if len(body) == 0 {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "未提供设置项"})
			return
		}

		for key, value := range body {
			if key == "syncInterval" || key == "driftThreshold" {
				if value == "" {
					c.JSON(consts.StatusBadRequest, map[string]string{"error": key + " 的 value 不能为空"})
					return
				}
			}
		}

		if syncVal, ok := body["syncInterval"]; ok {
			mins, err := strconv.Atoi(syncVal)
			if err != nil {
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "syncInterval 必须为有效整数"})
				return
			}
			if mins < 0 || mins > 10080 {
				c.JSON(consts.StatusBadRequest, map[string]string{"error": "syncInterval 必须在 0 到 10080 分钟之间（7天）"})
				return
			}
		}

		err = db.Transaction(func(tx *gorm.DB) error {
			for key, value := range body {
				if err := upsertSetting(tx, ctx, key, value, user.UserID, portfolioID); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		if _, ok := body["syncInterval"]; ok {
			s.UpdateSchedule(user.UserID, portfolioID)
		}

		c.JSON(consts.StatusOK, body)
	}
}

func GetMarketSources(router *marketsource.Router) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		c.JSON(consts.StatusOK, map[string]any{
			"available":   router.AvailableSources(),
			"config":      router.GetUserConfig(user.UserID),
			"sourceNames": router.SourceNames(),
		})
	}
}

func UpdateMarketSources(router *marketsource.Router) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		var body map[string][]string
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		slog.Info("updating market sources", "userId", user.UserID, "body", body)

		if err := router.UpdateUserConfig(user.UserID, body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		c.JSON(consts.StatusOK, map[string]string{"status": "ok"})
	}
}

// Test symbol for each market category (canonical format)
var testSymbols = map[string]string{
	"US":             "AAPL.US",
	"CN":             "600519.SH",
	"HK":             "0700.HK",
	"FUND":           "022485",
	"COMMODITY_CN":   "au9999",
	"COMMODITY_INTL": "GC.INTL",
	"CRYPTO":         "BTC",
}

type sourceTestJob struct {
	Source string
	Market string
}

type sourceTestResult struct {
	Key    string         `json:"key"`
	Source string         `json:"source"`
	Market string         `json:"market"`
	Result map[string]any `json:"result"`
}

const maxConcurrentTest = 5

func TestMarketSources(router *marketsource.Router) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		user := middleware.GetUser(c)
		if user == nil {
			c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
			return
		}

		var body map[string][]string
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		jobs := make([]sourceTestJob, 0)
		for market, sources := range body {
			for _, source := range sources {
				jobs = append(jobs, sourceTestJob{Source: source, Market: market})
			}
		}

		if len(jobs) == 0 {
			c.JSON(consts.StatusOK, map[string]any{"results": map[string]any{}})
			return
		}

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")

		w := sse.NewWriter(c)

		resultsCh := make(chan sourceTestResult, len(jobs))
		sem := make(chan struct{}, maxConcurrentTest)
		var wg sync.WaitGroup

		for _, job := range jobs {
			wg.Add(1)
			sem <- struct{}{}
			go func(j sourceTestJob) {
				defer func() {
					<-sem
					wg.Done()
				}()
				result := testSingleSource(router, j.Market, j.Source)
				resultsCh <- sourceTestResult{
					Key:    j.Source + "-" + j.Market,
					Source: j.Source,
					Market: j.Market,
					Result: result,
				}
			}(job)
		}

		go func() {
			wg.Wait()
			close(resultsCh)
		}()

		successCount := 0
		failCount := 0
		for r := range resultsCh {
			data, _ := json.Marshal(r)
			_ = w.WriteEvent("", "source-test-result", data)
			if r.Result["success"] == true {
				successCount++
			} else {
				failCount++
			}
		}

		summary := map[string]any{
			"total":   len(jobs),
			"success": successCount,
			"failed":  failCount,
		}
		summaryData, _ := json.Marshal(summary)
		_ = w.WriteEvent("", "source-test-complete", summaryData)
	}
}

func testSingleSource(router *marketsource.Router, market, source string) map[string]any {
	start := time.Now()

	if market == "EXCHANGE" {
		pair := "USDCNY"
		rate, err := router.TestExchangeSource(source, pair)
		latency := time.Since(start).Milliseconds()
		if err != nil {
			return map[string]any{
				"success": false,
				"error":   err.Error(),
				"latency": latency,
				"symbol":  pair,
			}
		}
		return map[string]any{
			"success": true,
			"rate":    rate.String(),
			"latency": latency,
			"symbol":  pair,
		}
	}

	symbol, ok := testSymbols[market]
	if !ok {
		return map[string]any{
			"success": false,
			"error":   "不支持的市场: " + market,
			"latency": time.Since(start).Milliseconds(),
		}
	}
	normalizedSymbol := marketsource.NormalizeSymbol(symbol, market)
	quote, err := router.TestSource(source, market, normalizedSymbol)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return map[string]any{
			"success": false,
			"error":   err.Error(),
			"latency": latency,
			"symbol":  normalizedSymbol,
		}
	}
	return map[string]any{
		"success":  true,
		"name":     quote.Name,
		"price":    quote.Price.String(),
		"currency": quote.Currency,
		"latency":  latency,
		"symbol":   normalizedSymbol,
	}
}
