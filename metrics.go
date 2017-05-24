package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	p8sRequestDuration = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name: "ksonnetplayground_request_duration",
			Help: "Duration of requests to the ksonnet playground",
		},
		[]string{"method"},
	)

	p8sRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ksonnetplayground_requests",
			Help: "Number of total requests to the ksonnet playground",
		},
		[]string{"method"},
	)

	p8sRateLimitedRequests = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ksonnetplayground_requests_ratelimited",
		Help: "Number of requests to the ksonnet playground where we responded with HTTP 429 due to rate limits",
	})

	p8sTimeoutRequests = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ksonnetplayground_requests_jsonnet_timeout",
		Help: "Number of requests to the ksonnet playground where we hit a timeout running jsonnet",
	})

	p8sRunningRequests = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "ksonnetplayground_running_requests",
		Help: "Number of requests to the ksonnet playground currently being processed by this instance",
	})

	p8sJsonnetCacheHits = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ksonnetplayground_jsonnet_cache_hits",
		Help: "Number of requests to the ksonnet playground API where the input jsonnet code is a cache hit",
	})

	p8sJsonnetCacheMisses = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ksonnetplayground_jsonnet_cache_misses",
		Help: "Number of requests to the ksonnet playground API where the input jsonnet code is a cache miss",
	})
)

func init() {
	prometheus.MustRegister(
		p8sRequests,
		p8sRateLimitedRequests,
		p8sTimeoutRequests,
		p8sRequestDuration,
		p8sRunningRequests,
		p8sJsonnetCacheHits,
		p8sJsonnetCacheMisses,
	)
}
