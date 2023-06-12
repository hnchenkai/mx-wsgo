package mxwsgo

import (
	"github.com/hnchenkai/mx-wsgo/limitcount"
	"github.com/hnchenkai/mx-wsgo/serverunit"
)

type LimitOption = limitcount.LimitOption

/**
 * 创建一个新的服务单元
 * @param  {[type]} dispatcher serverunit.Dispather 服务分发器
 * @param  {[type]} limitOption *limitcount.LimitOption 限制选项，支持本地限制链接和redis分布式限制
 * @return {[type]}             *serverunit.ServerUnit 服务器单元
 */
func NewServerUnit(dispatcher serverunit.Dispather, limitOption *limitcount.LimitOption) *serverunit.ServerUnit {
	return serverunit.NewServerUnit(dispatcher, limitOption)
}
