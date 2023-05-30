package mxwsgo_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	mxwsgo "github.com/hnchenkai/mx-wsgo"
	"github.com/hnchenkai/mx-wsgo/bytecoder"
)

func TestXxx(t *testing.T) {
	fmt.Println(os.Getpid())
	// 提供排队服务
	unit := mxwsgo.NewServerUnit(func(cmd mxwsgo.Cmd, msg *mxwsgo.WSMessage) {
		switch cmd {
		case mxwsgo.CmdAccept:
			// 发起一个断开消息
			msg.SendResponse(int32(bytecoder.MsgLocalCmd_MSG_LOCAL_CMD_WS_CLOSE), []byte("网络断开"), nil)
			msg.Close()
		case mxwsgo.CmdClose:
		case mxwsgo.CmdMessage:
		}
	}, false)
	go unit.Run()
	// 提供链接通信服务
	unit2 := mxwsgo.NewServerUnit(func(cmd mxwsgo.Cmd, msg *mxwsgo.WSMessage) {

	}, false)
	go unit2.Run()
	svr := http.Server{
		Addr: "0.0.0.0:8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/ws":
				unit.ServeWs(w, r, http.Header{})
			case "/ws2":
				unit2.ServeWs(w, r, http.Header{})
			default:
				body, _ := io.ReadAll(r.Body)
				fmt.Println(string(body))
				bt, _ := json.Marshal(map[string]interface{}{
					"code": 0,
				})
				w.Write(bt)
			}

		}),
	}
	if err := svr.ListenAndServe(); err != nil {
		t.Fatal(err)
	}
	unit.Close()
}
