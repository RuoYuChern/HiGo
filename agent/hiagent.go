package main

import (
	"context"
	"os/signal"
	"syscall"
	"time"

	"taiji666.top/higo/base"
)

type container struct {
	ctx  context.Context
	stop context.CancelFunc
}

var gConf *base.HiAgentConf = &base.HiAgentConf{}

func (app *container) Start() error {
	conf := "../config/hiagent.yaml"
	gConf.ReadConf(conf)
	base.InitLog(&gConf.Log)
	base.GslbGeoDb.Start()
	app.ctx, app.stop = signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	err := startS5(app.ctx)
	if err != nil {
		base.GLogger.Errorf("start socks5 failed:%s", err.Error())
		return err
	}

	err = startHttp(&app.ctx)
	if err != nil {
		base.GLogger.Errorf("start https failed:%s", err.Error())
		return err
	}

	startDns()

	base.GLogger.Info("AppLife startted")
	return nil
}

func (app *container) Wait() {
	<-app.ctx.Done()
	app.stop()
	base.GLogger.Info("Shutdown Server ...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stopS5()
	stopDns()
	stopHttp(&ctx)
}

func (app *container) Stop() {
	base.GslbGeoDb.Stop()
}

func main() {
	app := &container{}
	app.Start()
	base.GLogger.Infof("Agent is running")

	app.Wait()
	app.Stop()
}
