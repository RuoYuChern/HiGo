package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"taiji666.top/higo/base"
)

var httpSrv *http.Server

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Here you can implement logic to check the origin of the request
		// For simplicity, we allow all origins in this example
		return true
	},
}

func startWeb() {
	router := gin.New()
	router.MaxMultipartMemory = 8 << 20
	router.Use(gin.Recovery())
	addRouter(router)
	httpSrv = &http.Server{
		Addr:    fmt.Sprintf(":%d", 8080),
		Handler: router,
	}
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			base.GLogger.Errorf("listen error:%s", err.Error())
		}
	}()
}

func wsGo(c *gin.Context) {
	clientIp := c.ClientIP()
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		base.GLogger.Errorf("upgrade error:%s", err.Error())
		c.AbortWithError(500, err)
		return
	}
	rsp := &base.AuthRsp{}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		base.GLogger.Errorf("read error:%s", err.Error())
		rsp.Code = 500
		rsp.Msg = "read msg failed"
		echoWs(clientIp, rsp, conn)
		conn.Close()
		return
	}
	var dto base.AuthDto
	err = json.Unmarshal(msg, &dto)
	if err != nil {
		base.GLogger.Errorf("read error:%s", err.Error())
		rsp.Code = 500
		rsp.Msg = "Unmarshal msg failed"
		echoWs(clientIp, rsp, conn)
		conn.Close()
		return
	}

	remote, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", dto.Remote, dto.Port), 10*time.Second)
	if err != nil {
		base.GLogger.Infof("Tid:%s, connect to remote failed:%s", dto.Tid, err.Error())
		rsp.Code = 500
		rsp.Msg = "connect to remote failed"
		echoWs(dto.Tid, rsp, conn)
		conn.Close()
		return
	}
	base.GLogger.Infof("Tid:%s, From user:%s, connect to:%s:%d", dto.Tid, dto.User, dto.Remote, dto.Port)
	rsp.Code = 200
	rsp.Msg = "connect to remote failed"
	echoWs(dto.Tid, rsp, conn)
	go func() {
		fromLocal(dto.Tid, conn, remote)
	}()
	fromRemote(dto.Tid, conn, remote)
	conn.Close()
	remote.Close()
}

func fromRemote(tid string, local *websocket.Conn, remote net.Conn) {
	buf := make([]byte, 1024)
	for {
		n, err := remote.Read(buf)
		if err != nil {
			if err != io.EOF {
				base.GLogger.Infof("Tid:%s read error:%s", tid, err.Error())
			}
			return
		}
		err = local.WriteMessage(websocket.BinaryMessage, buf[:n])
		if err != nil {
			base.GLogger.Infof("Tid:%s write error:%s", tid, err.Error())
			return
		}
	}
}

func fromLocal(tid string, local *websocket.Conn, remote net.Conn) {
	for {
		mtp, msg, err := local.ReadMessage()
		if err != nil {
			base.GLogger.Infof("Tid:%s read error:%s", tid, err.Error())
			return
		}
		_, err = remote.Write(msg)
		if err != nil {
			base.GLogger.Infof("Tid:%s write error:%s", tid, err.Error())
			return
		}
		if mtp == websocket.CloseMessage {
			return
		}
	}
}

func echoWs(tid string, rsp *base.AuthRsp, stream *websocket.Conn) error {
	mtp := websocket.TextMessage
	rspStr, err := json.Marshal(rsp)
	if err != nil {
		base.GLogger.Errorf("Tid:%s marshal error:%s", tid, err.Error())
		return err
	}
	err = stream.WriteMessage(mtp, rspStr)
	if err != nil {
		base.GLogger.Errorf("Tid:%s write error:%s", tid, err.Error())
		return err
	}
	return nil
}

func addRouter(router *gin.Engine) {
	router.GET(("go/ws"), wsGo)
	router.GET(("go/dns-query"), httpDnsHandler)
	router.GET(("go/d"), httpDnsHandler2)
}

func stopWeb() {
	if httpSrv != nil {
		httpSrv.Shutdown(context.Background())
	}
}
