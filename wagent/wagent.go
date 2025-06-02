package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
	"taiji666.top/higo/base"
)

const socks5Ver = uint8(5)
const (
	Version byte = 0x05

	MethodNoAuth byte = 0x00
	MethodAuth   byte = 0x02
	MethodNone   byte = 0xFF

	CmdConnect      byte = 0x01
	CmdUdpAssociate byte = 0x03

	ATYPIPv4   byte = 0x01
	ATYPDomain byte = 0x03
	ATYPIPv6   byte = 0x04
)

type WSession struct {
	local  net.Conn
	remote *websocket.Conn
	tid    string
	addr   string
	port   int
}

type WAgent struct {
	server net.Listener
	dialer *websocket.Dialer
}

var gConf *base.HiAgentConf = &base.HiAgentConf{}

func isInternalIP(atyp byte, ip string) bool {
	// 解析IP地址
	if atyp == ATYPDomain {
		return false
	}

	addr := net.ParseIP(ip)
	if addr == nil {
		return false
	}
	return (addr.IsPrivate() || addr.IsLoopback())
}
func (w *WAgent) Start(ctx context.Context) error {
	conf := "../config/hiagent.yaml"
	gConf.ReadConf(conf)
	base.InitLog(&gConf.Log)
	srv, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", gConf.S5Port))
	if err != nil {
		base.GLogger.Infof("start s5 failed:%s", err.Error())
		return err
	}
	w.server = srv
	dialer := websocket.Dialer{
		HandshakeTimeout: 45 * time.Second,
		Subprotocols:     []string{"binary"}}
	w.dialer = &dialer
	base.GLogger.Infof("Socks5 proxy server is running on %s", srv.Addr().String())
	uid := base.GenerateRandomString(5)
	seq := 0
	go func() {
		for {
			conn, err := w.server.Accept()
			if err != nil {
				fmt.Println("accept failed:", err)
				break
			}
			seq++
			s5 := &WSession{local: conn, tid: fmt.Sprintf("SEQ-%s-%d", uid, seq)}
			go func() {
				if s5.handleS5Conn(ctx, w) == nil {
					s5.handleS5Forward()
				}
				s5.close()
			}()
		}
	}()
	return nil
}

func (w *WAgent) Stop() {
	if w.server != nil {
		w.server.Close()
		w.server = nil
	}
	if w.dialer != nil {
		w.dialer = nil
	}
}

func (w *WAgent) connectToRemote(tid string, ctx context.Context) *websocket.Conn {
	conn, _, err := w.dialer.DialContext(ctx, gConf.WUrl, nil)
	if err != nil {
		base.GLogger.Infof("Tid:%s, Failed to connect to remote server: %v", tid, err)
		return nil
	}
	return conn
}

func (s5 *WSession) close() {
	if s5.local != nil {
		s5.local.Close()
	}
	if s5.remote != nil {
		s5.remote.Close()
	}
}

func (s5 *WSession) handleS5Conn(ctx context.Context, w *WAgent) error {
	// 读取客户端发送的版本号和认证方法数量
	buf := make([]byte, 257)
	n, err := io.ReadFull(s5.local, buf[:2])
	if (err != nil) && (n < 2) {
		base.GLogger.Infof("Tid:%s, Failed to read version and method count: %v", s5.tid, err)
		return err
	}

	// 解析认证方法数量
	nmethods := int(buf[1])
	_, err = io.ReadFull(s5.local, buf[:nmethods])
	if err != nil {
		base.GLogger.Infof("Tid:%s, Failed to read methods: %v", s5.tid, err)
		return err
	}

	// 回复客户端，选择不需要认证的方法
	_, err = s5.local.Write([]byte{0x05, 0x00})
	if err != nil {
		base.GLogger.Infof("Tid:%s, Failed to write response: %v", s5.tid, err)
		return err
	}

	// 读取客户端发送的请求
	_, err = io.ReadFull(s5.local, buf[:4])
	if err != nil {
		base.GLogger.Infof("Tid:%s, Failed to read request: %v", s5.tid, err)
		return err
	}

	// 解析请求
	ver, _, _, atyp := buf[0], buf[1], buf[2], buf[3]
	if ver != socks5Ver {
		base.GLogger.Infof("Tid:%s, Unsupported SOCKS version: %v", s5.tid, ver)
		return errors.New("unsupported SOCKS version")
	}

	var addr string
	switch atyp {
	case ATYPIPv4: // IPv4
		_, err = io.ReadFull(s5.local, buf[:4])
		if err != nil {
			base.GLogger.Infof("Tid:%s, Failed to read IPv4 address: %v", s5.tid, err)
			return err
		}
		addr = fmt.Sprintf("%d.%d.%d.%d", buf[0], buf[1], buf[2], buf[3])
	case ATYPDomain: // 域名
		_, err = io.ReadFull(s5.local, buf[:1])
		if err != nil {
			base.GLogger.Infof("Tid:%s, Failed to read domain length: %v", s5.tid, err)
			return err
		}
		domainLen := int(buf[0])
		_, err = io.ReadFull(s5.local, buf[:domainLen])
		if err != nil {
			base.GLogger.Infof("Tid:%s, Failed to read domain: %v", s5.tid, err)
			return err
		}
		addr = string(buf[:domainLen])
	case ATYPIPv6: // IPv6
		_, err = io.ReadFull(s5.local, buf[:16])
		if err != nil {
			base.GLogger.Infof("Tid:%s, Failed to read IPv6 address: %v", s5.tid, err)
			return err
		}
		// 解析IPv6地址
		addr = fmt.Sprintf("%x:%x:%x:%x:%x:%x:%x:%x", buf[0:2], buf[2:4], buf[4:6], buf[6:8], buf[8:10], buf[10:12], buf[12:14], buf[14:16])
	default:
		base.GLogger.Infof("Tid:%s, Unsupported address type: %v", s5.tid, atyp)
		return errors.New("unsupported address type")
	}

	// 读取端口号
	_, err = io.ReadFull(s5.local, buf[:2])
	if err != nil {
		base.GLogger.Infof("Tid:%s, Failed to read port: %v", s5.tid, err)
		return err
	}
	port := int(buf[0])<<8 | int(buf[1])
	s5.addr = addr
	s5.port = port

	// 检查是否为内网IP
	if isInternalIP(atyp, addr) {
		return errors.New("internal IP address")
	}

	err = s5.connecToProxy(ctx, w)
	if err != nil {
		return err
	}
	// 回复客户端连接成功
	_, err = s5.local.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	if err != nil {
		base.GLogger.Infof("Tid:%s, Failed to write response: %v", s5.tid, err)
		return err
	}
	return nil
}

func (s5 *WSession) connecToProxy(ctx context.Context, w *WAgent) error {
	// 连接到远程服务器
	s5.remote = w.connectToRemote(s5.tid, ctx)
	if s5.remote == nil {
		return errors.New("connect to remote failed")
	}
	auth := base.AuthDto{
		User:   "amos",
		Port:   s5.port,
		Remote: s5.addr,
		Tid:    s5.tid,
	}
	authStr, _ := json.Marshal(auth)
	err := s5.remote.WriteMessage(websocket.BinaryMessage, authStr)
	if err != nil {
		base.GLogger.Infof("Tid:%s, write stream failed:%s", s5.tid, err.Error())
		return err
	}
	_, msg, err := s5.remote.ReadMessage()
	if err != nil {
		base.GLogger.Infof("Tid:%s, read stream failed:%s", s5.tid, err.Error())
		return err
	}
	rsp := base.AuthRsp{}
	err = json.Unmarshal(msg, &rsp)
	if err != nil {
		base.GLogger.Infof("Tid:%s, unmarshal failed:%s", s5.tid, err.Error())
		return err
	}
	if rsp.Code != 200 {
		base.GLogger.Infof("Tid:%s, Connect stream failed:%s", s5.tid, rsp.Msg)
		return errors.New("connect remote failed")
	}
	return nil
}

func (s5 *WSession) handleS5Forward() {
	// 转发
	ch := make(chan int)
	go func() {
		io.Copy(s5.remote.NetConn(), s5.local)
		ch <- 1
	}()
	io.Copy(s5.local, s5.remote.NetConn())
	<-ch
}
