package websocket

import (
	"Orion_Live/pkg/logger"
	"sync"
)

// Hub管理所有的弹幕房间
type Hub struct {
	rooms map[uint64]*Room // key是videoId, 对应自己的Room
	mu    sync.Mutex       // 对于低频、单一的事件，用mutex性能更高更方便

	unregisterRoom chan *Room // 接收需要销毁的空房间
}

// 建立Hub：rooms是map[string]*Room,需要初始化，mu不用初始化，零值本身就可以使用
func NewHub() *Hub {
	return &Hub{
		rooms:          make(map[uint64]*Room),
		unregisterRoom: make(chan *Room),
	}
}
func (h *Hub) Run() {
	for room := range h.unregisterRoom {
		delete(h.rooms, room.VideoID)
		logger.Log.Infof("视频ID为 %d 的视频房间由于用户都已离开，已经从Hub中移除", room.VideoID)
	}
}

// 获取或创建一个视频的房间：1、如果Hub里面能找到房间，则返回正在运行的房间 2、否则，NewRoom()创建房间，并运行这个房间 3、房间名加入到Hub中，返回新建的房间
func (h *Hub) GetOrCreateRoom(videoID uint64) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, ok := h.rooms[videoID]; ok {
		return room
	}
	room := NewRoom(h, videoID)
	go room.Run() // 为新房间启动一个独立的goroutine
	h.rooms[videoID] = room
	logger.Log.Infof("视频ID为 %d 的房间已被创建.", videoID)
	return room
}
