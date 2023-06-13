package mxwsgo

import (
	"context"
	"net/http"

	"github.com/hnchenkai/mx-wsgo/limitcount"
	"github.com/hnchenkai/mx-wsgo/serverunit"
	"github.com/hnchenkai/mx-wsgo/wsmessage"
)

type LimitOption = limitcount.LimitOption

type IServerUnit interface {
	// 添加链接信息
	AddHeader(clientId string, key string, value string) bool
	// 删除信息
	DelHeader(clientId string, key string) bool
	// 广播
	Broadcast(message []byte)
	// 发送数据
	Send(clientId string, message []byte) bool
	// 关闭服务
	Close()
	// 获取链接信息
	GetConnMessage(clientId string) *wsmessage.WSMessage
	// 启动定时器等
	Run()
	// 可以挂载到一个http服务上去,从http升级到https
	// header中 携带 Mx-Ws- 会被转发到ws的header中
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	// 获取等待信息
	WaitUnitInfo(ctx context.Context, limitkey string, clientId string) (int64, int64)
}

/**
 * 创建一个新的服务单元
 * @param  {[type]} dispatcher serverunit.Dispather 服务分发器
 * @param  {[type]} limitOption *limitcount.LimitOption 限制选项，支持本地限制链接和redis分布式限制
 * @return {[type]}             IServerUnit 服务器单元
 */
func NewServerUnit(dispatcher serverunit.Dispather, limitOption *LimitOption) IServerUnit {
	return serverunit.NewServerUnit(dispatcher, limitOption)
}

// 设置分组信息
func SetGroup(header http.Header, group string) http.Header {
	header.Add(wsmessage.WsGroupHeader, group)
	return header
}

// 添加head信息 会被转发到ws的header中
func AddHeader(header http.Header, k string, group string) http.Header {
	header.Add(wsmessage.PrefixProxyHeader+k, group)
	return header
}
