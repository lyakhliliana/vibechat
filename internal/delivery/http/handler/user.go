package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	mw "vibechat/internal/delivery/http/middleware"
	"vibechat/internal/delivery/http/response"
	"vibechat/internal/usecase/user"
)

type UserHandler struct {
	uc user.UseCase
}

func NewUserHandler(uc user.UseCase) *UserHandler {
	return &UserHandler{uc: uc}
}

func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var in user.RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.BadRequest(w)
		return
	}

	u, tokens, err := h.uc.Register(r.Context(), in)
	if err != nil {
		response.Err(w, r, err)
		return
	}
	response.Created(w, map[string]any{"user": u, "tokens": tokens})
}

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var in user.LoginInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.BadRequest(w)
		return
	}

	u, tokens, err := h.uc.Login(r.Context(), in)
	if err != nil {
		response.Err(w, r, err)
		return
	}
	response.OK(w, map[string]any{"user": u, "tokens": tokens})
}

func (h *UserHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RefreshToken == "" {
		response.BadRequest(w)
		return
	}

	tokens, err := h.uc.RefreshToken(r.Context(), body.RefreshToken)
	if err != nil {
		response.Err(w, r, err)
		return
	}
	response.OK(w, tokens)
}

func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	u, err := h.uc.GetProfile(r.Context(), mw.UserID(r))
	if err != nil {
		response.Err(w, r, err)
		return
	}
	response.OK(w, u)
}

func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	var in user.UpdateProfileInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.BadRequest(w)
		return
	}

	u, err := h.uc.UpdateProfile(r.Context(), mw.UserID(r), in)
	if err != nil {
		response.Err(w, r, err)
		return
	}
	response.OK(w, u)
}

func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUID(r, "user_id")
	if err != nil {
		response.BadRequest(w)
		return
	}

	u, err := h.uc.GetProfile(r.Context(), id)
	if err != nil {
		response.Err(w, r, err)
		return
	}
	response.OK(w, u)
}

func (h *UserHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	in := user.SearchInput{
		Query:  q.Get("q"),
		Limit:  queryInt(q.Get("limit"), 20),
		Offset: queryInt(q.Get("offset"), 0),
	}
	if in.Limit < 1 {
		in.Limit = 20
	}

	users, err := h.uc.Search(r.Context(), in)
	if err != nil {
		response.Err(w, r, err)
		return
	}
	response.OK(w, users)
}

func parseUUID(r *http.Request, key string) (uuid.UUID, error) {
	return uuid.Parse(r.PathValue(key))
}

func queryInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}
