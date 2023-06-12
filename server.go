package mxwsgo

import (
	"github.com/hnchenkai/mx-wsgo/limitcount"
	"github.com/hnchenkai/mx-wsgo/serverunit"
)

type LimitOption = limitcount.LimitOption

// NewServerUnit 创建服务单元
func NewServerUnit(dispatcher serverunit.Dispather, limitOption *limitcount.LimitOption) *serverunit.ServerUnit {
	return serverunit.NewServerUnit(dispatcher, limitOption)
}
