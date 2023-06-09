package mxwsgo_test

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	mxwsgo "github.com/hnchenkai/mx-wsgo"
	"github.com/hnchenkai/mx-wsgo/wsmessage"
)

func TestClose(t *testing.T) {
	fmt.Println(os.Getpid())
	// 提供排队服务
	unit := mxwsgo.NewServerUnit(func(cmd wsmessage.Cmd, msg *wsmessage.WSMessage) {
		switch cmd {
		case wsmessage.CmdAccept:
			// 发起一个断开消息
			msg.SetCloseMode("test")
		case wsmessage.CmdClose:
		case wsmessage.CmdMessage:
		}
	}, nil)
	go unit.Run()
	svr := http.Server{
		Addr: "0.0.0.0:8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/ws":
				unit.ServeHTTP(w, r)
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
	unit := mxwsgo.NewServerUnit(func(cmd wsmessage.Cmd, msg *wsmessage.WSMessage) {
		switch cmd {
		case wsmessage.CmdAccept:
		case wsmessage.CmdClose:
		case wsmessage.CmdMessage:
		}
	}, nil)
	go unit.Run()
	svr := http.Server{
		Addr: "0.0.0.0:8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/ws":
				mxwsgo.SetGroup(r.Header, "test")
				unit.ServeHTTP(w, r)
			default:
			}

		}),
	}
	if err := svr.ListenAndServe(); err != nil {
		t.Fatal(err)
	}
	unit.Close()
}

func TestWait(t *testing.T) {
	fmt.Println(os.Getpid())
	// 提供排队服务
	unit := mxwsgo.NewServerUnit(func(cmd wsmessage.Cmd, msg *wsmessage.WSMessage) {
		switch cmd {
		case wsmessage.CmdAccept:

		case wsmessage.CmdClose:
		case wsmessage.CmdCmd:

		case wsmessage.CmdMessage:
			// 判断是否已经成功开启了
		}
	}, &mxwsgo.LimitOption{
		ReadyLimitFunc: func(key string) int {
			return 0
		},
		WaitLimitFunc: func(key string) int {
			panic("not implement")
			return 1
		},
	})
	go unit.Run()
	svr := http.Server{
		Addr: "0.0.0.0:8080",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/ws":
				mxwsgo.SetGroup(r.Header, "test")
				unit.ServeHTTP(w, r)
			default:
			}

		}),
	}
	if err := svr.ListenAndServe(); err != nil {
		t.Fatal(err)
	}
	unit.Close()
}
