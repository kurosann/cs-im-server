package http

import (
	"net/http"

	gatewaydto "cs-im-server/internal/module/gateway/dto"
	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	relaydto "cs-im-server/internal/module/relay/dto"
	"cs-im-server/pkg/httpapi"
	"cs-im-server/pkg/seqquery"
	"cs-im-server/pkg/sse"
	commonmodule "vane-common/module"
)

type RelayMessageAPI struct {
	Messages gatewaypkg.MessageEgress
	Realtime gatewaypkg.RealtimeSubscriber
}

func NewRelayMessageAPI(messages gatewaypkg.MessageEgress, realtime gatewaypkg.RealtimeSubscriber) *RelayMessageAPI {
	return &RelayMessageAPI{Messages: messages, Realtime: realtime}
}

func canUseRelay(principal iampkg.Principal) bool {
	return principal.HasScope(iampkg.ScopeRelayAccess) &&
		(principal.HasKind(iampkg.PrincipalKindRelay) || principal.HasKind(iampkg.PrincipalKindHuman))
}

func (a *RelayMessageAPI) relayID(ctx httpapi.Context) (string, bool) {
	principal, ok := iampkg.PrincipalFromContext(ctx.Context())
	if !ok || !canUseRelay(principal) {
		writeServiceError(ctx, iampkg.ErrPrincipalMissing)
		return "", false
	}
	return principal.AccountKey, true
}

func relayMessageFromGateway(value gatewaydto.Message) relaydto.RelayMessage {
	page := relaydto.RelayMessage{
		MessageKey:      value.MessageKey,
		Seq:             value.Seq,
		ConversationKey: value.ConversationKey,
		CustomerKey:     value.CustomerKey,
		Sender: relaydto.RelayMessageSender{
			Role: string(value.Sender.Role),
			ID:   value.Sender.ID,
		},
		Type:       string(value.Type),
		Text:       value.Text,
		Status:     string(value.Status),
		ReceivedAt: value.ReceivedAt,
		CreatedAt:  value.CreatedAt,
		UpdatedAt:  value.UpdatedAt,
	}
	if value.Source != nil {
		page.Source = &relaydto.RelayMessageSource{
			ExternalID: value.Source.ExternalID,
			SentAt:     value.Source.SentAt,
		}
	}
	if value.Content != nil {
		page.Content = &relaydto.RelayMessageMedia{
			URL:             value.Content.URL,
			MimeType:        value.Content.MimeType,
			FileName:        value.Content.FileName,
			Size:            value.Content.Size,
			Width:           value.Content.Width,
			Height:          value.Content.Height,
			DurationSeconds: value.Content.DurationSeconds,
			ThumbnailURL:    value.Content.ThumbnailURL,
			Checksum:        value.Content.Checksum,
		}
	}
	return page
}

func relayMediaToGateway(value *relaydto.RelayMessageMedia) *gatewaydto.MessageMedia {
	if value == nil {
		return nil
	}
	return &gatewaydto.MessageMedia{
		URL:             value.URL,
		MimeType:        value.MimeType,
		FileName:        value.FileName,
		Size:            value.Size,
		Width:           value.Width,
		Height:          value.Height,
		DurationSeconds: value.DurationSeconds,
		ThumbnailURL:    value.ThumbnailURL,
		Checksum:        value.Checksum,
	}
}

func relayHistoryFromGateway(value gatewaydto.MessageHistoryPage) relaydto.RelayMessageHistoryPage {
	items := make([]relaydto.RelayMessage, 0, len(value.Items))
	for _, item := range value.Items {
		items = append(items, relayMessageFromGateway(item))
	}
	return relaydto.RelayMessageHistoryPage{
		Items:         items,
		NextAfterSeq:  value.NextAfterSeq,
		NextBeforeSeq: value.NextBeforeSeq,
		HasMore:       value.HasMore,
	}
}

func (a *RelayMessageAPI) Send(ctx httpapi.Context) {
	relayID, ok := a.relayID(ctx)
	if !ok {
		return
	}
	var req relaydto.CreateRelayMessageRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	message, err := a.Messages.CreateRelayMessage(ctx.Context(), relayID, gatewaydto.CreateMessageRequest{
		ConversationKey: req.ConversationKey,
		Type:            gatewaydto.MessageType(req.Type),
		Text:            req.Text,
		Content:         relayMediaToGateway(req.Content),
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", relayMessageFromGateway(message))
}

func (a *RelayMessageAPI) Stream(ctx httpapi.Context) {
	if _, ok := a.relayID(ctx); !ok {
		return
	}
	events, closeFn, err := a.Realtime.Subscribe(ctx.Context())
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	defer closeFn()
	sse.WriteHeaders(ctx)
	for {
		select {
		case <-ctx.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if !a.Messages.RelayEventAllowed(event) {
				continue
			}
			switch value := event.Data.(type) {
			case gatewaydto.Message:
				event.Data = relayMessageFromGateway(value)
			case []gatewaydto.Message:
				items := make([]relaydto.RelayMessage, 0, len(value))
				for _, message := range value {
					items = append(items, relayMessageFromGateway(message))
				}
				event.Data = items
			}
			if err := sse.WriteEvent(ctx, event); err != nil {
				return
			}
		}
	}
}

func (a *RelayMessageAPI) History(ctx httpapi.Context) {
	if _, ok := a.relayID(ctx); !ok {
		return
	}
	filter, err := seqquery.FromHTTP(ctx)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	page, err := a.Messages.RelayHistory(ctx.Context(), ctx.Param("conversationId"), filter)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", relayHistoryFromGateway(page))
}

func (a *RelayMessageAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("")
	routes.Post("/messages", a.Send)
	routes.Get("/conversations/:conversationId/messages", a.History)
	routes.Get("/realtime", a.Stream)
	return routes
}
