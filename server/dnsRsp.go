package main

type DnsBean struct {
	Domain string   `json:"domain"`
	Ip     []string `json:"ip"`
	TTL    int      `json:"ttl"`
}
