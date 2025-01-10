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
	}
	ln, err := quic.ListenAddr(address, tlsConf, conf)
	if err != nil {
		log.Fatalf("start quic failed:%s", err.Error())
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	go func() {
		log.Printf("start quic success:%s", address)
		for {
			local, err := ln.Accept(ctx)
			if err != nil {
				log.Printf("accept quic failed:%s", err.Error())
				return
			}
			go func() {
				log.Printf("handle:%s", local.LocalAddr().String())
				remote := connectTo(ctx)
				if remote != nil {
					log.Printf("connect to remote success:%s", remote.RemoteAddr().String())
					brokerTo(local, remote, ctx)
					remote.CloseWithError(0, "")
				}
				local.CloseWithError(0, "")
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
		InsecureSkipVerify: false,
		NextProtos:         []string{"free-go"},
		ClientSessionCache: tls.NewLRUClientSessionCache(100),
	}
	conf := &quic.Config{
		HandshakeIdleTimeout: 60 * time.Second,
		MaxIdleTimeout:       60 * time.Second,
		KeepAlivePeriod:      10 * time.Second,
	}
	url := "go.askdao.top:1080"
	remote, err := quic.DialAddr(ctx, url, tlsConf, conf)
	if err != nil {
		log.Fatalf("connect to remote failed:%s", err.Error())
		return nil
	}
	return remote
}

func brokerTo(local, remote quic.Connection, ctx context.Context) {
	localStream, err := local.OpenStreamSync(ctx)
	if err != nil {
		log.Fatalf("open stream failed:%s", err.Error())
		return
	}
	remoteStream, err := remote.AcceptStream(ctx)
	if err != nil {
		log.Fatalf("accept stream failed:%s", err.Error())
		return
	}
	go func() {
		io.Copy(localStream, remoteStream)
	}()
	io.Copy(remoteStream, localStream)
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
