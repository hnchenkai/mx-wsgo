package domain

// 关闭信号，专门用于安全关闭系统
type CloseSingal struct {
	sigCloseBegin chan bool // 等待关闭的信号
	sigCloseEnd   chan bool // 发起关闭的人等待关闭完成的信号
	closed        bool      // 关闭与否的状态
}

func NewCloseSingal() *CloseSingal {
	sig := CloseSingal{}
	sig.sigCloseBegin = make(chan bool, 1)
	sig.sigCloseEnd = make(chan bool, 1)
	return &sig
}

// 判定是否满足退出要求 true 表示程序退出
func (p *CloseSingal) Wait() bool {
	select {
	case <-p.sigCloseBegin:
		return true
	default:
		return false
	}
}

func (p *CloseSingal) WaitSingal() chan bool {
	return p.sigCloseBegin
}

// 在wait后面调用defer
func (p *CloseSingal) Defer() {
	p.closed = true
	p.sigCloseEnd <- true
}

// 发起退出请求
func (p *CloseSingal) Close() {
	if p.closed {
		return
	}
	p.sigCloseBegin <- true
	<-p.sigCloseEnd

	// 已经关闭了，就移除掉channal
	close(p.sigCloseBegin)
	close(p.sigCloseEnd)
}
