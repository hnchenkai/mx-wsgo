package mxwsgo_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"testing"
	"time"

	mxwsgo "github.com/hnchenkai/mx-wsgo"
	"github.com/hnchenkai/mx-wsgo/bytecoder"
)

func TestClose(t *testing.T) {
	fmt.Println(os.Getpid())
	// 提供排队服务
	unit := mxwsgo.NewServerUnit(func(cmd mxwsgo.Cmd, msg *mxwsgo.WSMessage) {
		switch cmd {
		case mxwsgo.CmdAccept:
			// 发起一个断开消息
			msg.SendError(int32(bytecoder.MsgLocalCmd_MSG_LOCAL_CMD_WS_CLOSE), "网络断开", nil)
			msg.Close()
		case mxwsgo.CmdClose:
		case mxwsgo.CmdMessage:
		}
	}, false)
	go unit.Run()
	svr := http.Server{
		Addr: "0.0.0.0:8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/ws":
				unit.ServeWs(w, r, http.Header{})
			default:
			}

		}),
	}
	if err := svr.ListenAndServe(); err != nil {
		t.Fatal(err)
	}
	unit.Close()
}

func TestAccept(t *testing.T) {
	fmt.Println(os.Getpid())
	// 提供排队服务
	unit := mxwsgo.NewServerUnit(func(cmd mxwsgo.Cmd, msg *mxwsgo.WSMessage) {
		switch cmd {
		case mxwsgo.CmdAccept:
			// 发起一个断开消息
			msg.SetAcceptMode()
		case mxwsgo.CmdClose:
		case mxwsgo.CmdMessage:
		}
	}, false)
	go unit.Run()
	svr := http.Server{
		Addr: "0.0.0.0:8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/ws":
				unit.ServeWs(w, r, http.Header{})
			default:
			}

		}),
	}
	if err := svr.ListenAndServe(); err != nil {
		t.Fatal(err)
	}
	unit.Close()
}

func toAccept(msg *mxwsgo.WSMessage, second time.Duration) {
	<-time.NewTicker(second * time.Second).C
	msg.SetAcceptMode()
}

var WaitCountMap = make(map[int64]int64)

type WaitUnit struct {
	Self  int64 `json:"self"`
	Total int64 `json:"total"`
}

func (w *WaitUnit) Byte() []byte {
	b, _ := json.Marshal(w)
	return b
}

func toWaitInfo(clientId int64) *WaitUnit {
	c, ok := WaitCountMap[clientId]
	total := 10
	if !ok {
		c = time.Now().Unix() + int64(rand.Intn(total))
		WaitCountMap[clientId] = c
	}
	info := &WaitUnit{
		Self:  c - time.Now().Unix(),
		Total: int64(total),
	}
	return info
}

func TestWait(t *testing.T) {
	fmt.Println(os.Getpid())

	// 提供排队服务
	unit := mxwsgo.NewServerUnit(func(cmd mxwsgo.Cmd, msg *mxwsgo.WSMessage) {
		switch cmd {
		case mxwsgo.CmdAccept:
			// 发起一个等待消息
			wt := toWaitInfo(msg.ClientId)
			if wt.Self > 0 {
				msg.SetWaitMode(wt.Self, wt.Total)
				go toAccept(msg, time.Duration(wt.Self))
			} else {
				msg.SetAcceptMode()
			}
		case mxwsgo.CmdClose:
		case mxwsgo.CmdCmd:
			switch msg.Cmd {
			case bytecoder.MsgLocalCmd_MSG_LOCAL_CMD_WS_REQ:
				wt := toWaitInfo(msg.ClientId)
				msg.WaitResponse(wt.Self, wt.Total)
			default:
				// 其他消息
			}
		case mxwsgo.CmdMessage:
			// 判断是否已经成功开启了
		}
	}, false)
	go unit.Run()
	svr := http.Server{
		Addr: "0.0.0.0:8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/ws":
				unit.ServeWs(w, r, http.Header{})
			default:
			}

		}),
	}
	if err := svr.ListenAndServe(); err != nil {
		t.Fatal(err)
	}
	unit.Close()
}
