package pkg

import (
	"context"
	"crypto/tls"
	"fmt"
	// "log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/d2jvkpn/gotk/impls"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

/*
```yaml
sidecar_proxy:

	addr: :8080
	service: localhost:8000
	cors: "*"
	tls: true
	cert: "configs/server.cert"
	key: "configs/server.key"
	auth:
	  enable: true
	  users:
	    x1: bycrypt-y1
	    x2: bycrypt-y2

```
*/
type SidecarProxyConfig struct {
	Service string `mapstructure:"service"`
	Cors    string `mapstructure:"cors"`

	Tls  bool   `mapstructure:"tls"`
	Cert string `mapstructure:"cert"`
	Key  string `mapstructure:"key"`

	BasicAuth impls.BasicAuthentication `mapstructure:"basic_auth"`
}

type SidecarProxyServer struct {
	config SidecarProxyConfig
	svcUrl *url.URL
	server *http.Server
	logger *zap.Logger
}

func NewSidecarProxyServer(vp *viper.Viper, logger *zap.Logger, opts ...func(*http.Server)) (
	sps *SidecarProxyServer, err error) {
	var (
		config SidecarProxyConfig
		cert   tls.Certificate
	)

	vp.SetDefault("cors", "*")
	if err = vp.Unmarshal(&config); err != nil {
		return nil, err
	}

	if err = config.BasicAuth.Validate(); err != nil {
		return nil, err
	}

	sps = &SidecarProxyServer{
		config: config,
		server: new(http.Server),
		logger: logger,
	}

	if sps.svcUrl, err = url.Parse(config.Service); err != nil {
		return nil, err
	}

	for i := range opts {
		opts[i](sps.server)
	}

	if config.Tls {
		if cert, err = tls.LoadX509KeyPair(config.Cert, config.Key); err != nil {
			return nil, err
		}

		sps.server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
	}

	return sps, nil
}

func (sps *SidecarProxyServer) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		header := w.Header()
		header.Set("Access-Control-Allow-Origin", sps.config.Cors)
		header.Set("Access-Control-Expose-Headers", "Content-Type, Authorization")

		header.Set("Access-Control-Expose-Headers", "Access-Control-Allow-Origin, "+
			"Access-Control-Allow-Headers, Content-Type, Content-Length")

		header.Set("Access-Control-Allow-Credentials", "true")
		header.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, HEAD")
		return
	}

	startAt := time.Now()
	msg := fmt.Sprintf("%s %s", r.Method, r.URL.Path)

	remoteAddr := r.RemoteAddr
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		remoteAddr = v
	}
	ip, _, _ := net.SplitHostPort(remoteAddr)

	user, authCode, err := sps.config.BasicAuth.Handle(w, r)
	if err != nil {
		sps.logger.Error(
			msg,
			zap.String("ip", ip),
			zap.String("user", user),
			zap.String("auth_code", authCode),
			zap.String("latency", time.Since(startAt).String()),
			zap.Any("error", err),
		)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(sps.svcUrl)
	r.Host = sps.svcUrl.Host
	proxy.ServeHTTP(w, r)

	sps.logger.Info(
		msg,
		zap.String("ip", ip),
		zap.String("user", user),
		zap.String("auth_code", authCode),
		zap.String("latency", time.Since(startAt).String()),
	)
}

func (sps *SidecarProxyServer) Serve(addr string) (shutdown func() error, err error) {
	var (
		listener net.Listener
		mux      *http.ServeMux
	)

	if listener, err = net.Listen("tcp", addr); err != nil {
		return nil, err
	}

	mux = http.NewServeMux()
	// mux.Handle("/", handler)
	mux.HandleFunc("/", sps.handle)
	sps.server.Handler = mux

	shutdown = func() error {
		var err error
		ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
		err = sps.server.Shutdown(ctx)
		cancel()
		return err
	}

	go func() {
		if sps.server.TLSConfig != nil {
			_ = sps.server.ServeTLS(listener, "", "")
		} else {
			_ = sps.server.Serve(listener)
		}
	}()

	return shutdown, nil
}
