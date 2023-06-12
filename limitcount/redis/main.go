package redis

import (
	"context"

	"github.com/go-redis/redis/v8"
)

type IRedisConn interface {
	rdb() *redis.Client
	doPrefix(key string) string
}

// 做一个local的模式

type IHash interface {
	Get(ctx context.Context, key, subKey string) (string, error)
	GetAll(ctx context.Context, key string) (map[string]string, error)
	Del(ctx context.Context, key string, subKeys ...string) error
	Set(ctx context.Context, key string, subKey string, val string) error
	IncrBy(ctx context.Context, key string, subKey string, val int64) (int64, error)
	DecrBy(ctx context.Context, key string, subKey string, val int64) (int64, error)
}

func NewRedisHash(conn IRedisConn, prefix string) IHash {
	if conn == nil {
		return NewLocalHash()
	}
	return &RedisHash{
		conn:   conn,
		prefix: prefix,
	}
}
