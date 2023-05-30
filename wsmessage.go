package mxwsgo

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hnchenkai/mx-wsgo/bytecoder"
)

type WSMessage struct {
	// 请求对应的host
	Host string
	// 连接id
	ClientId int64 `json:"clientId,omitempty"`
	// 原始头
	OrgHeader http.Header
	// 发送消息的方法
	Send func(message []byte) bool

	Close func()

	ReqId   int64             `json:"requestId"`
	Method  string            `json:"method"`
	Route   string            `json:"route"`
	Message []byte            `json:"message"`
	Header  map[string]string `json:"header"`

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
		app.ReqId = msg.GetRequestId()
		app.Route = msg.GetRoute()
		app.Message = msg.GetBody()
		app.Header = msg.GetHeader()
		app.Method = msg.GetMethod()
		app.ResponseHeader = msg.GetResponseHeader()
	case bytecoder.Version_VERSION_2:
		msg, _ := coder.UnmarshalV2()
		app.ReqId = msg.GetRequestId()
		app.Route = fmt.Sprintf("/%d", msg.Cmd)
		app.Message = msg.GetBody()
		app.Method = "POST"
		app.Header = msg.GetHeader()
		app.ResponseHeader = msg.GetResponseHeader()
	case bytecoder.Version_VERSION_0_UNSPECIFIED:
		// 收到一个错误信息，我也是真xxx了
		return fmt.Errorf("error message")
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

func (app *WSMessage) sendProto() bool {
	coder := bytecoder.ShowProtoFile()
	if coder != nil {
		coder.Gzip()
		coder.EncodeWS()
		return app.Send(coder)
	}

	return false
}
