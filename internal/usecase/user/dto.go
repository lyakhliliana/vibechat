package user

import "vibechat/utils/validate"

type RegisterInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (in RegisterInput) Validate() error {
	return validate.New().
		Check(validate.MinLen(in.Username, 3), "username must be at least 3 characters").
		Check(validate.MaxLen(in.Username, 50), "username must be at most 50 characters").
		Check(validate.IsAlphanumeric(in.Username), "username must contain only letters and digits").
		Check(validate.IsEmail(in.Email), "invalid email address").
		Check(validate.MinLen(in.Password, 8), "password must be at least 8 characters").
		Check(validate.MaxLen(in.Password, 72), "password must be at most 72 characters").
		Err()
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (in LoginInput) Validate() error {
	return validate.New().
		Check(validate.NonEmpty(in.Email), "email is required").
		Check(validate.NonEmpty(in.Password), "password is required").
		Err()
}

// UpdateProfileInput uses pointer fields for partial updates (nil = leave unchanged).
type UpdateProfileInput struct {
	Username  *string `json:"username"`
	Bio       *string `json:"bio"`
	AvatarURL *string `json:"avatar_url"`
}

func (in UpdateProfileInput) Validate() error {
	v := validate.New()
	if in.Username != nil {
		v.Check(validate.MinLen(*in.Username, 3), "username must be at least 3 characters").
			Check(validate.MaxLen(*in.Username, 50), "username must be at most 50 characters").
			Check(validate.IsAlphanumeric(*in.Username), "username must contain only letters and digits")
	}
	if in.Bio != nil {
		v.Check(validate.MaxLen(*in.Bio, 500), "bio must be at most 500 characters")
	}
	if in.AvatarURL != nil {
		v.Check(validate.IsURL(*in.AvatarURL), "avatar_url must be a valid HTTP/HTTPS URL")
	}
	return v.Err()
}

type SearchInput struct {
	Query  string `json:"query"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

func (in SearchInput) Validate() error {
	return validate.New().
		Check(validate.NonEmpty(in.Query), "query is required").
		Check(validate.InRange(in.Limit, 1, 100), "limit must be between 1 and 100").
		Check(in.Offset >= 0, "offset must be non-negative").
		Err()
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}
