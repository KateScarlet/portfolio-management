package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"portfolio-management/middleware"
	"portfolio-management/scheduler"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/cloudwego/hertz/pkg/protocol/sse"
	"github.com/google/uuid"
)

const (
	maxConnsPerUser   = 10
	heartbeatInterval = 15 * time.Second
)

type SSEHandler struct {
	eventBus  *scheduler.EventBus
	userConns map[uuid.UUID]int
	connMu    sync.RWMutex
}

func NewSSEHandler(eventBus *scheduler.EventBus) *SSEHandler {
	return &SSEHandler{
		eventBus:  eventBus,
		userConns: make(map[uuid.UUID]int),
	}
}

func (h *SSEHandler) Handle(ctx context.Context, c *app.RequestContext) {
	user := middleware.GetUser(c)
	if user == nil {
		c.JSON(consts.StatusUnauthorized, map[string]string{"error": "未登录"})
		return
	}

	h.connMu.Lock()
	if h.userConns[user.UserID] >= maxConnsPerUser {
		h.connMu.Unlock()
		c.JSON(http.StatusTooManyRequests, map[string]string{"error": "连接数过多"})
		return
	}
	h.userConns[user.UserID]++
	h.connMu.Unlock()

	defer func() {
		h.connMu.Lock()
		h.userConns[user.UserID]--
		if h.userConns[user.UserID] <= 0 {
			delete(h.userConns, user.UserID)
		}
		h.connMu.Unlock()
	}()

	w := sse.NewWriter(c)

	ch, unsub := h.eventBus.Subscribe(user.UserID)
	defer unsub()

	if err := w.WriteEvent("", "connected", []byte("{}")); err != nil {
		slog.Debug("sse: client disconnected during initial write", "userID", user.UserID)
		return
	}

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.WriteKeepAlive(); err != nil {
				slog.Debug("sse: client disconnected during heartbeat", "userID", user.UserID)
				return
			}
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				slog.Error("failed to marshal sse event", "error", err)
				continue
			}
			if err := w.WriteEvent("", event.Type, data); err != nil {
				slog.Debug("sse: client disconnected during event write", "userID", user.UserID, "eventType", event.Type)
				return
			}
		}
	}
}
