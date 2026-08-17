package middleware

import (
	"flag"
	_ "net/http/pprof"
	"regexp"
	"time"
	"yunka.io/framework/core/request"
	"yunka.io/gateway/dispatcher/proxy"
	"yunka.io/gateway/rpc/meta"
	"yunka.io/pkg/stringsExt"

	"github.com/prometheus/client_golang/prometheus"
)

/**
 * @BelongProject yunka
 * @BelongPackage middleware
 * @Description:
 *
 * @Copyright 2020 - Powered By 云咖
 * @Author: fworld
 * @Date:  2020/12/17 9:41 上午
 * @Version V1.0
 */

var (
	_ proxy.MiddleWare = (*PrometheusMiddleware)(nil)
)

const (
	namespace                = "service"
	prometheusMiddlewareName = `prometheus`
)

var addr = flag.String("listen-address", ":18080", "The address to listen on for HTTP requests.")

var (
	labels = []string{"status", "endpoint", "method"}

	uptime = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "uptime",
			Help:      "HTTP service uptime.",
		}, nil,
	)

	reqCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_request_count_total",
			Help:      "Total number of HTTP requests made.",
		}, labels,
	)

	reqDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latencies in seconds.",
		}, labels,
	)

	reqSizeBytes = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: namespace,
			Name:      "http_request_size_bytes",
			Help:      "HTTP request sizes in bytes.",
		}, labels,
	)

	respSizeBytes = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: namespace,
			Name:      "http_response_size_bytes",
			Help:      "HTTP request sizes in bytes.",
		}, labels,
	)
)

type PrometheusMiddleware struct {
	proxy.Next
}

// init registers the prometheus metrics
func Init() {
	//prometheus.MustRegister(uptime, reqCount, reqDuration, reqSizeBytes, respSizeBytes)
	//go recordUptime()
	//flag.Parse()
	//go func() {
	//	http.Handle("/metrics", promhttp.Handler())
	//	http.ListenAndServe(*addr, nil)
	//}()
}

// recordUptime increases service uptime per second.
func recordUptime() {
	for range time.Tick(time.Second) {
		uptime.WithLabelValues().Inc()
	}
}

// PromOpts represents the Prometheus middleware Options.
// It is used for filtering labels by regex.
type PromOpts struct {
	ExcludeRegexStatus   string
	ExcludeRegexEndpoint string
	ExcludeRegexMethod   string
}

var defaultPromOpts = &PromOpts{}

// checkLabel returns the match result of labels.
// Return true if regex-pattern compiles failed.
func (po *PromOpts) checkLabel(label, pattern string) bool {
	if pattern == "" {
		return true
	}

	matched, err := regexp.MatchString(pattern, label)
	if err != nil {
		return true
	}
	return !matched
}

func (pm *PrometheusMiddleware) Name() string {
	return prometheusMiddlewareName
}

// PromMiddleware returns a gin.HandlerFunc for exporting some Web metrics
func (pm *PrometheusMiddleware) Do(authStatus bool, rt request.Runtime, api *meta.RuntimeApi) {
	// make sure promOpts is not nil
	promOpts := defaultPromOpts
	start := time.Now()
	pm.Next.Do(authStatus, rt, api)
	status := rt.Status()
	endpoint := api.Uri
	method := stringsExt.SliceToString(rt.GetRequestCtx().Method())

	lvs := []string{status, endpoint, method}

	isOk := promOpts.checkLabel(status, promOpts.ExcludeRegexStatus) &&
		promOpts.checkLabel(endpoint, promOpts.ExcludeRegexEndpoint) &&
		promOpts.checkLabel(method, promOpts.ExcludeRegexMethod)

	if !isOk {
		return
	}

	reqCount.WithLabelValues(lvs...).Inc()
	reqDuration.WithLabelValues(lvs...).Observe(time.Since(start).Seconds())
	respSizeBytes.WithLabelValues(lvs...).Observe(float64(rt.GetRequestCtx().Response.Header.ContentLength()))
}
