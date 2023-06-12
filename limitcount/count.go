package limitcount

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/hnchenkai/mx-wsgo/domain"
	"github.com/hnchenkai/mx-wsgo/limitcount/redis"
)

// 这里计算一下redis key为中心的限制模式
type LimitStatic struct {
	allWaitQueue map[string]*domain.Queue

	limitTtlClient redis.IHash
	closeFd        *domain.CloseSingal
	// 有效期时间，单位秒 默认10秒
	ttlInterval time.Duration
	// gateKey 当前服务器的唯一标识
	gateKey string
	ttlKey  string

	// 心跳信息
	waitReadyInterval time.Duration

	parant *LimitCountUnit
}

func (s *LimitStatic) init() {
	if s.waitReadyInterval == 0 {
		s.waitReadyInterval = 5 * time.Second
	}

	if s.ttlInterval == 0 {
		s.ttlInterval = 10 * time.Second
	}
}

func (s *LimitStatic) getAll(ctx context.Context) (map[string]string, error) {
	return s.limitTtlClient.GetAll(ctx, s.ttlKey)
}

func (s *LimitStatic) del(ctx context.Context, outttls ...string) {
	if len(outttls) > 0 {
		s.limitTtlClient.Del(ctx, s.ttlKey, outttls...)
	}
}

// 激活当前的节点
func (s *LimitStatic) doActiveUnit() {
	s.limitTtlClient.Set(context.Background(), s.ttlKey, s.gateKey, fmt.Sprint(time.Now().Unix()))
}

// RunTtl负责定时同步redis中的key状态，负责wait的转换到ready
func (s *LimitStatic) Run() {
	// redis
	tick := time.NewTimer(0)
	tickUp := time.NewTimer(s.waitReadyInterval)
	close := false
	for {
		if close {
			break
		}
		select {
		case <-tick.C:
			// 刷新服务的有效期
			s.doActiveUnit()
			tick.Reset(s.ttlInterval * time.Second)
		case <-tickUp.C:
			// 单独一个协程负责更新
			if s.parant.getMsgFunc != nil {
				go s.RunAllocWaitToReady()
				tickUp.Reset(s.waitReadyInterval)
			}
		case <-s.closeFd.WaitSingal():
			close = true
		default:
			time.Sleep(100 * time.Millisecond)
		}

	}
	s.closeFd.Defer()
}

func (s *LimitStatic) CloseRun() {
	s.closeFd.Close()
}

func (s *LimitStatic) getWaitQueue(limitkey string) *domain.Queue {
	unit, ok := s.allWaitQueue[limitkey]
	if !ok {
		unit = domain.NewQueue()
		s.allWaitQueue[limitkey] = unit
	}

	return unit
}

// 有好多活动，每个活动都是不同的通道，需要单独更新
func (s *LimitStatic) RunAllocWaitToReady() {
	for k, v := range s.allWaitQueue {
		if v.Size() == 0 {
			continue
		}
		s.allocWaitToReady(k, v)
	}
}

// 负责分配多少人从wait转reday
func (s *LimitStatic) allocWaitToReady(limitkey string, waitQueue *domain.Queue) {
	if waitQueue.Size() == 0 {
		return
	}
	ctx := context.Background()
	// 第一步判断总量
	totalCount := s.parant.readyPool.TotalCount(ctx, limitkey)
	limitCount := s.parant.readyPool.limitFunc(limitkey)

	// 计算出总量
	leftCount := limitCount - totalCount

	//第二部判断可分配数量
	//获取等待队列总量
	waitTotalCount := s.parant.waitingPool.TotalCount(ctx, limitkey)

	// 计算出可分配数量
	allocCount := int64(math.Abs(float64(waitQueue.Size()) / float64(waitTotalCount) * float64(leftCount)))

	// 第三步分配 取出用户参与分配
	var loopIndex int64
	for loopIndex = 0; loopIndex < allocCount; loopIndex++ {
		clientId := waitQueue.Shift()
		if clientId == nil {
			// 找不到了
			break
		}
		sClientId := clientId.(string)
		msg := s.parant.getMsgFunc(sClientId)
		if msg == nil || msg.IsAccept() {
			continue
		}
		// 分配到ready
		if s.parant.UpgrageConnStatus(ctx, limitkey) {
			// 通知客户端
			msg.SetAcceptMode()
		} else {
			//再丢回去 然后退出操作了
			waitQueue.Add(sClientId)
			break
		}
	}
}
