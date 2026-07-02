package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"portfolio-management/middleware"
	"portfolio-management/scheduler"
	"sync"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/adaptor"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

const (
	maxConnsPerUser = 3
	heartbeatInterval = 15 * time.Second
)

type SSEHandler struct {
	eventBus  *scheduler.EventBus
	userConns map[string]int
	connMu    sync.RWMutex
}

func NewSSEHandler(eventBus *scheduler.EventBus) *SSEHandler {
	return &SSEHandler{
		eventBus:  eventBus,
		userConns: make(map[string]int),
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
		c.JSON(http.StatusTooManyRequests, map[string]string{"error": "too many connections"})
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

	w := adaptor.GetCompatResponseWriter(&c.Response)

	flusher, ok := w.(http.Flusher)
	if !ok {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "streaming not supported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch, unsub := h.eventBus.Subscribe(user.UserID)
	defer unsub()

	_, _ = fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
	flusher.Flush()

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				slog.Error("failed to marshal sse event", "error", err)
				continue
			}
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()
		}
	}
}
