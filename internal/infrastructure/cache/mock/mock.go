package mock

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type entry struct {
	value     string
	expiresAt time.Time // zero means no expiry
}

// Cache is an in-memory TTL cache for dev/test.
// Expiry is enforced lazily on Get and proactively by a background sweeper
// started via NewWithCleanup.
type Cache struct {
	mu    sync.RWMutex
	store map[string]entry
}

func New() *Cache {
	return &Cache{store: make(map[string]entry)}
}

// NewWithCleanup returns a Cache that runs a background goroutine sweeping
// expired entries every interval. The goroutine stops when ctx is cancelled.
func NewWithCleanup(ctx context.Context, interval time.Duration) *Cache {
	c := New()
	go c.runCleanup(ctx, interval)
	return c
}

func (c *Cache) runCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.deleteExpired()
		case <-ctx.Done():
			return
		}
	}
}

func (c *Cache) deleteExpired() {
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, e := range c.store {
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			delete(c.store, k)
		}
	}
}

func (c *Cache) Get(_ context.Context, key string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e, ok := c.store[key]
	if !ok {
		return "", fmt.Errorf("mock cache: key %q not found", key)
	}
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		delete(c.store, key)
		return "", fmt.Errorf("mock cache: key %q expired", key)
	}
	return e.value, nil
}

func (c *Cache) Set(_ context.Context, key, value string, ttl time.Duration) error {
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	c.mu.Lock()
	c.store[key] = entry{value: value, expiresAt: exp}
	c.mu.Unlock()
	return nil
}

func (c *Cache) Del(_ context.Context, key string) error {
	c.mu.Lock()
	delete(c.store, key)
	c.mu.Unlock()
	return nil
}

func (c *Cache) Ping(_ context.Context) error { return nil }
func (c *Cache) Close() error                 { return nil }
