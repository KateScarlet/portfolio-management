package handlers

import (
	"context"
	"permanent-portfolio/telegram"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

func TestTelegramConnection() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var body struct {
			BotToken string `json:"botToken"`
			ChatID   string `json:"chatID"`
		}
		if err := c.BindAndValidate(&body); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if body.BotToken == "" || body.ChatID == "" {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": "botToken and chatID are required"})
			return
		}

		botName, err := telegram.TestConnection(body.BotToken, body.ChatID)
		if err != nil {
			c.JSON(consts.StatusOK, map[string]any{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		c.JSON(consts.StatusOK, map[string]any{
			"success": true,
			"botName": botName,
		})
	}
}
