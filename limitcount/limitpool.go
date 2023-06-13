package limitcount

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/hnchenkai/mx-wsgo/limitcount/redis"
)

type LimitPool struct {
	// 限量池名称
	name string
	// 限量管理对象
	limitCountClient redis.IHash

	// 管理者对象
	parant *LimitCountUnit

	// 限量函数
	limitFunc func(limitkey string) int
}

func freshValidValue(ttls map[string]string, limits map[string]string, limitTime time.Duration) ([]string, []string) {
	ttlOutKeys := []string{}
	now := time.Now().Unix()
	for k, v := range ttls {
		// 这里踢掉哪些过期的
		iTime, _ := strconv.ParseInt(v, 10, 64)
		if iTime+int64(limitTime)*2 < now {
			// 这些就不需要了
			delete(ttls, k)
			ttlOutKeys = append(ttlOutKeys, k)
		}
	}

	limitsOutKeys := []string{}
	for k := range limits {
		if _, ok := ttls[k]; !ok {
			delete(limits, k)
			limitsOutKeys = append(limitsOutKeys, k)
		}
	}

	return ttlOutKeys, limitsOutKeys
}

func toMapValueInt(limits map[string]string, key string) int {
	count, ok := limits[key]
	if !ok {
		count = "0"
	}

	return toValueInt(count)
}

func toValueInt(count string) int {
	if count == "" {
		return 0
	}

	iCount, err := strconv.Atoi(count)
	if err != nil {
		return 0
	}

	return iCount
}

func (p *LimitPool) init() {
	if p.limitCountClient == nil {
		// 信息不存在的时候 创建一个本地的对象
		p.limitCountClient = redis.NewLocalHash()
	}
}

// AddCount 添加一个数量
func (p *LimitPool) AddCount(ctx context.Context, limitkey string) (result error) {
	limits, err := p.limitCountClient.GetAll(ctx, limitkey)
	if err != nil {
		return err
	}

	ttls, err := p.parant.limitStatic.getAll(ctx)
	if err != nil {
		return err
	}

	outttls, outlimits := freshValidValue(ttls, limits, p.parant.limitStatic.ttlInterval)

	// 这里要计算出，自己有多少了，总共有多少了
	selfCount := toMapValueInt(limits, p.parant.limitStatic.gateKey)
	// 这里要计算一下总量有多少了
	sumTotal := 0
	for _, v := range limits {
		sumTotal += toValueInt(v)
	}

	// 这里要读取配置信息，用来确定可以使用的上线
	limitCount := 0
	if p.limitFunc != nil {
		defer func() {
			if err := recover(); err != nil {
				limitCount = 0
				result = errors.New("limit func error")
			}
		}()
		limitCount = p.limitFunc(limitkey)
	}
	if limitCount >= 0 && sumTotal >= limitCount {
		return errors.New("数量满了，请等待")
	}

	//看看自己是否超标了，看看总量是否超标了
	if err := p.limitCountClient.Set(ctx, limitkey, p.parant.limitStatic.gateKey, fmt.Sprint(selfCount+1)); err != nil {
		return err
	}
	// redis.LimitCountClient.IncrBy(ctx, limitkey, gateKey, 1)

	if len(outlimits) > 0 {
		p.limitCountClient.Del(ctx, limitkey, outlimits...)
	}

	if len(outttls) > 0 {
		p.parant.limitStatic.del(ctx, outttls...)
	}

	return nil
}

// 移除一个数量
func (p *LimitPool) DelCount(ctx context.Context, limitkey string) error {
	count, err := p.limitCountClient.DecrBy(ctx, limitkey, p.parant.limitStatic.gateKey, 1)
	if err != nil {
		return err
	}

	if count < 0 {
		p.limitCountClient.Del(ctx, limitkey, p.parant.limitStatic.gateKey)
		return errors.New("limit key is out of range")
	}

	return nil
}

// 自己的链接数量
func (p *LimitPool) SelfCount(ctx context.Context, limitkey string) int {
	count, _ := p.limitCountClient.Get(ctx, limitkey, p.parant.limitStatic.gateKey)
	return toValueInt(count)
}

type PoolUnit struct {
	limits map[string]string
	ttls   map[string]string

	outLimits []string
	outTtls   []string

	statics *LimitStatic
}

// 新建一个池子单元结构
func NewPoolUnit(limits map[string]string, ttls map[string]string, statics *LimitStatic) *PoolUnit {
	unit := &PoolUnit{
		limits:  limits,
		ttls:    ttls,
		statics: statics,
	}

	return unit
}

// 初始化方法
func (pool *PoolUnit) Init() {
	out1, out2 := freshValidValue(pool.ttls, pool.limits, pool.statics.ttlInterval)
	pool.outTtls = out1
	pool.outLimits = out2
}

func (pool *PoolUnit) TotalCount() int {
	sumTotal := 0
	for _, v := range pool.limits {
		sumTotal += toValueInt(v)
	}
	return sumTotal
}

func (pool *PoolUnit) SelfCount() int {
	if count, ok := pool.limits[pool.statics.gateKey]; ok {
		return toValueInt(count)
	} else {
		return 0
	}
}

// 计算活动的总量
func (p *LimitPool) TotalCount(ctx context.Context, limitkey string) int {
	limits, err := p.limitCountClient.GetAll(ctx, limitkey)
	if err != nil {
		return 0
	}

	ttls, err := p.parant.limitStatic.getAll(ctx)
	if err != nil {
		return 0
	}

	freshValidValue(ttls, limits, p.parant.limitStatic.ttlInterval)
	// 这里要计算一下总量有多少了
	sumTotal := 0
	for _, v := range limits {
		sumTotal += toValueInt(v)
	}
	return sumTotal
}
