package redis

import "context"

type RedisHash struct {
	conn   IRedisConn
	prefix string
}

func (h *RedisHash) doPrefix(key string) string {
	if len(h.prefix) > 0 {
		key = h.prefix + ":" + key
	}

	return h.conn.doPrefix(key)
}

// 获取所有key的数据
func (h *RedisHash) GetAll(ctx context.Context, key string) (map[string]string, error) {
	cmd := h.conn.rdb().HGetAll(ctx, h.doPrefix(key))
	if cmd.Err() != nil {
		return nil, cmd.Err()
	}

	return cmd.Val(), nil
}

// 获取某几个subkey的数据
func (h *RedisHash) Get(ctx context.Context, key string, subKey string) (string, error) {
	cmd := h.conn.rdb().HMGet(ctx, h.doPrefix(key), subKey)
	if cmd.Err() != nil {
		return "", cmd.Err()
	}

	results := cmd.Val()
	if len(results) == 0 {
		return "", nil
	}
	return results[0].(string), nil
}

// 获取某几个subkey的数据
func (h *RedisHash) Gets(ctx context.Context, key string, subKeys ...string) ([]interface{}, error) {
	cmd := h.conn.rdb().HMGet(ctx, h.doPrefix(key), subKeys...)
	if cmd.Err() != nil {
		return nil, cmd.Err()
	}

	return cmd.Val(), nil
}

// 设置key的数据
func (h *RedisHash) Set(ctx context.Context, key string, subKey string, val string) error {
	cmd := h.conn.rdb().HSet(ctx, h.doPrefix(key), subKey, val)
	return cmd.Err()
}

func (h *RedisHash) IncrBy(ctx context.Context, key string, subKey string, val int64) (int64, error) {
	cmd := h.conn.rdb().HIncrBy(ctx, h.doPrefix(key), subKey, val)
	if cmd.Err() != nil {
		return 0, cmd.Err()
	}

	return cmd.Val(), nil
}

func (h *RedisHash) DecrBy(ctx context.Context, key string, subKey string, val int64) (int64, error) {
	cmd := h.conn.rdb().HIncrBy(ctx, h.doPrefix(key), subKey, -val)
	if cmd.Err() != nil {
		return 0, cmd.Err()
	}

	return cmd.Val(), nil
}

// 设置key的数据
func (h *RedisHash) Sets(ctx context.Context, key string, subKeyVals ...string) error {
	cmd := h.conn.rdb().HMSet(ctx, h.doPrefix(key), subKeyVals)
	return cmd.Err()
}

func (h *RedisHash) Del(ctx context.Context, key string, subKeys ...string) error {
	cmd := h.conn.rdb().HDel(ctx, h.doPrefix(key), subKeys...)
	return cmd.Err()
}
