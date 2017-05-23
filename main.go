package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// rateLimit is the number of requests per second we want to allow
	rateLimit         rate.Limit = 20
	rateLimitBurst               = 30
	jsonnetRunTimeout            = 5 * time.Second

	extraImportPath = "ksonnet.beta.1"
)

var (
	limiter *rate.Limiter

	skipCorsCheck = false
	errBusy       = errors.New("Server is busy, please try again")
	errTimeout    = errors.New("Jsonnet evaluation timed out")
	originRegexp  = regexp.MustCompile(`^https?://.*\.heptio\.com|localhost:\d+$`)

	p8sRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
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
)

func init() {
	if os.Getenv("SKIP_CORS_CHECK") == "true" {
		skipCorsCheck = true
	}

	limiter = rate.NewLimiter(rateLimit, rateLimitBurst)
	prometheus.MustRegister(p8sRequests)
	prometheus.MustRegister(p8sRateLimitedRequests)
	prometheus.MustRegister(p8sTimeoutRequests)
	prometheus.MustRegister(p8sRequestDuration)
	prometheus.MustRegister(p8sRunningRequests)
}

// JsonnetRequest represents a request from the client, containing
// code to be executed.
type JsonnetRequest struct {
	Code string `json:"code"`
}

// JsonnetResponse represents a response containing the result of some
// piece of code that was meant to be executed. The response is either
// an `Error` message, or an `Output` string containing syntactically
// valid JSON.
type JsonnetResponse struct {
	Error  *string `json:"error"`
	Output *string `json:"output"`
}

// errorResponse turns an error into a `JsonnetResponse`, serialized
// as a string.
func errorResponse(output string, err error) string {
	errorString := fmt.Sprintf("%s\n%s", err.Error(), output)
	res := JsonnetResponse{
		Error: &errorString,
	}
	bytes, err := json.Marshal(res)
	if err != nil {
		log.Fatalf("Failed to serialize success JSON response:\n%v", err)
	}
	return string(bytes)
}

// successResponse turns the string output of a Jsonnet run into a
// `JsonnetResponse`, serialized as a string.
func successResponse(output string) string {
	res := JsonnetResponse{
		Output: &output,
	}
	bytes, err := json.Marshal(res)
	if err != nil {
		log.Fatalf("Failed to serialize success JSON response:\n%v", err)
	}
	return string(bytes)
}

// runJsonnet wraps the execution of the jsonnet command.
// The output bytes are included even when there was an error.
func runJsonnet(ctx context.Context, code string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, jsonnetRunTimeout)
	defer cancel()

	outBytes, err := exec.CommandContext(ctx,
		"jsonnet",
		"-J", extraImportPath,
		"-e", code).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		p8sTimeoutRequests.Inc()
		err = errTimeout
	}
	return string(outBytes), err
}

func handler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	// Set CORS headers if requested
	if origin := r.Header.Get("Origin"); skipCorsCheck || originRegexp.Match([]byte(origin)) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers",
			"Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token")
	}

	// And if this is an OPTIONS request, stop here (don't process the body)
	if r.Method == http.MethodOptions {
		return
	}

	//Check rate limits
	if !limiter.Allow() {
		p8sRateLimitedRequests.Inc()
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(errorResponse("", errBusy)))
		return
	}

	// Decode the body and convert it
	decoder := json.NewDecoder(r.Body)
	var req JsonnetRequest
	err := decoder.Decode(&req)
	if err != nil {
		fmt.Fprint(w, errorResponse("", err))
		return
	}
	outBytes, err := runJsonnet(r.Context(), req.Code)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, errorResponse(string(outBytes), err))
	} else {
		fmt.Fprint(w, successResponse(string(outBytes)))
	}
	return
}

func main() {
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		// Host the main site on 8080
		defer wg.Done()

		mux := http.NewServeMux()

		p8sHandlerChain := promhttp.InstrumentHandlerInFlight(p8sRunningRequests,
			promhttp.InstrumentHandlerCounter(p8sRequests,
				promhttp.InstrumentHandlerDuration(p8sRequestDuration, http.HandlerFunc(handler)),
			),
		)

		mux.Handle("/", p8sHandlerChain)
		log.Println("Starting main server at :8080")
		err := http.ListenAndServe(":8080", mux)

		if err != nil {
			log.Fatal(err.Error())
		}
	}()

	go func() {
		// Host metrics on 9102 (so that we don't expose /metrics to the internet)
		defer wg.Done()

		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		log.Println("Starting metrics server at :9102")
		err := http.ListenAndServe(":9102", mux)

		if err != nil {
			log.Fatal(err.Error())
		}
	}()

	wg.Wait()
	log.Print("Graceful Exit...")
}
