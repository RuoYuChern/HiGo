package main

import (
	"errors"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/miekg/dns"
	"taiji666.top/higo/base"
)

func httpDnsHandler(c *gin.Context) {
	domain := c.Query("dns")
	// 判空操作
	if domain == "" {
		c.AbortWithError(400, errors.New("domain is empty"))
		return
	}
	clientIp := c.ClientIP()
	base.GLogger.Infof("dns query for %s", domain)
	resp, err := handleDNSRequest(c, domain)
	if err != nil {
		base.GLogger.Infof("dns exchange for %s failed:%s", clientIp, err.Error())
		c.AbortWithError(500, err)
		return
	}
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Header().Set("Content-Type", "application/dns-message")
	dnsPack, err := resp.Pack()
	if err != nil {
		base.GLogger.Infof("dns pack for %s failed:%s", clientIp, err.Error())
		c.AbortWithError(500, err)
		return
	}
	c.Writer.Write(dnsPack)
}

func httpDnsHandler2(c *gin.Context) {
	domain := c.Query("host")
	// 判空操作
	if domain == "" {
		c.AbortWithError(400, errors.New("domain is empty"))
		return
	}
	clientIp := c.ClientIP()
	base.GLogger.Infof("dns query for %s", domain)
	resp, err := handleDNSRequest(c, domain)
	if err != nil {
		base.GLogger.Infof("dns exchange for %s failed:%s", clientIp, err.Error())
		c.AbortWithError(500, err)
		return
	}
	dnsBean := &DnsBean{
		Domain: domain,
		Ip:     []string{},
		TTL:    0,
	}
	for _, ans := range resp.Answer {
		if a, ok := ans.(*dns.A); ok {
			dnsBean.Ip = append(dnsBean.Ip, a.A.String())
			dnsBean.TTL = int(a.Hdr.Ttl)
			continue
		}

		if aaaa, ok := ans.(*dns.AAAA); ok {
			dnsBean.Ip = append(dnsBean.Ip, aaaa.AAAA.String())
			dnsBean.TTL = int(aaaa.Hdr.Ttl)
		}
	}
	c.JSON(http.StatusOK, dnsBean)
}

func handleDNSRequest(ctx *gin.Context, domain string) (*dns.Msg, error) {
	c := new(dns.Client)
	c.Net = "udp"
	var upstreamDNS string
	clientIp := ctx.ClientIP()
	msg := new(dns.Msg)

	if isIpv4(clientIp) {
		msg.SetQuestion(dns.Fqdn(domain), dns.TypeA)
		upstreamDNS = "8.8.8.8:53"
	} else {
		upstreamDNS = "[2001:4860:4860::8888]:53"
		msg.SetQuestion(dns.Fqdn(domain), dns.TypeAAAA)
	}
	resp, _, err := c.Exchange(msg, upstreamDNS)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func isIpv4(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return true
	}
	return ip.To4() != nil
}
