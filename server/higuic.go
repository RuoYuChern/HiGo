package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/quic-go/quic-go"
	"taiji666.top/higo/base"
)

var guicLn *quic.Listener

func stopQuic(_ context.Context) {
	if guicLn != nil {
		guicLn.Close()
	}
}

type SessionPair struct {
	Remote  net.Conn
	Local   quic.Connection
	LStream quic.Stream
}

func (spr *SessionPair) cleanUp() {
	if spr.Local != nil {
		spr.Local.CloseWithError(0, "")
	}
	if spr.LStream != nil {
		spr.LStream.Close()
	}
	if spr.Remote != nil {
		spr.Remote.Close()
	}
}

func startQuic(ctx context.Context) error {
	address := "0.0.0.0:1080"
	tlsConf := &tls.Config{
		MinVersion: tls.VersionTLS13,
		MaxVersion: tls.VersionTLS13,
		NextProtos: []string{"free-go"},
		// 在这里添加你的证书和密钥文件路径
		Certificates: []tls.Certificate{loadCertificates()},
	}
	conf := &quic.Config{
		HandshakeIdleTimeout: 60 * time.Second,
		MaxIdleTimeout:       60 * time.Second,
	}
	ln, err := quic.ListenAddr(address, tlsConf, conf)
	guicLn = ln
	if err != nil {
		base.GLogger.Infof("start quic failed:%s", err.Error())
		return err
	}
	go func() {
		base.GLogger.Infof("start quic success:%s", address)
		for {
			sess, err := ln.Accept(ctx)
			if err != nil {
				base.GLogger.Infof("accept quic failed:%s", err.Error())
				return
			}
			spr := &SessionPair{Local: sess}
			go func() {
				handleSession(ctx, spr)
				spr.cleanUp()
			}()
		}
	}()

	return nil
}

func handleSession(ctx context.Context, spr *SessionPair) {
	// 处理 QUIC 会话
	// 在这里可以执行你的业务逻辑
	// 例如，接收和发送数据
	rsp := &base.AuthRsp{}
	lstream, err := spr.Local.AcceptStream(ctx)
	if err != nil {
		base.GLogger.Infof("read stream failed:%s", err.Error())
		rsp.Code = 500
		rsp.Msg = "read stream failed"
		return
	}
	spr.LStream = lstream
	buf := make([]byte, 1024)
	n, err := spr.LStream.Read(buf)
	if err != nil {
		base.GLogger.Infof("read stream failed:%s", err)
		rsp.Code = 500
		rsp.Msg = "read stream failed"
		echoQuic(spr.Local.LocalAddr().String(), rsp, spr.LStream)
		return
	}

	var dto base.AuthDto
	err = json.Unmarshal(buf[:n], &dto)
	if err != nil {
		base.GLogger.Infof("unmarshal stream failed:%s", err.Error())
		rsp.Code = 500
		rsp.Msg = "unmarshal stream failed"
		echoQuic(spr.Local.LocalAddr().String(), rsp, spr.LStream)
		return
	}

	base.GLogger.Debugf("Tid:%s, From user:%s, connect to:%s:%d", dto.Tid, dto.User, dto.Remote, dto.Port)
	remote, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", dto.Remote, dto.Port), 10*time.Second)
	if err != nil {
		base.GLogger.Infof("Tid:%s, connect to remote failed:%s", dto.Tid, err.Error())
		rsp.Code = 500
		rsp.Msg = "connect to remote failed"
		echoQuic(dto.Tid, rsp, spr.LStream)
		return
	}

	spr.Remote = remote
	rsp.Code = 200
	rsp.Msg = "success"
	if err := echoQuic(dto.Tid, rsp, spr.LStream); err != nil {
		return
	}

	// 开始代理数据
	ch := make(chan int)
	go func() {
		// 开始代理数据
		n, _ := io.Copy(spr.Remote, spr.LStream)
		ch <- int(n)
	}()
	io.Copy(spr.LStream, spr.Remote)
	<-ch
}

func echoQuic(tid string, rsp *base.AuthRsp, local quic.Stream) error {
	// 连接到远程 QUIC 服务器
	// 发送数据
	// 接收数据
	buf, err := json.Marshal(rsp)
	if err != nil {
		base.GLogger.Infof("marshal stream failed:%s", err.Error())
		return err
	}
	_, err = local.Write(buf)
	if err != nil {
		base.GLogger.Infof("Tid %s:write stream failed:%s", tid, err.Error())
		return err
	}
	return err
}

func loadCertificates() tls.Certificate {
	// 加载证书和密钥文件
	cert, err := tls.LoadX509KeyPair(gConf.Crt, gConf.Key)
	if err != nil {
		base.GLogger.Fatalf("Failed to load certificates: %v", err)
	}
	return cert
}
