package diagnostics

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
)

type HTTPOptions struct {
	// AllowRemote permits non-loopback clients. A non-empty Token is mandatory
	// when this is enabled.
	AllowRemote bool
	Token       string
	Timeout     time.Duration
}

func NewHTTPHandler(collector *Collector, options HTTPOptions) (http.Handler, error) {
	if collector == nil {
		return nil, errors.New("diagnostics: collector is required")
	}
	options.Token = strings.TrimSpace(options.Token)
	if options.AllowRemote && options.Token == "" {
		return nil, errors.New("diagnostics: remote access requires a token")
	}
	if options.Timeout <= 0 {
		options.Timeout = 3 * time.Second
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		if request.Method != http.MethodGet {
			writer.Header().Set("Allow", http.MethodGet)
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !options.AllowRemote && !remoteIsLoopback(request.RemoteAddr) {
			http.Error(writer, "diagnostics endpoint is loopback-only", http.StatusForbidden)
			return
		}
		if options.Token != "" && !validBearer(request.Header.Get("Authorization"), options.Token) {
			writer.Header().Set("WWW-Authenticate", `Bearer realm="yunka-diagnostics"`)
			http.Error(writer, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx, cancel := context.WithTimeout(request.Context(), options.Timeout)
		defer cancel()
		report := collector.Snapshot(ctx)
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		encoder := json.NewEncoder(writer)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(report); err != nil {
			return
		}
	}), nil
}

func remoteIsLoopback(remote string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remote))
	if err != nil {
		host = strings.TrimSpace(remote)
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func validBearer(header, token string) bool {
	const prefix = "Bearer "
	header = strings.TrimSpace(header)
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return false
	}
	candidate := strings.TrimSpace(header[len(prefix):])
	if len(candidate) != len(token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(candidate), []byte(token)) == 1
}
