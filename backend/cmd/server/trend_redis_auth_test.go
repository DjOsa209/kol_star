package main

import (
	"context"
	"os"
	"testing"
	"time"
)

// Only for an isolated fixture server started with this public test password.
// Never reads the application's actual Redis credentials.
func TestTrendRedisAuthenticatedFixture(t *testing.T) {
	addr := os.Getenv("TREND_REDIS_AUTH_TEST_ADDR")
	if addr == "" {
		t.Skip("isolated authenticated fixture only")
	}
	cache := redisTrendCache{config: RedisConfig{Addr: addr, Username: "default", Password: "codex-test-only-redis", Database: 15}}
	ctx := context.Background()
	key := "kol:trends:auth-fixture:" + time.Now().Format("150405.000000000")
	if err := cache.Set(ctx, key, []byte("fixture"), time.Now().Add(time.Minute)); err != nil {
		t.Fatal("authenticated cache write failed")
	}
	value, err := cache.Get(ctx, key)
	if err != nil || string(value) != "fixture" {
		t.Fatal("authenticated cache read failed")
	}
}
