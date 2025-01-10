package base

import (
	"crypto/rand"
	"errors"
	"math/big"
	"net"
	"strings"

	"github.com/oschwald/geoip2-golang"
)

type GeoDb struct {
	db *geoip2.Reader
}

var GslbGeoDb *GeoDb = &GeoDb{}

func (gdb *GeoDb) Start() {
	db, err := geoip2.Open("../data/GeoLite2-Country.mmdb")
	if err != nil {
		GLogger.Warnf("geoip2 open failed:%s", err)
		return
	}
	gdb.db = db
}

func (gdb *GeoDb) Stop() {
	if gdb.db != nil {
		gdb.db.Close()
	}
}

func (gdb *GeoDb) Country(rip string) (string, error) {
	if gdb.db == nil {
		return "", errors.New("gdb is nill")
	}

	ip := net.ParseIP(rip)
	record, err := gdb.db.Country(ip)
	if err != nil {
		GLogger.Infof("Get ip %s country failed:%s", rip, err)
		return "", err
	}
	return record.Country.IsoCode, nil
}

func SplitIp(ip string) string {
	pos := strings.Index(ip, ":")
	if pos < 0 {
		return ip
	}
	var r = []rune(ip)
	return string(r[0:pos])
}

func GenerateRandomString(n int) string {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"
	ret := make([]byte, n)
	for i := 0; i < n; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			// handle error
			panic(err)
		}
		ret[i] = letters[num.Int64()]
	}

	return string(ret)
}
