package main

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/quic-go/quic-go"
)

type Session struct {
	lcon quic.Connection
	rcon quic.Connection
	lstr quic.Stream
	rstr quic.Stream
}

func (s *Session) close() {
	if s.lcon != nil {
		s.lcon.CloseWithError(0, "")
	}
	if s.rcon != nil {
		s.rcon.CloseWithError(0, "")
	}
	if s.lstr != nil {
		s.lstr.Close()
	}
	if s.rstr != nil {
		s.rstr.Close()
	}
}

func main() {
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
		MaxIncomingStreams:   300,
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	ln, err := quic.ListenAddr(address, tlsConf, conf)
	if err != nil {
		log.Fatalf("start quic failed:%s", err.Error())
		return
	}

	go func() {
		log.Printf("start quic success:%s", address)
		for {
			local, err := ln.Accept(ctx)
			if err != nil {
				log.Printf("accept quic failed:%s", err.Error())
				return
			}
			sess := &Session{lcon: local}
			go func() {
				log.Printf("handle:%s", local.LocalAddr().String())
				sess.rcon = connectTo(ctx)
				brokerTo(ctx, sess)
				sess.close()
			}()
		}
	}()
	<-ctx.Done()
	log.Printf("Shutdown Server...")
	cancel()
	ln.Close()
}

func connectTo(ctx context.Context) quic.Connection {
	tlsConf := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true,
		NextProtos:         []string{"free-go"},
		ClientSessionCache: tls.NewLRUClientSessionCache(100),
	}
	conf := &quic.Config{
		HandshakeIdleTimeout: 60 * time.Second,
		MaxIdleTimeout:       60 * time.Second,
		KeepAlivePeriod:      10 * time.Second,
		MaxIncomingStreams:   300,
	}
	url := "go.askdao.top:1080"
	remote, err := quic.DialAddr(ctx, url, tlsConf, conf)
	if err != nil {
		log.Fatalf("connect to remote failed:%s", err.Error())
		return nil
	}
	return remote
}

func brokerTo(ctx context.Context, sess *Session) {
	if sess.rcon == nil {
		return
	}

	lstr, err := sess.lcon.AcceptStream(ctx)
	if err != nil {
		log.Fatalf("open stream failed:%s", err.Error())
		return
	}
	sess.lstr = lstr

	rstr, err := sess.rcon.OpenStreamSync(ctx)
	if err != nil {
		log.Fatalf("accept stream failed:%s", err.Error())
		return
	}
	sess.rstr = rstr

	ch := make(chan int)
	go func() {
		_, err = io.Copy(sess.rstr, sess.lstr)
		if err != nil {
			log.Fatalf("copy from local failed:%s", err.Error())
		}
		ch <- 1
	}()
	n, err := io.Copy(sess.lstr, sess.rstr)
	<-ch
	if err != nil {
		log.Fatalf("copy to local failed:%s", err.Error())
		return
	}
	log.Printf("copy %d", n)
}

func loadCertificates() tls.Certificate {
	// 加载证书和密钥文件
	crt := "../config/broker.askdao.top_bundle.crt"
	key := "../config/broker.askdao.top.key"
	cert, err := tls.LoadX509KeyPair(crt, key)
	if err != nil {
		log.Fatalf("Failed to load certificates: %v", err)
	}
	return cert
}
