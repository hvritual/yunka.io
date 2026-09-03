package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// NewTraceHTTPHandler exposes one read-only TraceID analysis request without
// coupling diagnostics to a vendor telemetry SDK. It uses the same loopback /
// bearer-token boundary as the snapshot diagnostics handler.
func NewTraceHTTPHandler(analyzer *TraceAnalyzer, options HTTPOptions) (http.Handler, error) {
	if analyzer == nil {
		return nil, errors.New("diagnostics: trace analyzer is required")
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
		report, err := analyzer.Analyze(ctx, request.URL.Query().Get("trace_id"))
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, context.DeadlineExceeded) {
				status = http.StatusGatewayTimeout
			} else if errors.Is(err, context.Canceled) {
				status = http.StatusRequestTimeout
			}
			http.Error(writer, err.Error(), status)
			return
		}
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		encoder := json.NewEncoder(writer)
		encoder.SetEscapeHTML(false)
		_ = encoder.Encode(report)
	}), nil
}
