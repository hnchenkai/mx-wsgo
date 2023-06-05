package mxwsgo

import (
	"fmt"
	"log"
	"net/http"

	"github.com/hnchenkai/mx-wsgo/bytecoder"
	"github.com/hnchenkai/mx-wsgo/closeSingal"
)

type Cmd string

const (
	CmdMessage Cmd = "message"
	CmdAccept  Cmd = "accept"
	CmdClose   Cmd = "close"
	CmdCmd     Cmd = "cmd"
)

type Dispather func(cmd Cmd, msg *WSMessage)
type IServer interface {
	// 发送数据
	Send(int64, []byte) bool
	// 广播
	Broadcast([]byte)
	// 启动服务
	Run()
	// 接入http服务
	ServeWs(http.ResponseWriter, *http.Request, http.Header)
	// 断开服务
	Unregister(int64)

	// 接收到消息后，消息分派给具体的用户处理协程
	Dispatch(string, int64, Cmd, []byte, http.Header)
}

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type ServerUnit struct {
	// Registered clients.
	clients map[int64]*Connection

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Connection

	// Unregister requests from clients.
	unregister chan int64

	genId int64

	fclose closeSingal.CloseSingal

	dispatch Dispather

	needInitPb bool
}

func NewServerUnit(dispatcher Dispather, initPb ...bool) *ServerUnit {
	if len(initPb) == 0 {
		initPb = []bool{false}
	}
	return &ServerUnit{
		broadcast:  make(chan []byte),
		register:   make(chan *Connection),
		unregister: make(chan int64),
		clients:    make(map[int64]*Connection),
		genId:      0,
		dispatch:   dispatcher,
		needInitPb: initPb[0],
	}
}

func (h *ServerUnit) Close() {
	h.fclose.Close()
}

// 这个是负责实现链接管理，断开链接，广播的转发
func (h *ServerUnit) Run() {
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
			go h.Dispatch(client.host, client.Id, CmdAccept, nil, client.header)
		case clientId := <-h.unregister:
			if client, ok := h.clients[clientId]; ok {
				delete(h.clients, clientId)
				close(client.send)
				go h.Dispatch(client.host, client.Id, CmdClose, nil, client.header)
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
func (h *ServerUnit) Unregister(clientId int64) {
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

func (h *ServerUnit) Send(clientId int64, message []byte) bool {
	client, ok := h.clients[clientId]
	if ok {
		client.send <- message
		return true
	} else {
		return false
	}
}

// 添加head信息
func (h *ServerUnit) AddHeader(clientId int64, key string, value string) bool {
	client, ok := h.clients[clientId]
	if !ok {
		return false
	}
	client.header.Add(key, value)
	return true
}

// 删除head信息
func (h *ServerUnit) DelHeader(clientId int64, key string) bool {
	client, ok := h.clients[clientId]
	if !ok {
		return false
	}
	client.header.Del(key)
	return true
}

// 可以挂载到一个http服务上去,从http升级到https extHeader额外携带的信息
func (h *ServerUnit) ServeWs(w http.ResponseWriter, r *http.Request, extHeader http.Header) {
	// 这里最好做一个权限校验，判断是否可以链接
	conn, err := upgrader.Upgrade(w, r, http.Header{
		"Sec-Websocket-Protocol": r.Header.Values("Sec-Websocket-Protocol"),
	})
	if err != nil {
		log.Println(err)
		return
	}
	opt := defaultOptions()
	opt.byteType = 2
	client := &Connection{
		Id:      h.nextId(),
		hub:     h,
		conn:    conn,
		send:    make(chan []byte, 256),
		options: opt,
		host:    r.Host,
		header:  extHeader,
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

func (h *ServerUnit) doDispatch(cmd Cmd, msg *WSMessage) {
	if h.dispatch != nil {
		h.dispatch(cmd, msg)
		return
	}
}

// 分发消息
func (h *ServerUnit) Dispatch(host string, clientId int64, cmd Cmd, message []byte, header http.Header) {
	msg := &WSMessage{
		ClientId:  clientId,
		Host:      host,
		OrgHeader: header,
		Send: func(message []byte) bool {
			return h.Send(clientId, message)
		},
		Close: func() {
			h.Unregister(clientId)
		},
		AddHeader: func(key string, value string) bool {
			return h.AddHeader(clientId, key, value)
		},
		DelHeader: func(key string) bool {
			return h.DelHeader(clientId, key)
		},
	}
	switch cmd {
	case CmdMessage:
		// 这里要么pass掉，要么回复一个错误消息
		if err := msg.FromPb(message); err != nil {
			msg.SendError(http.StatusBadRequest, err.Error(), nil)
			return
		}
		if msg.Version == int(bytecoder.Version_VERSION_CMD) {
			h.doDispatch(CmdCmd, msg)
		} else if msg.IsAccept() {
			h.doDispatch(cmd, msg)
		} else {
			// 回复一个消息，告诉客户端需要等待接入
			msg.SendError(http.StatusBadRequest, "need accept", nil)
		}
	case CmdAccept:
		// 先发起一个把pb协议推送出去的命令
		if h.needInitPb {
			msg.sendProto()
		}
		h.doDispatch(cmd, msg)
	case CmdClose:
		// 这里把send无效化掉
		msg.Send = func(message []byte) bool {
			// 这里就不发送消息了
			fmt.Println("CmdClose的时候不要发送消息了")
			return false
		}
		h.doDispatch(cmd, msg)
	}

}
