package mxwsgo

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hnchenkai/mx-wsgo/bytecoder"
	"google.golang.org/protobuf/proto"
)

type WSMessage struct {
	// 请求对应的host
	Host string
	// 连接id
	ClientId int64 `json:"clientId,omitempty"`
	// 原始头 这里只是拷贝，不能直接修改
	OrgHeader http.Header
	// 发送消息的方法
	Send func(message []byte) bool

	// 关闭连接的方法
	Close func()
	// 添加头的方法
	AddHeader func(key, value string) bool
	DelHeader func(key string) bool

	Version int

	ReqId   int64                 `json:"requestId"`
	Method  string                `json:"method"`
	Route   string                `json:"route"`
	Cmd     bytecoder.MsgLocalCmd `json:"cmd"` // cmd版本才会有
	Message []byte                `json:"message"`
	Header  map[string]string     `json:"header"`

	ResponseHeader bool `json:"responseHeader"`
}

// 从json格式过来的
func (app *WSMessage) FromJson(jsonbt []byte) error {
	return json.Unmarshal(jsonbt, app)
}

// 从数据流过来的
func (app *WSMessage) FromPb(msg1 []byte) error {
	coder := bytecoder.StreamCoder(msg1)
	coder.DecodeWS()
	// 这里要压缩获取信息
	coder.UnGzip()
	switch coder.Version() {
	case bytecoder.Version_VERSION_1:
		msg, _ := coder.UnmarshalV1()
		app.Version = 1
		app.ReqId = msg.GetRequestId()
		app.Route = msg.GetRoute()
		app.Message = msg.GetBody()
		app.Header = msg.GetHeader()
		app.Method = msg.GetMethod()
		app.ResponseHeader = msg.GetResponseHeader()
	case bytecoder.Version_VERSION_2:
		msg, _ := coder.UnmarshalV2()
		app.Version = 2
		app.ReqId = msg.GetRequestId()
		app.Route = fmt.Sprintf("/%d", app.Cmd)
		app.Message = msg.GetBody()
		app.Method = "POST"
		app.Header = msg.GetHeader()
		app.ResponseHeader = msg.GetResponseHeader()
	case bytecoder.Version_VERSION_0_UNSPECIFIED:
		// 收到一个错误信息，我也是真xxx了
		return fmt.Errorf("error message")
	case bytecoder.Version_VERSION_CMD:
		msg, _ := coder.UnmarshalCmd()
		app.Version = 3
		app.ReqId = msg.GetRequestId()
		app.Cmd = msg.GetCmd()
		app.Message = msg.GetBody()
		app.Method = "POST"
		app.Header = map[string]string{}
	default:
		return fmt.Errorf("unknown version %d", coder.Version())
	}

	return nil
}

// 应答消息给用户
func (app *WSMessage) SendResponse(code int32, body []byte, header map[string]string) bool {
	coder := bytecoder.MarshalV0(app.ReqId, code, body, header)
	// dst, _ := app.gzip(coder)
	coder.Gzip()
	coder.EncodeWS()
	return app.Send(coder)
}

func (app *WSMessage) SendError(code int32, body string, header map[string]string) bool {
	bt, _ := json.Marshal(map[string]string{
		"message": body,
	})
	coder := bytecoder.MarshalV0(app.ReqId, code, bt, header)
	// dst, _ := app.gzip(coder)
	coder.Gzip()
	coder.EncodeWS()
	return app.Send(coder)
}

// 应答消息给用户
func (app *WSMessage) SendResponseCmd(code bytecoder.MsgLocalCmd, body []byte, header map[string]string) bool {
	coder := bytecoder.MarshalCMD(app.ReqId, code, body, header)
	coder.Gzip()
	coder.EncodeWS()
	return app.Send(coder)
}

func (app *WSMessage) sendProto() bool {
	coder := bytecoder.ShowProtoFile()
	if coder != nil {
		coder.Gzip()
		coder.EncodeWS()
		return app.Send(coder)
	}

	return false
}

func (app *WSMessage) IsAccept() bool {
	ok := app.OrgHeader.Get("MX-WSGO-ACCEPT")
	return ok == "accept"
}

func (app *WSMessage) SetAcceptMode() {
	app.DelHeader("MX-WSGO-ACCEPT")
	app.AddHeader("MX-WSGO-ACCEPT", "accept")
	app.SendResponseCmd(bytecoder.MsgLocalCmd_MSG_LOCAL_CMD_WS_ACCEPT, []byte("连接成功"), nil)
}

type WaitInfo struct {
	Self  int64 `json:"self"`
	Total int64 `json:"total"`
}

func (app *WSMessage) SetWaitMode(self int64, total int64) {
	info := bytecoder.MessageWaitInfo{
		Self:  self,
		Total: total,
	}

	bt, _ := proto.Marshal(&info)
	// info := WaitInfo{
	// 	Self:  self,
	// 	Total: total,
	// }
	// bt, _ := json.Marshal(info)

	app.AddHeader("MX-WSGO-ACCEPT", "wait")
	app.SendResponseCmd(bytecoder.MsgLocalCmd_MSG_LOCAL_CMD_WS_WAIT, bt, nil)
}

func (app *WSMessage) WaitResponse(self int64, total int64) {
	info := bytecoder.MessageWaitInfo{
		Self:  self,
		Total: total,
	}

	bt, _ := proto.Marshal(&info)

	// info := WaitInfo{
	// 	Self:  self,
	// 	Total: total,
	// }
	// bt, _ := json.Marshal(info)
	app.SendResponseCmd(bytecoder.MsgLocalCmd_MSG_LOCAL_CMD_WS_RESP, bt, nil)
}
