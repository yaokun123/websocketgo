package main

import (
	"github.com/gorilla/websocket"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"websocketgo/chat"
)

var(
	upgrader = websocket.Upgrader{
		//允许跨域
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	cliMap = make(map[string]map[string]*chat.Connection)

	ipMap = make(map[string]map[string]string)
)


func wsHandler(w http.ResponseWriter,r *http.Request)  {
	//w.Write([]byte("hello"))
	rooms, ok := r.URL.Query()["room"]
	var room string

	if !ok {
		room = "1"		//如果传入的房间号不存在，则默认是第一个房间
	}else{
		room = rooms[0]
	}



	//完成ws协议的握手操作
	//Upgrade:websocket
	wsConn,err := upgrader.Upgrade(w,r,nil)
	if err != nil{
		return
	}

	ipAddress := wsConn.RemoteAddr().String() //获取远程客户端地址 127.0.0.1:61798


	//查看客户端是否已经在ipMap中，如果在则要通知已经存在的链接断掉
	ipArr := strings.Split(ipAddress,":")
	ip := ipArr[0]

	key,ok := ipMap[room][ip]
	if ok {				//如果已经存在
		if cliMap[room][key] != nil{
			cliMap[room][key].WriteMessage([]byte("otherConnection"))//向客户端发送一条约定的断开链接信息
		}
		delete(ipMap[room],ip)
	}


	conn,err := chat.InitConnection(wsConn,cliMap,room)

	/*if err != nil{
		conn.Close()
		return
	}*/


	//握手完成之后将连接保存到map中(注意使用二维map的规则)

	//这个地方有个bug（进入同一个房间的人会覆盖之前的
	_,existsCliMap := cliMap[room]
	if(existsCliMap){
		  cliMap[room][ipAddress] = conn
	}else{
		cliMapTmp := make(map[string]*chat.Connection)
		cliMapTmp[ipAddress] = conn
		cliMap[room] = cliMapTmp
	}

	_,existsIpMap := ipMap[room]
	if(existsIpMap){
		ipMap[room][ip] = ipAddress
	}else{
		ipMapTmp := make(map[string]string)
		ipMapTmp[ip] = ipAddress
		ipMap[room] = ipMapTmp
	}


	//启动协程，不断发送消息
	/*go func() {
		for{
			err2 := conn.WriteMessage([]byte("heartbeat"))
			if err2 != nil{
				return
			}
			time.Sleep(1*time.Second)
		}
	}()*/

	for{
		data,err3 := conn.ReadMessage()
		if err3 != nil{
			conn.Close()
			return
		}

		err4 := conn.WriteMessage(data)
		if err4 != nil{
			conn.Close()
			return
		}
	}

}

func main()  {
	http.HandleFunc("/ws",wsHandler)
	http.HandleFunc("/open",openHandler)
	http.HandleFunc("/chat",chatHandler)
	http.ListenAndServe("0.0.0.0:7777",nil)
}

func openHandler(w http.ResponseWriter,r *http.Request)  {
	f,err := os.Open("./main/open.html")
	defer f.Close()
	if err != nil{
		w.Write([]byte("file not found"))
		return
	}
	content,err2 := ioutil.ReadAll(f)
	if err2 != nil{
		w.Write([]byte("file read fail"))
		return
	}
	w.Write([]byte(content))
}

func chatHandler(w http.ResponseWriter,r *http.Request)  {

	//f,err := os.Open("/Users/yaok/Applications/Go/src/mygo/main/chat.html")
	rooms, ok := r.URL.Query()["room"]

	var room string
	if !ok {
		room = "1"
	}else {
		room = rooms[0]
	}

	f,err := os.Open("./main/chat.html")
	defer f.Close()
	if err != nil{
		w.Write([]byte("file not found"))
		return
	}

	content,err2 := ioutil.ReadAll(f)
	if err2 != nil{
		w.Write([]byte("file read fail"))
		return
	}

	//替换content中的{{$wsUri}}
	content = []byte(strings.Replace(string(content),"{{$wsUri}}","'ws://192.168.0.105:7777/ws?room="+string(room)+"'",-1))
	w.Write([]byte(content))
}
