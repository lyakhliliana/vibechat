package domain

import "errors"

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailTaken         = errors.New("email already taken")
	ErrUsernameTaken      = errors.New("username already taken")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

var (
	ErrChatNotFound       = errors.New("chat not found")
	ErrNotChatMember      = errors.New("not a chat member")
	ErrAlreadyChatMember  = errors.New("already a chat member")
	ErrInsufficientRights = errors.New("insufficient rights")
	ErrCannotRemoveOwner  = errors.New("cannot remove chat owner")
)

var (
	ErrMessageNotFound  = errors.New("message not found")
	ErrNotMessageAuthor = errors.New("not message author")
	ErrMessageDeleted   = errors.New("message is deleted")
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
)

var (
	ErrInternal   = errors.New("internal error")
	ErrForbidden  = errors.New("forbidden")
	ErrValidation = errors.New("validation error")
)
