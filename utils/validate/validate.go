// Package validate provides zero-dependency input validation wrapping domain.ErrValidation.
package validate

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"vibechat/internal/domain"
)

var (
	emailRe    = regexp.MustCompile(`(?i)^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)
	urlRe      = regexp.MustCompile(`^https?://\S+$`)
	alphanumRe = regexp.MustCompile(`^[a-zA-Z0-9]+$`)
)

type V struct{ err error }

func New() *V { return &V{} }

func (v *V) Check(cond bool, msg string) *V {
	if v.err == nil && !cond {
		v.err = fmt.Errorf("%w: %s", domain.ErrValidation, msg)
	}
	return v
}

func (v *V) Err() error { return v.err }

func NonEmpty(s string) bool      { return strings.TrimSpace(s) != "" }
func MinLen(s string, n int) bool { return utf8.RuneCountInString(s) >= n }
func MaxLen(s string, n int) bool { return utf8.RuneCountInString(s) <= n }

func IsAlphanumeric(s string) bool { return s != "" && alphanumRe.MatchString(s) }
func IsEmail(s string) bool        { return emailRe.MatchString(s) }
func IsURL(s string) bool          { return urlRe.MatchString(s) }

func OneOf(s string, allowed ...string) bool {
	return slices.Contains(allowed, s)
}

func InRange(v, min, max int) bool { return v >= min && v <= max }
