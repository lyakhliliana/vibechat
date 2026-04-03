package handler

import (
	"encoding/json"
	"net/http"

	mw "vibechat/internal/delivery/http/middleware"
	"vibechat/internal/delivery/http/response"
	"vibechat/internal/usecase/chat"
)

type ChatHandler struct {
	uc chat.UseCase
}

func NewChatHandler(uc chat.UseCase) *ChatHandler {
	return &ChatHandler{uc: uc}
}

func (h *ChatHandler) CreateDirect(w http.ResponseWriter, r *http.Request) {
	var in chat.CreateDirectInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.BadRequest(w)
		return
	}

	ch, created, err := h.uc.CreateDirect(r.Context(), mw.UserID(r), in)
	if err != nil {
		response.Err(w, r, err)
		return
	}
	if created {
		response.Created(w, ch)
	} else {
		response.OK(w, ch)
	}
}

func (h *ChatHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var in chat.CreateGroupInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.BadRequest(w)
		return
	}

	ch, err := h.uc.CreateGroup(r.Context(), mw.UserID(r), in)
	if err != nil {
		response.Err(w, r, err)
		return
	}
	response.Created(w, ch)
}

func (h *ChatHandler) GetUserChats(w http.ResponseWriter, r *http.Request) {
	previews, err := h.uc.GetUserChats(r.Context(), mw.UserID(r))
	if err != nil {
		response.Err(w, r, err)
		return
	}
	response.OK(w, previews)
}

func (h *ChatHandler) GetChat(w http.ResponseWriter, r *http.Request) {
	chatID, err := parseUUID(r, "chat_id")
	if err != nil {
		response.BadRequest(w)
		return
	}

	ch, err := h.uc.GetChat(r.Context(), chatID, mw.UserID(r))
	if err != nil {
		response.Err(w, r, err)
		return
	}
	response.OK(w, ch)
}

func (h *ChatHandler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	chatID, err := parseUUID(r, "chat_id")
	if err != nil {
		response.BadRequest(w)
		return
	}

	var in chat.UpdateGroupInput
	if err = json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.BadRequest(w)
		return
	}

	ch, err := h.uc.UpdateGroup(r.Context(), chatID, mw.UserID(r), in)
	if err != nil {
		response.Err(w, r, err)
		return
	}
	response.OK(w, ch)
}

func (h *ChatHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	chatID, err := parseUUID(r, "chat_id")
	if err != nil {
		response.BadRequest(w)
		return
	}

	members, err := h.uc.GetMembers(r.Context(), chatID, mw.UserID(r))
	if err != nil {
		response.Err(w, r, err)
		return
	}
	response.OK(w, members)
}

func (h *ChatHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	chatID, err := parseUUID(r, "chat_id")
	if err != nil {
		response.BadRequest(w)
		return
	}

	var in chat.AddMemberInput
	if err = json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.BadRequest(w)
		return
	}

	if err = h.uc.AddMember(r.Context(), chatID, mw.UserID(r), in); err != nil {
		response.Err(w, r, err)
		return
	}
	response.NoContent(w)
}

func (h *ChatHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	chatID, err := parseUUID(r, "chat_id")
	if err != nil {
		response.BadRequest(w)
		return
	}
	targetID, err := parseUUID(r, "user_id")
	if err != nil {
		response.BadRequest(w)
		return
	}

	if err = h.uc.RemoveMember(r.Context(), chatID, mw.UserID(r), targetID); err != nil {
		response.Err(w, r, err)
		return
	}
	response.NoContent(w)
}

func (h *ChatHandler) ChangeMemberRole(w http.ResponseWriter, r *http.Request) {
	chatID, err := parseUUID(r, "chat_id")
	if err != nil {
		response.BadRequest(w)
		return
	}
	targetID, err := parseUUID(r, "user_id")
	if err != nil {
		response.BadRequest(w)
		return
	}

	var in chat.ChangeMemberRoleInput
	if err = json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.BadRequest(w)
		return
	}
	in.UserID = targetID

	if err = h.uc.ChangeMemberRole(r.Context(), chatID, mw.UserID(r), in); err != nil {
		response.Err(w, r, err)
		return
	}
	response.NoContent(w)
}

func (h *ChatHandler) LeaveChat(w http.ResponseWriter, r *http.Request) {
	chatID, err := parseUUID(r, "chat_id")
	if err != nil {
		response.BadRequest(w)
		return
	}

	if err = h.uc.LeaveChat(r.Context(), chatID, mw.UserID(r)); err != nil {
		response.Err(w, r, err)
		return
	}
	response.NoContent(w)
}
