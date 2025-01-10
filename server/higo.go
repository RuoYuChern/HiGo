package main

import (
	"context"
	"os/signal"
	"syscall"

	"taiji666.top/higo/base"
)

type container struct {
	ctx  context.Context
	stop context.CancelFunc
}

var gConf *base.HiServerConf = &base.HiServerConf{}

func (app *container) Start() error {
	conf := "../config/higo.yaml"
	gConf.ReadConf(conf)
	base.InitLog(&gConf.Log)
	base.GslbGeoDb.Start()
	app.ctx, app.stop = signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	base.GLogger.Info("AppLife startted")
	startWeb()
	startQuic()
	return nil
}

func (app *container) Wait() {
	<-app.ctx.Done()
	app.stop()
	base.GLogger.Info("Shutdown Server ...")
}

func (app *container) Stop() {
	base.GslbGeoDb.Stop()
	stopWeb()
	stopQuic(context.Background())
}

func main() {
	app := &container{}
	app.Start()
	base.GLogger.Infof("Agent is running")

	app.Wait()
	app.Stop()
}
