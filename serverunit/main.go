package serverunit

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/hnchenkai/mx-wsgo/bytecoder"
	"github.com/hnchenkai/mx-wsgo/domain"
	"github.com/hnchenkai/mx-wsgo/limitcount"
	"github.com/hnchenkai/mx-wsgo/wsmessage"
)

type TGroup string
type Dispather func(cmd wsmessage.Cmd, msg *wsmessage.WSMessage)

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type ServerUnit struct {
	// Registered clients.
	clients map[string]*Connection

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Connection

	// Unregister requests from clients.
	unregister chan string

	genId int64

	fclose domain.CloseSingal

	dispatch Dispather

	// needInitPb bool

	limitcount *limitcount.LimitCountUnit
}

/**
 * @brief:  生成一个服务单元
 * @param:  dispatcher 分发器
 * @param:  limitOption 限流配置
 * @return: *ServerUnit
 */
func NewServerUnit(dispatcher Dispather, limitOption *limitcount.LimitOption) *ServerUnit {
	unit := &ServerUnit{
		broadcast:  make(chan []byte),
		register:   make(chan *Connection),
		unregister: make(chan string),
		clients:    make(map[string]*Connection),
		genId:      0,
		dispatch:   dispatcher,
		// needInitPb: initPb[0],
	}

	unit.limitcount = limitcount.NewLimitCountUnit(unit.GetConnMessage)

	if limitOption != nil {
		unit.limitcount.Init(limitOption)
	}

	return unit
}

func (h *ServerUnit) Close() {
	// 关闭升降级定时器
	h.limitcount.Close()
	// 关闭ws链接
	h.fclose.Close()
}

// 这个是负责实现链接管理，断开链接，广播的转发
func (h *ServerUnit) Run() {
	h.limitcount.Run()
	defer func() {
		for clientId, client := range h.clients {
			close(client.send)
			delete(h.clients, clientId)
		}
		h.fclose.Defer()
	}()
	for {
		if h.fclose.Wait() {
			break
		}
		select {
		case client := <-h.register:
			h.clients[client.Id] = client
			go h.Dispatch(client.host, client.Id, wsmessage.CmdAccept, nil, client.header)
		case clientId := <-h.unregister:
			if client, ok := h.clients[clientId]; ok {
				delete(h.clients, clientId)
				close(client.send)
				go h.Dispatch(client.host, client.Id, wsmessage.CmdClose, nil, client.header)
			}
		case message := <-h.broadcast:
			for clientId, client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, clientId)
				}
			}
		}
	}
}

// 这个是负责消息广播的
func (h *ServerUnit) Broadcast(message []byte) {
	h.broadcast <- message
}

// 这个是关闭链接的
func (h *ServerUnit) Unregister(clientId string) {
	h.unregister <- clientId
}

// 这个是生成
func (h *ServerUnit) nextId() int64 {
	var num int64 = 1
	if h.genId > (num << 62) {
		h.genId = 0
	}
	h.genId++

	return h.genId
}

func (h *ServerUnit) Send(clientId string, message []byte) bool {
	client, ok := h.clients[clientId]
	if ok {
		client.send <- message
		return true
	} else {
		return false
	}
}

// 添加head信息
func (h *ServerUnit) AddHeader(clientId string, key string, value string) bool {
	client, ok := h.clients[clientId]
	if !ok {
		return false
	}
	client.header.Add(key, value)
	return true
}

func (h *ServerUnit) SetHeader(clientId string, key string, value string) bool {
	client, ok := h.clients[clientId]
	if !ok {
		return false
	}
	client.header.Set(key, value)
	return true
}

// 删除head信息
func (h *ServerUnit) DelHeader(clientId string, key string) bool {
	client, ok := h.clients[clientId]
	if !ok {
		return false
	}
	client.header.Del(key)
	return true
}

// 可以挂载到一个http服务上去,从http升级到https
// header中 携带 Mx-Ws- 会被转发到ws的header中
func (h *ServerUnit) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 这里最好做一个权限校验，判断是否可以链接
	conn, err := upgrader.Upgrade(w, r, http.Header{
		"Sec-Websocket-Protocol": r.Header.Values("Sec-Websocket-Protocol"),
	})
	if err != nil {
		log.Println(err)
		return
	}

	prefix := r.Header.Get("Sec-Websocket-Accept")
	opt := defaultOptions()
	opt.byteType = 2
	client := &Connection{
		Id:      fmt.Sprintf("%s_%d", prefix, h.nextId()),
		hub:     h,
		conn:    conn,
		send:    make(chan []byte, 256),
		options: opt,
		host:    r.Host,
		header:  http.Header{},
	}

	for k, v := range r.Header {
		if strings.HasPrefix(k, wsmessage.PrefixProxyHeader) {
			k1 := strings.Replace(k, wsmessage.PrefixProxyHeader, "", 1)
			client.header[k1] = v
		}
	}

	for key, value := range r.Header {
		switch key {
		case "User-Agent":
		// case "Cache-Control":
		// case "Accept-Language":
		case "Accept-Encoding":
		default:
			continue
		}
		client.header[key] = value
	}

	h.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}

func (h *ServerUnit) doDispatch(cmd wsmessage.Cmd, msg *wsmessage.WSMessage) {
	if h.dispatch != nil {
		h.dispatch(cmd, msg)
		return
	}
}

func (h *ServerUnit) msgBind(msg *wsmessage.WSMessage) *wsmessage.WSMessage {
	clientId := msg.ClientId
	msg.Send = func(message []byte) bool {
		return h.Send(clientId, message)
	}
	msg.Close = func() {
		h.Unregister(clientId)
	}
	msg.AddHeader = func(key string, value string) bool {
		return h.AddHeader(clientId, key, value)
	}
	msg.DelHeader = func(key string) bool {
		return h.DelHeader(clientId, key)
	}
	msg.SetHeader = func(key string, value string) bool {
		return h.SetHeader(clientId, key, value)
	}

	return msg
}

// 获取一个链接对象，负责主动发送消息
func (h *ServerUnit) GetConnMessage(clientId string) *wsmessage.WSMessage {
	client, ok := h.clients[clientId]
	if !ok {
		return nil
	}
	return h.msgBind(&wsmessage.WSMessage{
		ClientId:  clientId,
		Host:      client.host,
		OrgHeader: client.header,
	})
}

// 分发消息
func (h *ServerUnit) Dispatch(host string, clientId string, cmd wsmessage.Cmd, message []byte, header http.Header) {
	msg := h.msgBind(&wsmessage.WSMessage{
		ClientId:  clientId,
		Host:      host,
		OrgHeader: header,
	})
	switch cmd {
	case wsmessage.CmdMessage:
		// 这里要么pass掉，要么回复一个错误消息
		if err := msg.FromPb(message); err != nil {
			msg.SendError(http.StatusBadRequest, err.Error(), nil)
			return
		}
		if msg.Version == int(bytecoder.Version_VERSION_CMD) {
			switch msg.Cmd {
			case bytecoder.MsgLocalCmd_MSG_LOCAL_CMD_WS_REQ:
				self, total := h.WaitUnitInfo(context.Background(), msg.Group(), msg.ClientId)
				msg.WaitResponse(self, total)
			default:
				// 其他消息
				h.doDispatch(wsmessage.CmdCmd, msg)
			}
		} else if msg.IsAccept() {
			h.doDispatch(cmd, msg)
		} else {
			// 回复一个消息，告诉客户端需要等待接入
			msg.SendError(http.StatusBadRequest, "need accept", nil)
		}
	case wsmessage.CmdAccept:
		// 进行一个是否限制链接的判断
		if status, err := h.limitcount.MakeConnStatus(msg.Group(), msg.ClientId); err != nil {
			msg.SetCloseMode(err.Error())
		} else {
			switch status {
			case wsmessage.LimitAccept:
				msg.SetAcceptMode()
				h.doDispatch(cmd, msg)
			case wsmessage.LimitWait:
				self, total := h.WaitUnitInfo(context.Background(), msg.Group(), msg.ClientId)
				msg.SetWaitMode(self, total)
				h.doDispatch(wsmessage.CmdWait, msg)
			case wsmessage.LimitReject:
				msg.SetCloseMode("too many requests")
				h.doDispatch(wsmessage.CmdReject, msg)
			}
		}
	case wsmessage.CmdClose:
		// 这里把send无效化掉
		msg.Send = func(message []byte) bool {
			// 这里就不发送消息了
			fmt.Println("CmdClose的时候不要发送消息了")
			return false
		}
		h.limitcount.CloseConnStatus(msg.Group(), msg.ClientId, msg.Status())
		h.doDispatch(cmd, msg)
	}

}

// WaitUnitInfo 获取等待队列信息
func (h *ServerUnit) WaitUnitInfo(ctx context.Context, limitkey string, clientId string) (int64, int64) {
	return h.limitcount.WaitUnitInfo(ctx, limitkey, clientId)
}
