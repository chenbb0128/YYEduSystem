package queue

import (
	"testing"
	"time"

	"github.com/chenbb0128/tuoguan-system-server/internal/config"
)

func TestRedisClientOptFromConfig(t *testing.T) {
	cfg := config.RedisConfig{
		Addr:         "127.0.0.1:6379",
		Username:     "user",
		Password:     "pass",
		DB:           2,
		DialTimeout:  time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     11,
	}

	opt := RedisClientOptFromConfig(cfg)
	if opt.Network != "tcp" {
		t.Fatalf("Network = %q, want tcp", opt.Network)
	}
	if opt.Addr != cfg.Addr || opt.Username != cfg.Username || opt.Password != cfg.Password || opt.DB != cfg.DB {
		t.Fatalf("RedisClientOptFromConfig() did not copy connection fields")
	}
	if opt.DialTimeout != cfg.DialTimeout || opt.ReadTimeout != cfg.ReadTimeout || opt.WriteTimeout != cfg.WriteTimeout {
		t.Fatalf("RedisClientOptFromConfig() did not copy timeout fields")
	}
	if opt.PoolSize != cfg.PoolSize {
		t.Fatalf("PoolSize = %d, want %d", opt.PoolSize, cfg.PoolSize)
	}
}
