package main

import (
	"os"
	"time"

	flag "github.com/spf13/pflag"
	"golang.org/x/time/rate"
)

// Config is all the cmdline-flag configurable options for ksonnet-playground
type Config struct {
	RateLimit         rate.Limit
	RateLimitBurst    int
	JsonnetRunTimeout time.Duration
	ExtraImportPath   string
	SkipCorsCheck     bool
	MaxContentLength  int64
	CacheSize         int64
}

var config = &Config{}

func init() {
	var timeoutSeconds int
	var rateLimit float64

	flag.Float64Var(&rateLimit, "rate-limit", 20.0, "Rate limit for API calls that aren't served from cache")
	flag.IntVar(&config.RateLimitBurst, "rate-limit-burst", 30, "Allowed burst for the rate limit")
	flag.Int64Var(&config.MaxContentLength, "max-content-length", 10240, "Maximum content length of input jsonnet")
	flag.IntVar(&timeoutSeconds, "jsonnet-run-timeout", 5, "Maximum duration to run jsonnet command for requests, in seconds")
	flag.StringVar(&config.ExtraImportPath, "extra-import-path", "ksonnet.beta.2", "Additional path to search for jsonnet import files")
	flag.BoolVar(&config.SkipCorsCheck, "skip-cors-check", false, "Set this flag to allow all origins to access the API")
	flag.Int64Var(&config.CacheSize, "cache-size", 10000, "Number of request entries to LRU cache")

	flag.Parse()

	config.RateLimit = rate.Limit(rateLimit)
	config.JsonnetRunTimeout = time.Duration(timeoutSeconds) * time.Second

	if os.Getenv("SKIP_CORS_CHECK") == "true" {
		config.SkipCorsCheck = true
	}

}
