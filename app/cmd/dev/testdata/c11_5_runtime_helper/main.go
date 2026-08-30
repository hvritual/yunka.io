package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	name := flag.String("name", "", "process name")
	listen := flag.String("listen", "", "explicit loopback listen address")
	events := flag.String("events", "", "event log path")
	readyDelay := flag.Duration("ready-delay", 100*time.Millisecond, "readiness delay")
	flag.Parse()
	if strings.TrimSpace(*name) == "" || strings.TrimSpace(*listen) == "" || strings.TrimSpace(*events) == "" {
		fmt.Fprintln(os.Stderr, "name, listen and events are required")
		os.Exit(2)
	}
	if err := appendEvent(*events, "start:"+*name); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	readyAt := time.Now().Add(*readyDelay)
	var readyOnce sync.Once
	mux := http.NewServeMux()
	mux.HandleFunc("/diagnostics", func(writer http.ResponseWriter, _ *http.Request) {
		ready := !time.Now().Before(readyAt)
		writer.Header().Set("Content-Type", "application/json")
		if !ready {
			writer.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(writer).Encode(map[string]any{"ready": false})
			return
		}
		readyOnce.Do(func() { _ = appendEvent(*events, "ready:"+*name) })
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"core": map[string]any{
				"state": "running",
				"health": map[string]any{"state": "healthy", "live": true, "ready": true},
				"runtime": map[string]any{
					"routeCount": 1,
					"rpcClientConfigured": false,
					"rpcServerCount": 0,
					"eventBusConfigured": false,
				},
			},
		})
	})
	server := &http.Server{Handler: mux}
	serveDone := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err == http.ErrServerClosed {
			err = nil
		}
		serveDone <- err
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	select {
	case <-signals:
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = server.Shutdown(ctx)
		cancel()
		_ = appendEvent(*events, "stop:"+*name)
		if err := <-serveDone; err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case err := <-serveDone:
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func appendEvent(path, value string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = fmt.Fprintln(file, value)
	return err
}
