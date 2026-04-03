package validate

import (
	"testing"
)

func TestNonEmpty(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"hello", true},
		{"  hello  ", true},
		{"", false},
		{"   ", false},
		{"\t\n", false},
	}
	for _, c := range cases {
		if got := NonEmpty(c.in); got != c.want {
			t.Errorf("NonEmpty(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestMinMaxLen(t *testing.T) {
	// unicode-aware: каждый символ = 1 rune
	if !MinLen("abc", 3) {
		t.Error("MinLen(abc, 3) should be true")
	}
	if MinLen("ab", 3) {
		t.Error("MinLen(ab, 3) should be false")
	}
	if !MinLen("привет", 6) {
		t.Error("MinLen(привет, 6) should be true (6 runes)")
	}
	if !MaxLen("abc", 3) {
		t.Error("MaxLen(abc, 3) should be true")
	}
	if MaxLen("abcd", 3) {
		t.Error("MaxLen(abcd, 3) should be false")
	}
}

func TestIsAlphanumeric(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"abc123", true},
		{"ABC", true},
		{"abc 123", false},
		{"abc-123", false},
		{"привет", false},
		{"", false},
	}
	for _, c := range cases {
		if got := IsAlphanumeric(c.in); got != c.want {
			t.Errorf("IsAlphanumeric(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestIsEmail(t *testing.T) {
	valid := []string{
		"user@example.com",
		"user+tag@sub.domain.org",
		"USER@EXAMPLE.COM",
	}
	invalid := []string{
		"not-an-email",
		"@example.com",
		"user@",
		"user@.com",
		"",
	}
	for _, e := range valid {
		if !IsEmail(e) {
			t.Errorf("IsEmail(%q) should be true", e)
		}
	}
	for _, e := range invalid {
		if IsEmail(e) {
			t.Errorf("IsEmail(%q) should be false", e)
		}
	}
}

func TestIsURL(t *testing.T) {
	valid := []string{
		"http://example.com",
		"https://example.com/path?q=1",
	}
	invalid := []string{
		"ftp://example.com",
		"example.com",
		"//example.com",
		"",
	}
	for _, u := range valid {
		if !IsURL(u) {
			t.Errorf("IsURL(%q) should be true", u)
		}
	}
	for _, u := range invalid {
		if IsURL(u) {
			t.Errorf("IsURL(%q) should be false", u)
		}
	}
}

func TestOneOf(t *testing.T) {
	if !OneOf("admin", "admin", "member") {
		t.Error("OneOf(admin, ...) should be true")
	}
	if OneOf("owner", "admin", "member") {
		t.Error("OneOf(owner, admin, member) should be false")
	}
	if OneOf("", "admin") {
		t.Error("OneOf(empty) should be false")
	}
}

func TestInRange(t *testing.T) {
	if !InRange(5, 1, 10) {
		t.Error("InRange(5,1,10) should be true")
	}
	if !InRange(1, 1, 10) {
		t.Error("InRange(1,1,10) boundary should be true")
	}
	if !InRange(10, 1, 10) {
		t.Error("InRange(10,1,10) boundary should be true")
	}
	if InRange(0, 1, 10) {
		t.Error("InRange(0,1,10) should be false")
	}
	if InRange(11, 1, 10) {
		t.Error("InRange(11,1,10) should be false")
	}
}

func TestValidator_StopsAtFirstError(t *testing.T) {
	var calls int
	sideEffect := func() bool {
		calls++
		return false
	}

	// Check() always evaluates its arguments (Go is eager), but only the first
	// error message is retained. Verify that only the first message is kept.
	v := New().
		Check(false, "first error").
		Check(sideEffect(), "second error")

	err := v.Err()
	if err == nil {
		t.Fatal("expected an error")
	}
	if err.Error() != "validation error: first error" {
		t.Errorf("unexpected error message: %q", err.Error())
	}
	if calls != 1 {
		t.Errorf("sideEffect called %d times, want 1", calls)
	}
}

func TestValidator_NoErrorWhenAllPass(t *testing.T) {
	v := New().
		Check(true, "should not appear").
		Check(true, "also fine")
	if err := v.Err(); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}
