package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
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
	Remote    net.Conn
	QLocalCon quic.Connection
	QLocalStr quic.Stream
}

func (spr *SessionPair) cleanUp() {
	if spr.QLocalCon != nil {
		spr.QLocalCon.CloseWithError(0, "")
	}
	if spr.QLocalStr != nil {
		spr.QLocalStr.Close()
	}
	if spr.Remote != nil {
		spr.Remote.Close()
	}
}

func startQuic() error {
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
			sess, err := ln.Accept(context.Background())
			if err != nil {
				base.GLogger.Infof("accept quic failed:%s", err.Error())
				return
			}
			spr := &SessionPair{QLocalCon: sess}
			go func() {
				handleSession(spr)
				spr.cleanUp()
			}()
		}
	}()

	return nil
}

func handleSession(spr *SessionPair) {
	// 处理 QUIC 会话
	// 在这里可以执行你的业务逻辑
	// 例如，接收和发送数据
	rsp := &base.AuthRsp{}
	stream, err := spr.QLocalCon.AcceptStream(context.Background())
	if err != nil {
		base.GLogger.Infof("accept stream failed:%s", err.Error())
		rsp.Code = 500
		rsp.Msg = "accept stream failed"
		echoQuic(spr.QLocalCon.LocalAddr().String(), rsp, stream)
		return
	}
	// 处理 QUIC 流
	// 在这里可以执行你的业务逻辑
	spr.QLocalStr = stream
	buf := make([]byte, 1024)
	n, err := stream.Read(buf)
	if err != nil {
		base.GLogger.Infof("read stream failed:%s", err.Error())
		rsp.Code = 500
		rsp.Msg = "read stream failed"
		echoQuic(spr.QLocalCon.LocalAddr().String(), rsp, stream)
		return
	}

	if n < 0 {
		base.GLogger.Infof("read stream failed:%d", n)
		rsp.Code = 500
		rsp.Msg = "read stream failed"
		echoQuic(spr.QLocalCon.LocalAddr().String(), rsp, stream)
		return
	}

	var dto base.AuthDto
	err = json.Unmarshal(buf[:n], &dto)
	if err != nil {
		base.GLogger.Infof("unmarshal stream failed:%s", err.Error())
		rsp.Code = 500
		rsp.Msg = "unmarshal stream failed"
		echoQuic(spr.QLocalCon.LocalAddr().String(), rsp, stream)
		return
	}

	base.GLogger.Debugf("Tid:%s, From user:%s, connect to:%s:%d", dto.Tid, dto.User, dto.Remote, dto.Port)
	remote, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", dto.Remote, dto.Port), 10*time.Second)
	if err != nil {
		base.GLogger.Infof("Tid:%s, connect to remote failed:%s", dto.Tid, err.Error())
		rsp.Code = 500
		rsp.Msg = "connect to remote failed"
		echoQuic(dto.Tid, rsp, stream)
		return
	}

	spr.Remote = remote
	rsp.Code = 200
	rsp.Msg = "success"
	if err := echoQuic(dto.Tid, rsp, stream); err != nil {
		return
	}

	// 开始代理数据
	go func() {
		// 开始代理数据
		io.Copy(remote, stream)
	}()
	io.Copy(stream, remote)
}

func echoQuic(tid string, rsp *base.AuthRsp, stream quic.Stream) error {
	// 连接到远程 QUIC 服务器
	// 发送数据
	// 接收数据
	buf, err := json.Marshal(rsp)
	if err != nil {
		base.GLogger.Infof("marshal stream failed:%s", err.Error())
		return err
	}
	if buf == nil {
		base.GLogger.Infof("marshal stream failed:buf is nil")
		return errors.New("buf is nil")
	}
	_, err = stream.Write(buf)
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
