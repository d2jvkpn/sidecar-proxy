package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/d2jvkpn/sidecar-proxy/pkg"

	"github.com/d2jvkpn/gotk"
	"github.com/d2jvkpn/gotk/impls"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	//go:embed project.yaml
	_Project []byte
)

func init() {
	gotk.RegisterLogPrinter()
}

func main() {
	var (
		config   string
		addr     string
		err      error
		vp       *viper.Viper
		project  *viper.Viper
		logger   *impls.Logger
		sps      *pkg.SidecarProxyServer
		shutdown func() error
	)

	defer func() {
		if logger != nil {
			_ = logger.Down()
		}
		if err != nil {
			log.Fatalln(err)
		}
	}()

	if project, err = impls.LoadYamlBytes(_Project); err != nil {
		return
	}

	flag.StringVar(&config, "config", "configs/local.yaml", "configuration yaml file")
	flag.StringVar(&addr, "addr", ":8080", "http server address")

	flag.Usage = func() {
		output := flag.CommandLine.Output()

		fmt.Fprintf(output, "Usage:\n")
		flag.PrintDefaults()
		fmt.Fprintf(output, "\nConfiguration:\n```yaml\n%s```\n", project.GetString("config"))
	}

	flag.Parse()

	if vp, err = impls.LoadYamlConfig(config, "Configuration"); err != nil {
		return
	}

	logger, err = impls.NewLogger("logs/sidecar-proxy.log", zapcore.InfoLevel, 256)

	vp = vp.Sub("sidecar_proxy")
	if sps, err = pkg.NewSidecarProxyServer(vp, logger.Named("proxy")); err != nil {
		return
	}

	if shutdown, err = sps.Serve(addr); err != nil {
		return
	}
	msg := fmt.Sprintf(
		"Http server is listening on: %s => %s",
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
	} else {
		logger.Info("http server is down")
		log.Println("<<< Exit")
	}
}
