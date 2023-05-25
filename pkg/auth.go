package pkg

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

type AuthConfig struct {
	Enable bool              `mapstructure:"enable"`
	Users  map[string]string `mapstructure:"users"`
}

func (sps *SidecarProxyServer) auth(w http.ResponseWriter, r *http.Request) (code string, err error) {
	auth := &sps.config.Auth
	if !auth.Enable {
		return "auth_disabled", nil
	}

	var (
		ok       bool
		key      []byte
		password string
	)

	key = []byte(r.Header.Get("Authorization"))
	if !bytes.HasPrefix(key, []byte("Basic ")) {
		w.Header().Set("Www-Authenticate", `Basic realm="username:password"`)
		w.WriteHeader(http.StatusUnauthorized)
		return "login_required", fmt.Errorf("login required")
	}
	key = key[6:]

	if key, err = base64.StdEncoding.DecodeString(string(key)); err != nil {
		w.Header().Set("Www-Authenticate", `Basic realm="username:password"`)
		w.WriteHeader(http.StatusUnauthorized)
		return "decode_basic_failed", fmt.Errorf("invalid basic")
	}

	u, p, found := bytes.Cut(key, []byte{':'})
	if !found {
		w.Header().Set("Www-Authenticate", `Basic realm="username:password"`)
		w.WriteHeader(http.StatusUnauthorized)
		return "invalid_token", fmt.Errorf("invalid token")
	}

	if password, ok = auth.Users[string(u)]; !ok {
		_ = bcrypt.CompareHashAndPassword([]byte(password), p)
		w.WriteHeader(http.StatusUnauthorized)
		w.Header().Set("Www-Authenticate", `Basic realm="username:password"`)
		return "incorrect_username", fmt.Errorf("incorrect username or password")
	}

	if err = bcrypt.CompareHashAndPassword([]byte(password), p); err != nil {
		w.Header().Set("Www-Authenticate", `Basic realm="username:password"`)
		w.WriteHeader(http.StatusUnauthorized)
		return "incorrect_password", fmt.Errorf("incorrect username or password")
	}

	r.Header.Del("Authorization")

	return "ok", nil
}
