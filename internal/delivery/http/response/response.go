package response

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/rs/zerolog"

	"vibechat/internal/domain"
)

type Body struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func OK(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, Body{Data: data})
}

func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, Body{Data: data})
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func BadRequest(w http.ResponseWriter) {
	JSON(w, http.StatusBadRequest, Body{Error: "bad request"})
}

func Err(w http.ResponseWriter, r *http.Request, err error) {
	status := statusCode(err)
	if status >= 500 {
		zerolog.Ctx(r.Context()).Error().Err(err).Msg("internal error")
	}
	JSON(w, status, Body{Error: err.Error()})
}

func statusCode(err error) int {
	switch {
	case errors.Is(err, domain.ErrUserNotFound),
		errors.Is(err, domain.ErrChatNotFound),
		errors.Is(err, domain.ErrMessageNotFound):
		return http.StatusNotFound

	case errors.Is(err, domain.ErrInvalidCredentials),
		errors.Is(err, domain.ErrInvalidToken),
		errors.Is(err, domain.ErrExpiredToken):
		return http.StatusUnauthorized

	case errors.Is(err, domain.ErrEmailTaken),
		errors.Is(err, domain.ErrUsernameTaken),
		errors.Is(err, domain.ErrAlreadyChatMember):
		return http.StatusConflict

	case errors.Is(err, domain.ErrNotChatMember),
		errors.Is(err, domain.ErrInsufficientRights),
		errors.Is(err, domain.ErrNotMessageAuthor),
		errors.Is(err, domain.ErrForbidden),
		errors.Is(err, domain.ErrCannotRemoveOwner):
		return http.StatusForbidden

	case errors.Is(err, domain.ErrMessageDeleted):
		return http.StatusGone

	case errors.Is(err, domain.ErrValidation):
		return http.StatusUnprocessableEntity

	default:
		return http.StatusInternalServerError
	}
}
