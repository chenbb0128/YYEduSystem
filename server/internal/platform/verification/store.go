package verification

import (
	"context"
	"errors"
	"sync"
	"time"

	redisclient "github.com/chenbb0128/tuoguan-system-server/internal/platform/redis"
	goredis "github.com/redis/go-redis/v9"
)

var ErrNotFound = errors.New("verification value not found")

// Store is intentionally small so local tests can use memory while deployed
// instances use Redis. Values are opaque to the store and expire server-side.
type Store interface {
	Get(context.Context, string) ([]byte, error)
	Set(context.Context, string, []byte, time.Duration) error
	Delete(context.Context, string) error
	Acquire(context.Context, string, []byte, time.Duration) (bool, time.Duration, error)
}

type memoryValue struct {
	value     []byte
	expiresAt time.Time
}

type MemoryStore struct {
	mu     sync.Mutex
	values map[string]memoryValue
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{values: make(map[string]memoryValue)}
}

func (s *MemoryStore) Get(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.values[key]
	if !ok || !item.expiresAt.After(time.Now()) {
		delete(s.values, key)
		return nil, ErrNotFound
	}
	return append([]byte(nil), item.value...), nil
}

func (s *MemoryStore) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		return errors.New("verification store TTL must be positive")
	}
	s.mu.Lock()
	s.values[key] = memoryValue{value: append([]byte(nil), value...), expiresAt: time.Now().Add(ttl)}
	s.mu.Unlock()
	return nil
}

func (s *MemoryStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	delete(s.values, key)
	s.mu.Unlock()
	return nil
}

func (s *MemoryStore) Acquire(_ context.Context, key string, value []byte, ttl time.Duration) (bool, time.Duration, error) {
	if ttl <= 0 {
		return false, 0, errors.New("verification store TTL must be positive")
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	if item, ok := s.values[key]; ok && item.expiresAt.After(now) {
		return false, time.Until(item.expiresAt), nil
	}
	s.values[key] = memoryValue{value: append([]byte(nil), value...), expiresAt: now.Add(ttl)}
	return true, 0, nil
}

type RedisStore struct {
	client *redisclient.Client
	keys   redisclient.KeyBuilder
}

func NewRedisStore(client *redisclient.Client) (*RedisStore, error) {
	if client == nil || client.Redis == nil {
		return nil, errors.New("redis client is required")
	}
	return &RedisStore{client: client, keys: client.Keys()}, nil
}

func (s *RedisStore) Get(ctx context.Context, key string) ([]byte, error) {
	redisKey, err := s.keys.Build(key)
	if err != nil {
		return nil, err
	}
	value, err := s.client.Redis.Get(ctx, redisKey).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (s *RedisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	redisKey, err := s.keys.Build(key)
	if err != nil {
		return err
	}
	return s.client.Redis.Set(ctx, redisKey, value, ttl).Err()
}

func (s *RedisStore) Delete(ctx context.Context, key string) error {
	redisKey, err := s.keys.Build(key)
	if err != nil {
		return err
	}
	return s.client.Redis.Del(ctx, redisKey).Err()
}

func (s *RedisStore) Acquire(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, time.Duration, error) {
	redisKey, err := s.keys.Build(key)
	if err != nil {
		return false, 0, err
	}
	acquired, err := s.client.Redis.SetNX(ctx, redisKey, value, ttl).Result()
	if err != nil {
		return false, 0, err
	}
	if acquired {
		return true, 0, nil
	}
	remaining, err := s.client.Redis.TTL(ctx, redisKey).Result()
	if err != nil {
		return false, 0, err
	}
	if remaining <= 0 {
		remaining = ttl
	}
	return false, remaining, nil
}
