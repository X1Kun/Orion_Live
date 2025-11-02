// internal/handler/websocket_handler.go (新建)

package handler

import (
	"net/http"
	"strconv"

	"Orion_Live/internal/metrics"
	"Orion_Live/internal/service"
	InterWebsocket "Orion_Live/internal/websocket"
	"Orion_Live/pkg/logger"

	"github.com/gorilla/websocket"

	"github.com/gin-gonic/gin"
)

type WebSocketHandler struct {
	Hub          *InterWebsocket.Hub
	VideoService service.VideoService // 依赖 VideoService 接口
}

func NewWebSocketHandler(hub *InterWebsocket.Hub, videoService service.VideoService) *WebSocketHandler {
	return &WebSocketHandler{
		Hub:          hub,
		VideoService: videoService,
	}
}

var upgrader = websocket.Upgrader{
	// 设置读、写缓冲区大小
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// 安全检查，浏览器出于安全考虑，默认不允许a.com网站的页面去连接b.com的WebSocket服务器（这叫“跨域”）
	CheckOrigin: func(r *http.Request) bool {
		// 暂时允许任何来源的连接
		return true
	},
}

// 对/api/v1/ws/videos/:video_id 的URL进行处理：1、解析URL的videoID参数，验证存在性 2、利用context中的请求c.Request以及我们定义的websocket.Upgrader将HTTP升级为websocket连接 3、创造videoID的room或获取已有room 4、为client创建与room的连接，并在room中注册client 5、启动client的读写goroutine
func (h *WebSocketHandler) ServeWs(c *gin.Context) {
	videoIDstr := c.Param("video_id")
	// 检查videoID的格式
	videoID, err := strconv.ParseUint(videoIDstr, 10, 64)
	if err != nil {
		sendErrorResponse(c, http.StatusBadRequest, "无效的视频ID")
		return
	}
	logCtx := logger.Log.WithField("video_id", videoID)
	logCtx.Info("开始处理查找视频请求")
	_, err = h.VideoService.GetVideoByID(videoID)
	if err != nil {
		// GetVideoByID失败通常意味着资源不存在
		logCtx.WithError(err).Warn("查找视频失败")
		sendErrorResponse(c, http.StatusNotFound, "视频不存在")
		return
	}
	// upgrader.Upgrade用来处理客户端的特殊的“执行升级”的HTTP请求，将HTTP升级为Websocket连接
	// c.Request是特殊的HTTP请求本身，c.Writer是Gin封装的、用于向客户端回信的工具，回复HTTP响应，则升级成功
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Log.WithError(err).Error("WebSocket升级失败")
		return
	}
	// prometheus监控Websocket数量，新建连接，则记录增加
	metrics.ActiveWebsocketConnections.Inc()
	// 为一个视频获取或者建立房间
	room := h.Hub.GetOrCreateRoom(videoID)
	// 专门为此客户端建立与视频的Client对象，并且建立了一个可以给客户端发消息的send通道（有缓冲）
	client := InterWebsocket.NewClient(conn, room)
	// room注册client
	room.RegisterClient(client)

	// 启动两个goroutine，client.WritePump()用于向客户端方向写消息，client.ReadPump()用于读取用户发来的消息
	go client.WritePump()
	go client.ReadPump()
}
