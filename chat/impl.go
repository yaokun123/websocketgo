package chat

import (
	"bytes"
	"errors"
	"github.com/gorilla/websocket"
	"sync"
)

//封装websocket
type Connection struct {
	wsConnect *websocket.Conn
	inChan    chan []byte
	outChan   chan []byte
	closeChan chan byte
	cliMap    map[string]map[string]*Connection
	room      string

	mutex    sync.Mutex //对closeChan关闭上锁
	isClosed bool       //防止closeChan被关闭多次
}

func InitConnection(wsConn *websocket.Conn, cliMap map[string]map[string]*Connection, room string) (conn *Connection, e error) {
	conn = &Connection{
		wsConnect: wsConn,
		inChan:    make(chan []byte, 1000),
		outChan:   make(chan []byte, 1000),
		closeChan: make(chan byte, 1),
		cliMap:    cliMap,
		room:      room,
	}

	//启动读协程
	go conn.readLoop()

	//启动写协程
	go conn.writeLoop()

	return
}

func (conn *Connection) ReadMessage() (data []byte, err error) {
	select {
	case data = <-conn.inChan:
	case <-conn.closeChan:
		err = errors.New("connection is closed")
	}
	return
}

func (conn *Connection) WriteMessage(data []byte) (err error) {
	select {
	case conn.outChan <- data:
	case <-conn.closeChan:
		err = errors.New("connection is closed")
	}
	return
}

func (conn *Connection) Close() {
	//线程安全，可以多次调用
	conn.wsConnect.Close()

	//利用标记，让closeChan只关闭一次
	conn.mutex.Lock()

	if !conn.isClosed {
		close(conn.closeChan)
		conn.isClosed = true

		//告诉大家该用户退出
		for _, others := range conn.cliMap[conn.room] {
			if others == conn {
				continue
			}
			select {
			case others.inChan <- BytesCombine([]byte(conn.wsConnect.RemoteAddr().String() + "退出聊天室")): //读取到数据之后写入inChan通道中
				//case val.inChan <- data:		//读取到数据之后写入inChan通道中
			case <-others.closeChan:
				others.Close()
				continue
			}

		}
		//将关闭的资源从map中删除
		delete(conn.cliMap[conn.room], conn.wsConnect.RemoteAddr().String())
	}
	conn.mutex.Unlock()
}

//内部实现
func (conn *Connection) readLoop() {
	for {
		_, data, err := conn.wsConnect.ReadMessage() //读取来自websocket协议的信息
		if err != nil {
			conn.Close()
			return
		}

		//阻塞在这里，等待inChan有空闲位置
		for _, val := range conn.cliMap[conn.room] {
			if val == conn {
				continue
			}
			select {
			case val.inChan <- BytesCombine([]byte(conn.wsConnect.RemoteAddr().String()+"在聊天室["+conn.room+"]对大家说："), data): //读取到数据之后写入inChan通道中
			//case val.inChan <- data:		//读取到数据之后写入inChan通道中
			case <-val.closeChan:
				val.Close()
				continue
			}
		}

	}
}

func BytesCombine(pBytes ...[]byte) []byte {
	return bytes.Join(pBytes, []byte(""))
}

func (conn *Connection) writeLoop() {
	var data []byte
	for {
		select {
		case data = <-conn.outChan: //读取outChan通道中的信息
		case <-conn.closeChan:
			conn.Close()
			return
		}
		err := conn.wsConnect.WriteMessage(websocket.TextMessage, data) //将读到的信息发送给客户端

		if err != nil {
			conn.Close()
			return
		}

		if string(data) == "otherConnection" {
			conn.Close()
			return
		}
	}
}
