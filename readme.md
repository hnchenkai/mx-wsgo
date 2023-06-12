# websocket-golang

这个是 websocket 的服务端封装  
支持排队等待模式  
数据使用 ws+pb+gzip

## 开启 ws 模式

```
func main(){
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
		panic(err)
	}
	unit.Close()
}
```

## 开启 limit 模式

需要额外提供 redis 链接才能支持分布式
