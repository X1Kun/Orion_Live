package websocket

import (
	"Orion_Live/internal/metrics"

	"github.com/gorilla/websocket"
)

// Client代表一个连接到服务器的用户
type Client struct {
	// Username string
	conn *websocket.Conn // 用户的WebSocket连接
	send chan []byte     // 一个带缓冲的channel，用于向该用户发送消息
	room *Room           // 该用户所在的房间
}

func NewClient(conn *websocket.Conn, room *Room) *Client {
	return &Client{
		// Username: Username,
		conn: conn,
		send: make(chan []byte, 256),
		room: room,
	}
}

// 从WebSocket连接中读取用户发来的消息：1、for循环中通过c.conn.ReadMessage()不断监听用户发来的消息 2、如果收到消息，则将其存入c.room.broadcast这个channel中 3、如果用户的连接断开，则会通过注销用户，并将用户的Websocket连接断开
func (c *Client) ReadPump() {
	defer func() {
		c.room.unregister <- c // 从房间注销，从房间的map中删除，并且将其send的channel关闭
		c.conn.Close()         // 删除本身的WebSocket连接
		// 在连接删除之后，也相应的在prometheus减少连接数
		metrics.ActiveWebsocketConnections.Dec()
	}()
	// 当goroutine的工作是主动地、重复地去做某件事，并且生命周期由它正在处理的资源来决定，for循环最直观
	for {
		// 通过conn.ReadMessage()不断监听客户端发送的消息（弹幕），阻塞等待
		_, message, err := c.conn.ReadMessage()
		// 用户关闭浏览器标签页，网络异常断开，发送不符合WebSocket协议规范的数据时，会抛出Error
		if err != nil {
			// 连接断开
			break
		}
		// 将收到的消息交给room的broadcast channel（无缓存）处理
		c.room.broadcast <- message
	}
}

// 消息广播：阻塞读取c.send这个channel，如果收到消息，则取出消息并通过c.conn.WriteMessage写入WebSocket连接，发送给用户
func (c *Client) WritePump() {
	// 健壮性设计，ReadPump()和WritePump()都有c.conn.Close()，保证资源释放
	defer c.conn.Close()
	for message := range c.send {
		// 通过conn.WriteMessage()将读到的消息发送给客户端
		c.conn.WriteMessage(websocket.TextMessage, message)
	}
}
