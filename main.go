package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/d2jvkpn/sidecar-proxy/pkg"
	"github.com/d2jvkpn/sidecar-proxy/pkg/wrap"

	"github.com/d2jvkpn/gotk"
	"github.com/d2jvkpn/gotk/impls"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func init() {
	gotk.RegisterLogPrinter()
}

func main() {
	var (
		config   string
		err      error
		vp       *viper.Viper
		logger   *wrap.Logger
		sps      *pkg.SidecarProxyServer
		shutdown func() error
	)

	flag.StringVar(&config, "config", "configs/local.yaml", "configuration yaml file")
	flag.Parse()

	if vp, err = impls.LoadYamlConfig(config, "Configuration"); err != nil {
		log.Fatalln(err)
	}

	logger, err = wrap.NewLogger("logs/sidecar-proxy.log", zapcore.InfoLevel, 256)
	if err != nil {
		log.Fatalln(err)
	}

	vp = vp.Sub("sidecar_proxy")
	if sps, err = pkg.NewSidecarProxyServer(vp, logger.Named("proxy")); err != nil {
		log.Fatalln(err)
	}

	if shutdown, err = sps.Serve(); err != nil {
		log.Fatalln(err)
	}
	msg := fmt.Sprintf(
		"Http server is listening on: %q => %q",
		vp.GetString("addr"), vp.GetString("service"),
	)
	log.Println("==>", msg)
	logger.Info(msg)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGUSR2)

	select {
	case sig := <-quit: // sig := <-quit:
		// if sig == syscall.SIGUSR2 {...}
		fmt.Println("... received:", sig)
	}

	if err = shutdown(); err != nil {
		logger.Error("http server shutdown", zap.Any("error", err))
		log.Fatalln(err)
	} else {
		log.Println("<<< Exit")
		logger.Info("http server is down")
	}
}
