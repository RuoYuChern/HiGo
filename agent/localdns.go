package main

import (
	"net"

	"github.com/miekg/dns"
	"taiji666.top/higo/base"
)

var dnsSrv *dns.Server

func startDns() {
	// 创建 DNS 服务器
	dns.HandleFunc(".", HandleDNSRequest)
	dnsSrv = &dns.Server{Addr: ":53", Net: "udp"}
	go func() {
		err := dnsSrv.ListenAndServe()
		if err != nil {
			base.GLogger.Infof("start dns failed:%s", err.Error())
		}
	}()
}

func stopDns() {
	if dnsSrv != nil {
		dnsSrv.Shutdown()
		dnsSrv = nil
	}
}

func HandleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	c := new(dns.Client)
	c.Net = "udp"

	var upstreamDNS string
	if w.RemoteAddr().(*net.UDPAddr).IP.To4() != nil {
		upstreamDNS = "43.134.55.135:55"
	} else {
		upstreamDNS = "[2001:4860:4860::8888]:53"
	}
	resp, _, err := c.Exchange(r, upstreamDNS)
	if err != nil {
		base.GLogger.Infof("dns exchange for %s failed:%s", w.RemoteAddr().String(), err.Error())
		return
	}
	w.WriteMsg(resp)
}
