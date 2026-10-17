package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cs-im-server/pkg/httpapi"
)

type Event struct {
	Type string    `json:"type"`
	ID   string    `json:"id,omitempty"`
	Data any       `json:"data,omitempty"`
	At   time.Time `json:"at"`
}

func WriteHeaders(ctx httpapi.Context) {
	ctx.SetHeader("Content-Type", "text/event-stream")
	ctx.SetHeader("Cache-Control", "no-cache")
	ctx.SetHeader("Connection", "keep-alive")
	_ = ctx.SetWriteDeadline(time.Time{})
	ctx.Status(http.StatusOK)
}

func WriteEvent(ctx httpapi.Context, event Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(ctx, "data: %s\n\n", payload); err != nil {
		return err
	}
	return ctx.Flush()
}
