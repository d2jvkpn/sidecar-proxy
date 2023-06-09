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
	"strings"
	"time"

	"github.com/d2jvkpn/gotk"
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
	Service        string   `mapstructure:"service"`
	Cors           string   `mapstructure:"cors"`
	PassWithPrefix []string `mapstructure:"pass_with_prefix"`

	Tls  bool   `mapstructure:"tls"`
	Cert string `mapstructure:"cert"`
	Key  string `mapstructure:"key"`

	BasicAuth gotk.BasicAuths `mapstructure:"basic_auth"`
}

type SidecarProxyServer struct {
	config SidecarProxyConfig
	svcUrl *url.URL
	proxy  *httputil.ReverseProxy
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

	/*
		for _, v := range config.PassWithPrefix {
			if !strings.HasPrefix(v, "/") {
				return nil, fmt.Errorf("invalid valid in pass_with_prefix: %s", v)
			}
		}
	*/

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
	sps.proxy = httputil.NewSingleHostReverseProxy(sps.svcUrl)

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

	var (
		shouldPass bool
		msg        string
		remoteAddr string
		ip         string
		authCode   string
		err        error
		startAt    time.Time
		user       *gotk.BasicAuthUser
		fields     []zap.Field
	)

	r.Host = sps.svcUrl.Host
	msg = fmt.Sprintf("%s@%s", r.Method, r.URL.Path)

	shouldPass = false
	for i := range sps.config.PassWithPrefix {
		// fmt.Println("~~~", msg, sps.config.PassWithPrefix[i])
		if strings.HasPrefix(msg, sps.config.PassWithPrefix[i]) {
			shouldPass = true
			break
		}
	}
	if shouldPass {
		sps.proxy.ServeHTTP(w, r)
		return
	}

	startAt = time.Now()
	remoteAddr = r.RemoteAddr
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		remoteAddr = v
	}
	ip, _, _ = net.SplitHostPort(remoteAddr)

	fields = make([]zap.Field, 0, 5)
	fields = append(fields, zap.String("ip", ip))

	user, authCode, err = sps.config.BasicAuth.Handle(w, r)
	fields = append(fields, zap.String("auth_code", authCode))

	if user != nil {
		fields = append(fields, zap.String("user", user.Username))
	}

	if err != nil {
		fields = append(fields, zap.Any("error", err))
		sps.logger.Error(msg, fields...)
		return
	}

	sps.proxy.ServeHTTP(w, r)
	fields = append(fields, zap.String("latency", time.Since(startAt).String()))
	sps.logger.Info(msg, fields...)
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
