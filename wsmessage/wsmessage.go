package wsmessage

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/hnchenkai/mx-wsgo/bytecoder"
	"google.golang.org/protobuf/proto"
)

type Cmd string

const (
	PrefixProxyHeader = "Mx-Ws-"
	PrefixLocalHeader = "Mx-Wsgo-"
	WsGroupHeader     = PrefixLocalHeader + "Group"
	WsStatusHeader    = PrefixLocalHeader + "Status"
)

const (
	CmdMessage Cmd = "message" // 收到消息
	CmdAccept  Cmd = "accept"  // 链接被正确接受
	CmdClose   Cmd = "close"   // 链接被关闭

	CmdCmd Cmd = "cmd" // 内部特殊的命令

	CmdWait   Cmd = "wait"   // 开启排队模式后被列入排队状态的
	CmdReject Cmd = "reject" // 开启排队模式后被拒绝的
)

// 链接状态信息
type LimitStatus string

// 内部使用的链接状态信息
const (
	LimitAccept LimitStatus = "accept"
	LimitWait   LimitStatus = "wait"
	LimitReject LimitStatus = "reject"
)

type WSMessage struct {
	// 请求对应的host
	Host string
	// 连接id
	ClientId string `json:"clientId,omitempty"`
	// 原始头 这里只是拷贝，不能直接修改
	OrgHeader http.Header
	// 发送消息的方法
	Send func(message []byte) bool

	// 关闭连接的方法
	Close func()
	// 添加头的方法
	AddHeader func(key, value string) bool
	SetHeader func(key, value string) bool
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
		app.Version = int(bytecoder.Version_VERSION_1)
		app.ReqId = msg.GetRequestId()
		app.Route = msg.GetRoute()
		app.Message = msg.GetBody()
		app.Header = msg.GetHeader()
		app.Method = msg.GetMethod()
		app.ResponseHeader = msg.GetResponseHeader()
	case bytecoder.Version_VERSION_2:
		msg, _ := coder.UnmarshalV2()
		app.Version = int(bytecoder.Version_VERSION_2)
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
		app.Version = int(bytecoder.Version_VERSION_CMD)
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

func (app *WSMessage) Group() string {
	return app.OrgHeader.Get(WsGroupHeader)
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

func (app *WSMessage) SendProto() bool {
	coder := bytecoder.ShowProtoFile()
	if coder != nil {
		coder.Gzip()
		coder.EncodeWS()
		return app.Send(coder)
	}

	return false
}

func (app *WSMessage) IsAccept() bool {
	ok := app.OrgHeader.Get(WsStatusHeader)
	return ok == "accept"
}

func (app *WSMessage) Status() LimitStatus {
	ok := app.OrgHeader.Get(WsStatusHeader)
	if ok == "accept" {
		return LimitAccept
	} else if ok == "wait" {
		return LimitWait
	} else {
		return LimitReject
	}
}

func (app *WSMessage) SetAcceptMode() {
	app.DelHeader(WsStatusHeader)
	app.AddHeader(WsStatusHeader, "accept")
	app.SendResponseCmd(bytecoder.MsgLocalCmd_MSG_LOCAL_CMD_WS_ACCEPT, []byte("连接成功"), nil)
}

func (app *WSMessage) SetWaitMode(self int64, total int64) {
	info := bytecoder.MessageWaitInfo{
		Self:  self,
		Total: total,
	}

	bt, _ := proto.Marshal(&info)
	app.AddHeader(WsStatusHeader, "wait")
	app.SendResponseCmd(bytecoder.MsgLocalCmd_MSG_LOCAL_CMD_WS_WAIT, bt, nil)
}

// 获取额外注入的消息头
func (app *WSMessage) GetAllHeader() http.Header {
	hd := http.Header{}
	for k, v := range app.OrgHeader {
		if !strings.Contains(k, PrefixLocalHeader) {
			hd[k] = v
		}
	}

	for k, v := range app.Header {
		hd[k] = []string{v}
	}

	return hd
}

func (app *WSMessage) WaitResponse(self int64, total int64) {
	info := bytecoder.MessageWaitInfo{
		Self:  self,
		Total: total,
	}

	bt, _ := proto.Marshal(&info)
	app.SendResponseCmd(bytecoder.MsgLocalCmd_MSG_LOCAL_CMD_WS_RESP, bt, nil)
}

func (app *WSMessage) SetCloseMode(msg string) {
	app.SendResponseCmd(bytecoder.MsgLocalCmd_MSG_LOCAL_CMD_WS_CLOSE, []byte(msg), nil)
	app.Close()
}
