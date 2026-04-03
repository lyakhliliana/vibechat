package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	mw "vibechat/internal/delivery/http/middleware"
	"vibechat/internal/delivery/http/response"
	"vibechat/internal/domain"
	"vibechat/internal/usecase/message"
)

type MessageHandler struct {
	uc message.UseCase
}

func NewMessageHandler(uc message.UseCase) *MessageHandler {
	return &MessageHandler{uc: uc}
}

func (h *MessageHandler) Send(w http.ResponseWriter, r *http.Request) {
	chatID, err := parseUUID(r, "chat_id")
	if err != nil {
		response.BadRequest(w)
		return
	}

	var body struct {
		Content   string             `json:"content"`
		Type      domain.MessageType `json:"type"`
		ReplyToID *uuid.UUID         `json:"reply_to_id"`
	}
	if err = json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.BadRequest(w)
		return
	}
	if body.Type == "" {
		body.Type = domain.MessageTypeText
	}
	// Treat zero UUID as no reply (Swagger UI sends "00000000-..." when field is left blank)
	if body.ReplyToID != nil && *body.ReplyToID == uuid.Nil {
		body.ReplyToID = nil
	}

	msg, err := h.uc.Send(r.Context(), mw.UserID(r), message.SendInput{
		ChatID:    chatID,
		Content:   body.Content,
		Type:      body.Type,
		ReplyToID: body.ReplyToID,
	})
	if err != nil {
		response.Err(w, r, err)
		return
	}
	response.Created(w, msg)
}

// GetHistory returns paginated messages older than the optional cursor.
// Query params: limit, cursor_id, cursor_ts (RFC3339Nano).
func (h *MessageHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	chatID, err := parseUUID(r, "chat_id")
	if err != nil {
		response.BadRequest(w)
		return
	}

	q := r.URL.Query()
	limit := queryInt(q.Get("limit"), 30)
	if limit < 1 {
		limit = 30
	} else if limit > 100 {
		limit = 100
	}
	in := message.ListInput{
		ChatID: chatID,
		Limit:  limit,
	}

	if cursorID := q.Get("cursor_id"); cursorID != "" {
		id, e1 := uuid.Parse(cursorID)
		ts, e2 := time.Parse(time.RFC3339Nano, q.Get("cursor_ts"))
		if e1 != nil || e2 != nil {
			response.BadRequest(w)
			return
		}
		in.Cursor = &domain.MessageCursor{ID: id, CreatedAt: ts}
	}

	page, err := h.uc.GetHistory(r.Context(), mw.UserID(r), in)
	if err != nil {
		response.Err(w, r, err)
		return
	}
	response.OK(w, page)
}

func (h *MessageHandler) Edit(w http.ResponseWriter, r *http.Request) {
	msgID, err := parseUUID(r, "msg_id")
	if err != nil {
		response.BadRequest(w)
		return
	}

	var in message.EditInput
	if err = json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.BadRequest(w)
		return
	}

	msg, err := h.uc.Edit(r.Context(), msgID, mw.UserID(r), in)
	if err != nil {
		response.Err(w, r, err)
		return
	}
	response.OK(w, msg)
}

func (h *MessageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	msgID, err := parseUUID(r, "msg_id")
	if err != nil {
		response.BadRequest(w)
		return
	}

	if err = h.uc.Delete(r.Context(), msgID, mw.UserID(r)); err != nil {
		response.Err(w, r, err)
		return
	}
	response.NoContent(w)
}

func (h *MessageHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	chatID, err := parseUUID(r, "chat_id")
	if err != nil {
		response.BadRequest(w)
		return
	}

	if err = h.uc.MarkRead(r.Context(), chatID, mw.UserID(r)); err != nil {
		response.Err(w, r, err)
		return
	}
	response.NoContent(w)
}

func (h *MessageHandler) AddReaction(w http.ResponseWriter, r *http.Request) {
	msgID, err := parseUUID(r, "msg_id")
	if err != nil {
		response.BadRequest(w)
		return
	}

	var in message.ReactInput
	if err = json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.BadRequest(w)
		return
	}

	if err = h.uc.AddReaction(r.Context(), msgID, mw.UserID(r), in); err != nil {
		response.Err(w, r, err)
		return
	}
	response.NoContent(w)
}

// RemoveReaction removes an emoji reaction. Pass the emoji as ?emoji=👍
func (h *MessageHandler) RemoveReaction(w http.ResponseWriter, r *http.Request) {
	msgID, err := parseUUID(r, "msg_id")
	if err != nil {
		response.BadRequest(w)
		return
	}

	emoji := r.URL.Query().Get("emoji")
	if emoji == "" {
		response.BadRequest(w)
		return
	}

	if err = h.uc.RemoveReaction(r.Context(), msgID, mw.UserID(r), message.ReactInput{Emoji: emoji}); err != nil {
		response.Err(w, r, err)
		return
	}
	response.NoContent(w)
}
