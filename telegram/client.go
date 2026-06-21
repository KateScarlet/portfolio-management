package telegram

import (
	"fmt"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Client struct {
	bot    *tgbotapi.BotAPI
	chatID int64
}

func NewClient(token, chatID string) (*Client, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	var cid int64
	if _, scanErr := fmt.Sscanf(chatID, "%d", &cid); scanErr != nil {
		return nil, fmt.Errorf("invalid chat ID %q: %w", chatID, scanErr)
	}

	return &Client{bot: bot, chatID: cid}, nil
}

func NewClientFromSettings(token, chatID string) (*Client, error) {
	if token == "" || chatID == "" {
		return nil, fmt.Errorf("telegram bot token and chat ID are required")
	}
	return NewClient(token, chatID)
}

func (c *Client) BotName() string {
	return c.bot.Self.UserName
}

func (c *Client) SendMessage(text string) error {
	msg := tgbotapi.NewMessage(c.chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	_, err := c.bot.Send(msg)
	if err != nil {
		slog.Error("failed to send telegram message", "error", err)
		return err
	}
	return nil
}

func (c *Client) SendPhoto(photoBytes []byte, caption string) error {
	photo := tgbotapi.NewPhoto(c.chatID, tgbotapi.FileBytes{
		Name:  "chart",
		Bytes: photoBytes,
	})
	if caption != "" {
		photo.Caption = caption
		photo.ParseMode = tgbotapi.ModeHTML
	}
	_, err := c.bot.Send(photo)
	if err != nil {
		slog.Error("failed to send telegram photo", "error", err)
		return err
	}
	return nil
}

func (c *Client) SendPhotoAndMessage(photoBytes []byte, caption, text string) error {
	if err := c.SendPhoto(photoBytes, caption); err != nil {
		return err
	}
	if text != "" {
		return c.SendMessage(text)
	}
	return nil
}

func TestConnection(token, chatID string) (string, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return "", fmt.Errorf("invalid bot token: %w", err)
	}

	var cid int64
	if _, scanErr := fmt.Sscanf(chatID, "%d", &cid); scanErr != nil {
		return "", fmt.Errorf("invalid chat ID: %w", scanErr)
	}

	msg := tgbotapi.NewMessage(cid, "✅ Telegram 通知已连接成功！")
	_, err = bot.Send(msg)
	if err != nil {
		return "", fmt.Errorf("failed to send test message: %w", err)
	}

	return bot.Self.UserName, nil
}
