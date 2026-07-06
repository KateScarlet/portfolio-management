package bark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const defaultServerURL = "https://api.day.app"

type Client struct {
	deviceKey string
	serverURL string
	http      *http.Client
}

type pushRequest struct {
	DeviceKey string `json:"device_key"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Group     string `json:"group,omitempty"`
}

type pushResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewClient(deviceKey, serverURL string) (*Client, error) {
	if deviceKey == "" {
		return nil, fmt.Errorf("device key is required")
	}
	if serverURL == "" {
		serverURL = defaultServerURL
	}
	serverURL = strings.TrimRight(serverURL, "/")

	return &Client{
		deviceKey: deviceKey,
		serverURL: serverURL,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (c *Client) SendNotification(title, body, group string) error {
	req := pushRequest{
		DeviceKey: c.deviceKey,
		Title:     title,
		Body:      body,
		Group:     group,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.http.Post(c.serverURL+"/push", "application/json", bytes.NewReader(data))
	if err != nil {
		slog.Error("failed to send bark notification", "error", err)
		return fmt.Errorf("failed to send bark notification: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var result pushResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.Code != 200 {
		return fmt.Errorf("bark API error: %s", result.Message)
	}

	return nil
}

func TestConnection(deviceKey, serverURL string) error {
	c, err := NewClient(deviceKey, serverURL)
	if err != nil {
		return err
	}
	return c.SendNotification("连接测试", "Bark 通知已连接成功！", "portfolio")
}
