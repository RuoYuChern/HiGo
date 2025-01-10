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

var s5Server net.Listener

const socks5Ver = uint8(5)

type S5Session struct {
	local  net.Conn
	remote quic.Stream
	tid    string
	addr   string
	port   int
}

func (s5 *S5Session) close() {
	if s5.local != nil {
		s5.local.Close()
	}
	if s5.remote != nil {
		s5.remote.Close()
	}
}

func stopS5() {
	if s5Server != nil {
		s5Server.Close()
		s5Server = nil
	}
}
func startS5(_ context.Context) error {
	srv, err := net.Listen("tcp", fmt.Sprintf(":%d", gConf.S5Port))
	if err != nil {
		base.GLogger.Infof("start s5 failed:%s", err.Error())
		return err
	}
	s5Server = srv
	fmt.Printf("Socks5 proxy server is running on 127.0.0.1:%d", gConf.S5Port)
	uid := base.GenerateRandomString(5)
	seq := 0
	go func() {
		for {
			conn, err := s5Server.Accept()
			if err != nil {
				fmt.Println("accept failed:", err)
				continue
			}
			seq++
			s5 := &S5Session{local: conn, tid: fmt.Sprintf("SEQ-%s-%d", uid, seq)}
			go func() {
				if handleS5Conn(s5) != nil {
					s5.close()
					return
				}
				handleS5Forward(s5)
				s5.close()
			}()
		}
	}()
	return nil

}

func isInternalIP(ip string) bool {
	// 解析IP地址
	addr := net.ParseIP(ip)
	if addr == nil {
		return false
	}
	return (addr.IsPrivate() || addr.IsLoopback())
}

func handleS5Conn(s5 *S5Session) error {
	// 读取客户端发送的版本号和认证方法数量
	buf := make([]byte, 257)
	_, err := io.ReadFull(s5.local, buf[:2])
	if err != nil {
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
	case 0x01: // IPv4
		_, err = io.ReadFull(s5.local, buf[:4])
		if err != nil {
			base.GLogger.Infof("Tid:%s, Failed to read IPv4 address: %v", s5.tid, err)
			return err
		}
		addr = fmt.Sprintf("%d.%d.%d.%d", buf[0], buf[1], buf[2], buf[3])
	case 0x03: // 域名
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
	case 0x04: // IPv6
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
	if isInternalIP(addr) {
		return errors.New("internal IP address")
	}

	sse := connecToProxy(s5)
	if sse == nil {
		base.GLogger.Infof("Tid:%s, Failed to connect to target %s", s5.tid, s5.addr)
		return errors.New("failed to connect to proxy")
	}
	s5.remote = sse
	// 回复客户端连接成功
	_, err = s5.local.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	if err != nil {
		base.GLogger.Infof("Tid:%s, Failed to write response: %v", s5.tid, err)
		return err
	}
	return nil
}

func connecToProxy(s5 *S5Session) quic.Stream {
	tlsConf := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
		InsecureSkipVerify: false,
		NextProtos:         []string{"free-go"},
		ClientSessionCache: tls.NewLRUClientSessionCache(100),
	}
	conf := &quic.Config{
		HandshakeIdleTimeout: 60 * time.Second,
		MaxIdleTimeout:       60 * time.Second,
		KeepAlivePeriod:      10 * time.Second,
	}

	conn, err := quic.DialAddr(context.Background(), gConf.Url, tlsConf, conf)
	if err != nil {
		base.GLogger.Infof("Tid:%s, connect to %s failed:%s", s5.tid, gConf.Url, err.Error())
		return nil
	}
	stream, err := conn.OpenStreamSync(context.Background())
	if err != nil {
		base.GLogger.Infof("Tid:%s, create stream failed:%s", s5.tid, err.Error())
		conn.CloseWithError(0, "")
		return nil
	}
	auth := base.AuthDto{
		User:   "amos",
		Port:   s5.port,
		Remote: s5.addr,
		Tid:    s5.tid,
	}
	authStr, _ := json.Marshal(auth)
	_, err = stream.Write(authStr)
	if err != nil {
		base.GLogger.Infof("Tid:%s, write stream failed:%s", s5.tid, err.Error())
		conn.CloseWithError(0, "")
		return nil
	}
	buf := make([]byte, 256)
	n, err := stream.Read(buf)
	if err != nil {
		base.GLogger.Infof("Tid:%s, read stream failed:%s", s5.tid, err.Error())
		conn.CloseWithError(0, "")
		return nil
	}
	rsp := base.AuthRsp{}
	err = json.Unmarshal(buf[:n], &rsp)
	if err != nil {
		base.GLogger.Infof("Tid:%s, Unmarshal stream failed:%s", s5.tid, err.Error())
		conn.CloseWithError(0, "")
		return nil
	}
	if rsp.Code != 200 {
		base.GLogger.Infof("Tid:%s, Connect stream failed:%s", s5.tid, rsp.Msg)
		conn.CloseWithError(0, "")
		return nil
	}
	s5.remote = stream
	return stream
}

func handleS5Forward(s5 *S5Session) {
	// 开始代理数据
	go io.Copy(s5.remote, s5.local)
	io.Copy(s5.local, s5.remote)
}
