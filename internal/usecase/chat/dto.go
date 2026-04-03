package chat

import (
	"github.com/google/uuid"

	"vibechat/utils/validate"
)

type CreateDirectInput struct {
	TargetUserID uuid.UUID `json:"target_user_id"`
}

func (in CreateDirectInput) Validate() error {
	return validate.New().
		Check(in.TargetUserID != uuid.Nil, "target_user_id is required").
		Err()
}

type CreateGroupInput struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	MemberIDs   []uuid.UUID `json:"member_ids"`
}

func (in CreateGroupInput) Validate() error {
	return validate.New().
		Check(validate.MinLen(in.Name, 1), "name is required").
		Check(validate.MaxLen(in.Name, 100), "name must be at most 100 characters").
		Check(validate.MaxLen(in.Description, 500), "description must be at most 500 characters").
		Err()
}

type UpdateGroupInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	AvatarURL   *string `json:"avatar_url"`
}

func (in UpdateGroupInput) Validate() error {
	v := validate.New()
	if in.Name != nil {
		v.Check(validate.MinLen(*in.Name, 1), "name must not be empty").
			Check(validate.MaxLen(*in.Name, 100), "name must be at most 100 characters")
	}
	if in.Description != nil {
		v.Check(validate.MaxLen(*in.Description, 500), "description must be at most 500 characters")
	}
	if in.AvatarURL != nil {
		v.Check(validate.IsURL(*in.AvatarURL), "avatar_url must be a valid HTTP/HTTPS URL")
	}
	return v.Err()
}

type AddMemberInput struct {
	UserID uuid.UUID `json:"user_id"`
}

func (in AddMemberInput) Validate() error {
	return validate.New().
		Check(in.UserID != uuid.Nil, "user_id is required").
		Err()
}

type ChangeMemberRoleInput struct {
	UserID uuid.UUID `json:"user_id"`
	Role   string    `json:"role"`
}

func (in ChangeMemberRoleInput) Validate() error {
	return validate.New().
		Check(in.UserID != uuid.Nil, "user_id is required").
		Check(validate.OneOf(in.Role, "admin", "member"), "role must be one of: admin, member").
		Err()
}
