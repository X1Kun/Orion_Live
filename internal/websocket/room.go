package websocket

import "Orion_Live/pkg/logger"

// Room 代表一个视频的弹幕房间
type Room struct {
	VideoID    uint64
	clients    map[*Client]bool // 存储房间内所有客户端的集合
	broadcast  chan []byte      // 接收需要广播的消息，无缓冲
	register   chan *Client     // 接收新加入的客户端
	unregister chan *Client     // 接收需要离开的客户端

	hub *Hub // Room的管理者
}

func NewRoom(hub *Hub, VideoID uint64) *Room {
	return &Room{
		VideoID: VideoID,
		// 用map[*Client]bool模拟set
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		hub:        hub,
	}
}

// 将client送入r.register这个channel中，等待r.Run()执行case client := <-r.register:
func (r *Room) RegisterClient(c *Client) {
	r.register <- c
}

// CSP（Communicating Sequential Processes）不要通过共享内存来通信，而要通过通信来共享内存
// map在Go中是并发不安全的，select-for可以一个接一个地处理map的并发不安全问题
// 房间启动！1、for+select组合，循环判断r.register，r.unregister，r.broadcast三个channel中是否有东西进来，执行注册用户，注销用户，给用户广播消息操作
func (r *Room) Run() {
	for {
		// select会伪随机地执行case，不会因为case的顺序饿死后面的，被处理的机会均等
		select {
		// 加入聊天室：r.register中收到handler传来的client，将client加入r.clients这个map中
		case client := <-r.register:
			// 新客户端加入map
			r.clients[client] = true
			logger.Log.WithField("client_addr", client).WithField("room_size", len(r.clients)).Info("客户进入直播间")
		// 离开聊天室：确保用户的存在性后，将用户从map中删除，并关闭用户的client.send这个channel
		case client := <-r.unregister:
			if _, ok := r.clients[client]; ok {
				delete(r.clients, client)
				close(client.send)
				// 使用 WithFields 来添加结构化的上下文信息
				logger.Log.WithField("client_addr", client).WithField("room_size", len(r.clients)).Info("客户端已注销")
			}
			if len(r.clients) == 0 {
				// 房间空了，通知Hub来销毁自己
				r.hub.unregisterRoom <- r
				return
			}
		// 广播消息
		case message := <-r.broadcast:
			// 非阻塞发送：channel+select
			for client := range r.clients {
				select {
				case client.send <- message:
				default:
					// 如果发送失败（channel满了），则认为该客户端已掉线
					// 上面是先删除map中用户再关闭chan，这个是先关闭chan再删除map有什么区别呢？如果这时候client.send的chan已经没了，再delete会发生什么？
					delete(r.clients, client)
					close(client.send)
				}
			}
		}
	}
}
