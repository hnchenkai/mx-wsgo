package limitcount

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hnchenkai/mx-wsgo/domain"
	"github.com/hnchenkai/mx-wsgo/limitcount/redis"
	"github.com/hnchenkai/mx-wsgo/wsmessage"
)

// 限流配置
type LimitOption struct {
	RedisConn      redis.IRedisConn
	Namekey        string                    // 服务的命名空间
	TtlInterval    time.Duration             // 有效期更新时间 单位秒 ttl有效期是这个的2倍 默认是10秒
	ReadyLimitFunc func(limitkey string) int // 链接成功状态的总量
	WaitLimitFunc  func(limitkey string) int // 等待状态的总量
}

func (lo *LimitOption) init() {
	if lo.TtlInterval == 0 {
		lo.TtlInterval = 10 * time.Second
	}
}

type IGetMessageFunc = func(clientId string) *wsmessage.WSMessage

// 限流静态数据
type LimitCountUnit struct {
	limitStatic *LimitStatic
	readyPool   *LimitPool
	waitingPool *LimitPool
	getMsgFunc  IGetMessageFunc
}

// NewLimitCountUnit 创建一个限流单元
func NewLimitCountUnit(u IGetMessageFunc) *LimitCountUnit {
	unit := &LimitCountUnit{
		getMsgFunc: u,
	}
	return unit
}

// 初始化方法
func (unit *LimitCountUnit) Init(option *LimitOption) {
	if option == nil {
		option = &LimitOption{}
	}

	option.init()

	unit.limitStatic = &LimitStatic{
		ttlKey:         "gate",
		ttlInterval:    option.TtlInterval,
		limitTtlClient: redis.NewRedisHash(option.RedisConn, fmt.Sprintf("%s:ttl", option.Namekey)),
		closeFd:        domain.NewCloseSingal(),
		gateKey:        uuid.New().String(),
		allWaitQueue:   make(map[string]*domain.Queue),
		parant:         unit,
	}
	unit.limitStatic.init()
	unit.readyPool = &LimitPool{
		name:             "readypool",
		limitCountClient: redis.NewRedisHash(option.RedisConn, fmt.Sprintf("%s:ready:count", option.Namekey)),
		parant:           unit,
		limitFunc:        option.ReadyLimitFunc,
	}
	unit.readyPool.init()
	unit.waitingPool = &LimitPool{
		name:             "waitingPool",
		limitCountClient: redis.NewRedisHash(option.RedisConn, fmt.Sprintf("%s:wait:count", option.Namekey)),
		parant:           unit,
		limitFunc:        option.WaitLimitFunc,
	}
	unit.waitingPool.init()
}

func (unit *LimitCountUnit) Status(limitkey string) string {
	ctx := context.Background()
	ready := unit.readyPool.TotalCount(ctx, limitkey)
	wait := unit.waitingPool.TotalCount(ctx, limitkey)
	return fmt.Sprintf("ready:%d,wait:%d", ready, wait)
}

// Run 负责定时同步redis中的key状态，负责wait的转换到ready
func (unit *LimitCountUnit) Run() {
	if unit.limitStatic == nil {
		return
	}
	// 先激活一下自己
	unit.limitStatic.doActiveUnit()
	go unit.limitStatic.Run()
}

func (unit *LimitCountUnit) Close() {
	if unit.limitStatic == nil {
		return
	}
	unit.limitStatic.CloseRun()
}

// 获取等待信息
func (unit *LimitCountUnit) WaitUnitInfo(ctx context.Context, limitkey string, clientId string) (int64, int64) {
	if unit.limitStatic == nil {
		return -1, 0
	}
	// 这里就按照自己的等待列表里面的数据返回
	return unit.limitStatic.getWaitQueue(limitkey).IndexOf(clientId)
}

// MakeConnStatus 负责生成连接状态
func (unit *LimitCountUnit) MakeConnStatus(limitkey string, clientId string) wsmessage.LimitStatus {
	if unit.limitStatic == nil {
		return wsmessage.LimitAccept
	}
	ctx := context.Background()
	// 优先查一下是否有人排队中，是否需要清理排队队列
	waitQueue := unit.limitStatic.getWaitQueue(limitkey)
	if waitQueue.Size() > 0 {
		// 放入等待队列
		if err := unit.waitingPool.AddCount(ctx, limitkey); err == nil {
			waitQueue.Add(clientId)
			return wsmessage.LimitWait
		}
	} else {
		if err := unit.readyPool.AddCount(ctx, limitkey); err == nil {
			return wsmessage.LimitAccept
		} else if err := unit.waitingPool.AddCount(ctx, limitkey); err == nil {
			waitQueue.Add(clientId)
			return wsmessage.LimitWait
		}
	}

	return wsmessage.LimitReject
}

// CloseConnStatus 负责关闭连接状态
func (unit *LimitCountUnit) CloseConnStatus(limitkey string, clientId string, status wsmessage.LimitStatus) error {
	if unit.limitStatic == nil {
		return nil
	}
	ctx := context.Background()
	switch status {
	case wsmessage.LimitAccept:
		return unit.readyPool.DelCount(ctx, limitkey)
	case wsmessage.LimitWait:
		// 从等待队列中删除
		unit.limitStatic.getWaitQueue(limitkey).Del(clientId)
		return unit.waitingPool.DelCount(ctx, limitkey)
	case wsmessage.LimitReject:
	}

	return nil
}

// UpgrageConnStatus 负责升级连接状态
func (unit *LimitCountUnit) UpgrageConnStatus(ctx context.Context, limitkey string) bool {
	if unit.limitStatic == nil {
		return true
	}
	if unit.readyPool.AddCount(ctx, limitkey) == nil {
		if unit.waitingPool.DelCount(ctx, limitkey) == nil {
			return true
		} else {
			// 这里出现了异常，但是不管他，也放过去了
			return true
		}
	}
	return false
}
