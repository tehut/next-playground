package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"sync"
	"time"

	"github.com/ghodss/yaml"
	"github.com/heptio/ksonnet-playground/api"
	"github.com/karlseguin/ccache"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

var (
	limiter      *rate.Limiter
	codeCache    *ccache.Cache
	errBusy      = errors.New("Server is busy, please try again")
	errTimeout   = errors.New("Jsonnet evaluation timed out")
	originRegexp = regexp.MustCompile(`^https?://.*\.heptio\.com|localhost:\d+$`)
)

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

// CachedResult is the results of a jsonnet response, stored in an LRU cache.
// It contains the body and HTTP code we should respond with when we see a
// given request body.
type CachedResult struct {
	Response string
	HTTPCode int
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
	ctx, cancel := context.WithTimeout(ctx, config.JsonnetRunTimeout)
	defer cancel()

	outBytes, err := exec.CommandContext(ctx,
		"jsonnet",
		"-J", config.ExtraImportPath,
		"-e", code).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		p8sTimeoutRequests.Inc()
		err = errTimeout
	}

	// Convert to yaml
	if err == nil {
		outBytes, err = yaml.JSONToYAML(outBytes)
	}
	return string(outBytes), err
}

func makeJsonnetCache(ctx context.Context, body []byte) CachedResult {
	// Decode the body and convert it
	decoder := json.NewDecoder(bytes.NewBuffer(body))
	var req JsonnetRequest
	err := decoder.Decode(&req)
	if err != nil {
		return CachedResult{
			HTTPCode: http.StatusBadRequest,
			Response: errorResponse("", err),
		}
	}

	outBytes, err := runJsonnet(ctx, req.Code)

	if err != nil {
		return CachedResult{
			HTTPCode: http.StatusBadRequest,
			Response: errorResponse(string(outBytes), err),
		}
	}

	return CachedResult{
		HTTPCode: http.StatusOK,
		Response: successResponse(string(outBytes)),
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, config.MaxContentLength)
	defer r.Body.Close()

	// Set CORS headers if requested
	if origin := r.Header.Get("Origin"); config.SkipCorsCheck || originRegexp.Match([]byte(origin)) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers",
			"Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token")
	}

	// And if this is an OPTIONS request, stop here (don't process the body)
	if r.Method == http.MethodOptions {
		return
	}

	// Check if this text is cached... read the body in so we can use it as a
	// cache key. A failure from the MaxBytesReader in this case means the request
	// is too large.
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		w.Write([]byte(errorResponse("", fmt.Errorf("Request too large - Code must be smaller than %v bytes", config.MaxContentLength))))
		return
	}

	if result := codeCache.Get(string(body)); result != nil && !result.Expired() {
		p8sJsonnetCacheHits.Inc()
		// Read the proper object from cache
		if realResult, ok := result.Value().(CachedResult); ok {
			w.WriteHeader(realResult.HTTPCode)
			w.Write([]byte(realResult.Response))
			return
		}
		//uh oh...
		log.Printf("Could not marshal result from cache into a CachedResult: %v", result.Value())
	}

	// If it wasn't cached, throttle before calculating it. We use rate limits for
	// the expensive part (running jsonnet). Requests that respond from cache or
	// are too large don't count against the limit.
	if !limiter.Allow() {
		p8sRateLimitedRequests.Inc()
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(errorResponse("", errBusy)))
		return
	}

	// Finally, generate a new cache result
	p8sJsonnetCacheMisses.Inc()
	cachedResult := makeJsonnetCache(r.Context(), body)
	codeCache.Set(string(body), cachedResult, 1*time.Hour)

	w.WriteHeader(cachedResult.HTTPCode)
	w.Write([]byte(cachedResult.Response))

	return
}

// func newHandler(w http.ResponseWriter, r *http.Request, resp api.InitResponse) {
// TO DO: Integrate this with init
// 	init, _ := json.Marshal(resp)
// 	s := string(init)
// 	fmt.Fprintf(w, s)
// 	// http.Serve(w, r, "init.json")
// }
func ksInit(w http.ResponseWriter, r *http.Request) {
	// TODO: make sure to put the return value back api.InitResponse
	resp := api.InitResponse{
		AppName: "greatapp",
		Components: map[string]interface{}{
			"params.libsonnet": "{\n  global: {\n    // User-defined global parameters; accessible to all component and environments, Ex:\n    // replicas: 4,\n  },\n  components: {\n    // Component-level parameters, defined initially from 'ks prototype use ...'\n    // Each object below should correspond to a component in the components/ directory\n  },\n}\n",
		},
		Environments: map[string]interface{}{
			"base.libsonnet": "local components = std.extVar(\"__ksonnet/components\");\ncomponents + {\n  // Insert user-specified overrides here.\n}\n",
			"default": map[string]interface{}{
				"default.jsonnet":  "local base = import \"/Users/tehut/test/environments/base.libsonnet\";\nlocal k = import \"k.libsonnet\";\n\nbase + {\n  // Insert user-specified overrides here. For example if a component is named \"nginx-deployment\", you might have something like:\n  //   \"nginx-deployment\"+: k.deployment.mixin.metadata.labels({foo: \"bar\"})\n}\n",
				"params.libsonnet": "local params = import \"/Users/tehut/test/components/params.libsonnet\";\nparams + {\n  components +: {\n    // Insert component parameter overrides here. Ex:\n    // guestbook +: {\n    //   name: \"guestbook-dev\",\n    //   replicas: params.global.replicas,\n    // },\n  },\n}\n",
				"spec.json":        "{\n  \"server\": \"https://kubecfg-t-apiloadb-1k1vqoxu7q212-666493100.us-west-2.elb.amazonaws.com\",\n  \"namespace\": \"dev-tehut\"\n}",
			},
		}}

	i, _ := json.Marshal(resp)
	s := string(i)
	fmt.Fprintf(w, s)
	// return resp
}

func ksShow(w http.ResponseWriter, r *http.Request) {
	resp := api.ShowResponse{
		Components: map[string]interface{}{
			"params.libsonnet": "{\n  global: {\n    // User-defined global parameters; accessible to all component and environments, Ex:\n    // replicas: 4,\n  },\n  components: {\n    // Component-level parameters, defined initially from 'ks prototype use ...'\n    // Each object below should correspond to a component in the components/ directory\n  },\n}\n",
			"ui.libsonnet":     "{\n  global: {\n    // User-defined global parameters; accessible to all component and environments, Ex:\n    // replicas: 4,\n  },\n  components: {\n    // Component-level parameters, defined initially from 'ks prototype use ...'\n    // Each object below should correspond to a component in the components/ directory\n  },\n}\n",
			"redis.libsonnet":  "{\n  global: {\n    // User-defined global parameters; accessible to all component and environments, Ex:\n    // replicas: 4,\n  },\n  components: {\n    // Component-level parameters, defined initially from 'ks prototype use ...'\n    // Each object below should correspond to a component in the components/ directory\n  },\n}\n",
		}}

	show, _ := json.Marshal(resp)
	s := string(show)
	fmt.Fprintf(w, s)
}

func ksGenerate(w http.ResponseWriter, r *http.Request) {
	// http.ServeFile(w, r, "generate.json")
	resp := api.GenerateResponse{
		Components: map[string]interface{}{
			"params.libsonnet": "{\n  global: {\n    // User-defined global parameters; accessible to all component and environments, Ex:\n    // replicas: 4,\n  },\n  components: {\n    // Component-level parameters, defined initially from 'ks prototype use ...'\n    // Each object below should correspond to a component in the components/ directory\n  },\n}\n",
			"ui.libsonnet":     "{\n  global: {\n    // User-defined global parameters; accessible to all component and environments, Ex:\n    // replicas: 4,\n  },\n  components: {\n    // Component-level parameters, defined initially from 'ks prototype use ...'\n    // Each object below should correspond to a component in the components/ directory\n  },\n}\n",
			"redis.libsonnet":  "{\n  global: {\n    // User-defined global parameters; accessible to all component and environments, Ex:\n    // replicas: 4,\n  },\n  components: {\n    // Component-level parameters, defined initially from 'ks prototype use ...'\n    // Each object below should correspond to a component in the components/ directory\n  },\n}\n",
		}}

	gen, _ := json.Marshal(resp)
	s := string(gen)
	fmt.Fprintf(w, s)
}

func main() {

	limiter = rate.NewLimiter(config.RateLimit, config.RateLimitBurst)
	codeCache = ccache.New(ccache.Configure().MaxSize(config.CacheSize))

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
		mux.HandleFunc("/show", ksShow)
		mux.HandleFunc("/generate", ksGenerate)
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
