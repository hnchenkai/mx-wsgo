package redis

import (
	"context"
	"fmt"
	"strconv"
	"sync"
)

type LocalHashUnit struct {
	lock *sync.RWMutex
	data map[string]string
}

func (l *LocalHashUnit) Get(ctx context.Context, key string) (string, error) {
	l.lock.RLock()
	defer l.lock.RUnlock()
	return l.data[key], nil
}

func (l *LocalHashUnit) GetAll(ctx context.Context) (map[string]string, error) {
	l.lock.RLock()
	defer l.lock.RUnlock()
	return l.data, nil
}

func (l *LocalHashUnit) Del(ctx context.Context, subKeys ...string) error {
	l.lock.Lock()
	defer l.lock.Unlock()
	for _, v := range subKeys {
		delete(l.data, v)
	}
	return nil
}

func (l *LocalHashUnit) Set(ctx context.Context, key string, val string) error {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.data[key] = val
	return nil
}

func (l *LocalHashUnit) IncrBy(ctx context.Context, key string, val int64) (int64, error) {
	l.lock.Lock()
	defer l.lock.Unlock()
	v, ok := l.data[key]
	if !ok {
		v = "0"
	}
	vi, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return vi, err
	}
	vi += val
	l.data[key] = fmt.Sprint(vi)
	return vi, nil
}

func (l *LocalHashUnit) DecrBy(ctx context.Context, key string, val int64) (int64, error) {
	l.lock.Lock()
	defer l.lock.Unlock()
	v, ok := l.data[key]
	if !ok {
		v = "0"
	}
	vi, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return vi, err
	}
	vi -= val
	l.data[key] = fmt.Sprint(vi)
	return vi, nil
}

type LocalHash struct {
	data map[string]*LocalHashUnit
}

func (l *LocalHash) getUnit(key string) *LocalHashUnit {
	unit, ok := l.data[key]
	if !ok {
		unit = &LocalHashUnit{
			lock: &sync.RWMutex{},
			data: make(map[string]string),
		}
		l.data[key] = unit
	}
	return unit
}

func (l *LocalHash) Get(ctx context.Context, key, subKey string) (string, error) {
	unit, ok := l.data[key]
	if !ok {
		return "", nil
	}
	return unit.Get(ctx, subKey)
}

func (l *LocalHash) GetAll(ctx context.Context, key string) (map[string]string, error) {
	unit, ok := l.data[key]
	if !ok {
		return nil, nil
	}
	return unit.GetAll(ctx)
}

func (l *LocalHash) Del(ctx context.Context, key string, subKeys ...string) error {
	unit, ok := l.data[key]
	if !ok {
		return nil
	}
	return unit.Del(ctx, subKeys...)
}

func (l *LocalHash) Set(ctx context.Context, key string, subKey string, val string) error {
	unit := l.getUnit(key)
	return unit.Set(ctx, subKey, val)
}

func (l *LocalHash) IncrBy(ctx context.Context, key string, subKey string, val int64) (int64, error) {
	unit := l.getUnit(key)
	return unit.IncrBy(ctx, subKey, val)
}

func (l *LocalHash) DecrBy(ctx context.Context, key string, subKey string, val int64) (int64, error) {
	unit := l.getUnit(key)
	return unit.DecrBy(ctx, subKey, val)
}

func NewLocalHash() *LocalHash {
	return &LocalHash{
		data: make(map[string]*LocalHashUnit),
	}
}
