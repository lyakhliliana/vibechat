package mock

import (
	"context"
	"testing"
	"time"
)

func TestGetSetDel(t *testing.T) {
	c := New()
	ctx := context.Background()

	if err := c.Set(ctx, "k", "v", 0); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := c.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != "v" {
		t.Fatalf("want v, got %q", got)
	}

	if err := c.Del(ctx, "k"); err != nil {
		t.Fatalf("Del: %v", err)
	}

	if _, err := c.Get(ctx, "k"); err == nil {
		t.Fatal("expected error after Del, got nil")
	}
}

func TestGet_MissingKey(t *testing.T) {
	c := New()
	_, err := c.Get(context.Background(), "no-such-key")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestTTL_ExpiredOnGet(t *testing.T) {
	c := New()
	ctx := context.Background()

	_ = c.Set(ctx, "k", "v", 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	_, err := c.Get(ctx, "k")
	if err == nil {
		t.Fatal("expected error for expired key")
	}
}

func TestTTL_Zero_NeverExpires(t *testing.T) {
	c := New()
	ctx := context.Background()

	_ = c.Set(ctx, "k", "v", 0) // 0 = no expiry

	got, err := c.Get(ctx, "k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "v" {
		t.Fatalf("want v, got %q", got)
	}
}

func TestDeleteExpired_Sweeper(t *testing.T) {
	c := New()
	ctx := context.Background()

	_ = c.Set(ctx, "expires", "v", 1*time.Millisecond)
	_ = c.Set(ctx, "permanent", "v", 0)

	time.Sleep(5 * time.Millisecond)
	c.deleteExpired()

	if _, err := c.Get(ctx, "expires"); err == nil {
		t.Fatal("expired key should be gone after sweep")
	}
	if _, err := c.Get(ctx, "permanent"); err != nil {
		t.Fatal("permanent key should survive sweep")
	}
}

func TestDel_NonExistentKey_NoError(t *testing.T) {
	c := New()
	if err := c.Del(context.Background(), "no-such-key"); err != nil {
		t.Fatalf("Del on missing key should not error: %v", err)
	}
}

func TestPing(t *testing.T) {
	c := New()
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestOverwrite(t *testing.T) {
	c := New()
	ctx := context.Background()

	_ = c.Set(ctx, "k", "first", 0)
	_ = c.Set(ctx, "k", "second", 0)

	got, _ := c.Get(ctx, "k")
	if got != "second" {
		t.Fatalf("want second, got %q", got)
	}
}
