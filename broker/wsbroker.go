package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/quic-go/quic-go"
)

// WSBroker 代表 WebSocket 代理服务器
type WSBroker struct {
	ctx context.Context
	// WebSocket 升级器
	upgrader websocket.Upgrader

	// 连接管理
	clients    map[*ForwardChannel]bool
	clientsMux sync.RWMutex
}

// ForwardConfig 定义消息转发的配置
type ForwardChannel struct {
	conn   *websocket.Conn
	qConn  quic.Connection
	qStram quic.Stream
}

func (fc *ForwardChannel) stop() {
	if fc.conn != nil {
		fc.conn.Close()
	}
	if fc.qConn != nil {
		fc.qConn.CloseWithError(0, "Close to remote")
	}
	if fc.qStram != nil {
		fc.qStram.Close()
	}
}

// NewWSBroker 创建一个新的 WebSocket 代理服务器
func NewWSBroker(ctx context.Context) *WSBroker {
	return &WSBroker{
		ctx: ctx,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // 允许所有来源，生产环境中应该更严格
			},
		},
		clients: make(map[*ForwardChannel]bool),
	}
}

// Start 启动 WebSocket 代理服务器
func (b *WSBroker) Start() error {
	address := "0.0.0.0:2080"
	http.HandleFunc("/ws", b.handleWebSocket)
	useHttps := true

	log.Printf("WebSocket broker starting on %s (HTTPS: %v)", address, useHttps)

	if useHttps {
		// 使用HTTPS
		crt := "../config/broker.askdao.top_bundle.crt"
		key := "../config/broker.askdao.top.key"
		return http.ListenAndServeTLS(address, crt, key, nil)
	} else {
		// 使用HTTP
		return http.ListenAndServe(address, nil)
	}
}

// Stop 停止 WebSocket 代理服务器
func (b *WSBroker) Stop() {
	b.clientsMux.Lock()
	for conn := range b.clients {
		conn.stop()
	}
	b.clientsMux.Unlock()
}

// handleWebSocket 处理新的 WebSocket 连接
func (b *WSBroker) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 升级 HTTP 连接到 WebSocket
	conn, err := b.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	fc := &ForwardChannel{
		conn: conn,
	}

	// 注册新客户端
	b.clientsMux.Lock()
	b.clients[fc] = true
	b.clientsMux.Unlock()

	// 启动客户端的读写处理
	go b.handleClientMessages(fc)
}

// handleClientMessages 处理客户端消息
func (b *WSBroker) handleClientMessages(fc *ForwardChannel) {
	defer func() {
		b.clientsMux.Lock()
		delete(b.clients, fc)
		b.clientsMux.Unlock()
		fc.stop()
	}()
	// connect to remote
	err := fc.connect(b.ctx)
	if err == nil {
		fc.forward()
	}
}

func (fc *ForwardChannel) connect(ctx context.Context) error {
	// connect to remote
	qc := connectTo(ctx)
	if qc == nil {
		return errors.New("connect to remote failed")
	}
	qstr, err := qc.OpenStreamSync(ctx)
	if err != nil {
		log.Printf("Error opening stream: %v", err)
		qc.CloseWithError(1000, "Error connecting to remote")
		return err
	}

	fc.qConn = qc
	fc.qStram = qstr
	return nil
}

func (fc *ForwardChannel) forward() {
	ch := make(chan int)
	go func() {
		// copy from remote to local
		_, err := io.Copy(fc.qStram, fc.conn.NetConn())
		if err != nil {
			log.Printf("Error copying from local to remote: %v", err)
		}
		ch <- 1
	}()
	// copy from local to remote
	n, err := io.Copy(fc.conn.NetConn(), fc.qStram)
	<-ch
	if err != nil {
		log.Printf("copy to local failed:%s", err.Error())
		return
	}
	log.Printf("copy %d", n)
}
