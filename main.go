package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/d2jvkpn/sidecar-proxy/pkg"

	"github.com/d2jvkpn/gotk"
	"github.com/d2jvkpn/gotk/cloud-logging"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/crypto/bcrypt"
)

var (
	//go:embed project.yaml
	_Project []byte
)

func init() {
	gotk.RegisterLogPrinter()
}

func main() {
	var err error

	if len(os.Args) == 1 {
		log.Fatalln("subcommand serve/create-user required")
	}
	log.SetOutput(os.Stderr)

	serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
	createUserCmd := flag.NewFlagSet("create-user", flag.ExitOnError)

	switch os.Args[1] {
	case serveCmd.Name():
		err = serve(serveCmd, os.Args[2:])
	case createUserCmd.Name():
		err = createUser(createUserCmd, os.Args[2:])
	default:
		log.Fatalln("!!! invalid subcommand")
	}

	if err != nil {
		log.Fatalln(err)
	}
}

func serve(fSet *flag.FlagSet, args []string) (err error) {
	var (
		config   string
		addr     string
		meta     map[string]any
		vp       *viper.Viper
		project  *viper.Viper
		logger   *logging.Logger
		sps      *pkg.SidecarProxyServer
		shutdown func() error
	)

	if project, err = gotk.LoadYamlBytes(_Project); err != nil {
		return
	}

	meta = gotk.BuildInfo()
	meta["project"] = project.GetString("project")
	meta["version"] = project.GetString("version")

	fSet.StringVar(&config, "config", "configs/local.yaml", "configuration yaml file")
	fSet.StringVar(&addr, "addr", ":9000", "http server address")

	fSet.Usage = func() {
		output := flag.CommandLine.Output()

		fmt.Fprintf(output, "Usage:\n")
		flag.PrintDefaults()
		fmt.Fprintf(output, "\nConfiguration:\n```yaml\n%s```\n", project.GetString("config"))
		fmt.Fprintf(output, "\nBuild:\n```text\n%s\n```\n", gotk.BuildInfoText(meta))
	}

	fSet.Parse(args)

	if vp, err = gotk.LoadYamlConfig(config, "Configuration"); err != nil {
		return
	}

	logger, err = logging.NewLogger("logs/sidecar-proxy.log", zapcore.InfoLevel, 256)
	defer func() {
		_ = logger.Down()
	}()

	vp = vp.Sub("sidecar_proxy")
	if sps, err = pkg.NewSidecarProxyServer(vp, logger.Named("proxy")); err != nil {
		return
	}

	if shutdown, err = sps.Serve(addr); err != nil {
		return
	}
	msg := fmt.Sprintf(
		"Http server is listening on: %s => %s",
		addr, vp.GetString("service"),
	)
	log.Println("==>", msg)
	logger.Info(msg)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGUSR2)

	select {
	case sig := <-quit: // sig := <-quit:
		// if sig == syscall.SIGUSR2 {...}
		fmt.Fprintln(os.Stderr, "... received:", sig)
	}

	if err = shutdown(); err != nil {
		logger.Error("http server shutdown", zap.Any("error", err))
	} else {
		logger.Info("http server is down")
		log.Println("<<< Exit")
	}

	return err
}

func createUser(fSet *flag.FlagSet, args []string) (err error) {
	var (
		cost     int
		username []byte
		password []byte
		bts      []byte
		method   string
		reader   *bufio.Reader
	)

	fSet.StringVar(&method, "method", "md5", "password hash method: md5 or bcrypt")
	fSet.IntVar(&cost, "cost", 10, "bcrypt adaptive hashing cost, works with method bcrypt")
	fSet.Parse(args)

	if method != "md5" && method != "bcrypt" {
		return fmt.Errorf("!!! invlaid hash method")
	}

	reader = bufio.NewReader(os.Stdin)
	fmt.Fprint(os.Stderr, ">>> Username: ")
	if username, err = reader.ReadBytes('\n'); err != nil {
		return
	}
	if username = bytes.TrimSpace(username); len(username) == 0 {
		return fmt.Errorf("empty username")
	}

	fmt.Fprint(os.Stderr, ">>> Password: ")
	if password, err = reader.ReadBytes('\n'); err != nil {
		return
	}
	if password = bytes.TrimSpace(password); len(password) == 0 {
		return fmt.Errorf("empty password")
	}

	switch method {
	case "md5":
		sum := md5.Sum(bytes.Join([][]byte{username, password}, []byte(":")))
		bts = []byte(fmt.Sprintf("%x", sum[:]))
	default: // bcrypt
		bts, err = bcrypt.GenerateFromPassword(password, cost)
	}

	if err != nil {
		return
	}

	// os.Stdout
	fmt.Printf("users:\n- { username: %q, password: %q }\n", username, bts)
	return
}
