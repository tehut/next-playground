package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"time"

	"golang.org/x/time/rate"
)

const (
	// rateLimit is the number of requests per second we want to allow
	rateLimit         rate.Limit = 20
	rateLimitBurst               = 30
	jsonnetRunTimeout            = 5 * time.Second
)

var limiter *rate.Limiter
var errBusy = errors.New("Server is busy, please try again")
var errTimeout = errors.New("Jsonnet evaluation timed out")

func init() {
	limiter = rate.NewLimiter(rateLimit, rateLimitBurst)
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

	outBytes, err := exec.CommandContext(ctx, "jsonnet", "-e", code).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		err = errTimeout
	}
	return string(outBytes), err
}

func handler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// Set CORS headers if requested
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		// TODO: set this back once we're done with development
		// w.Header().Set("Access-Control-Allow-Origin", "ksonnet.heptio.com")
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
	log.Println("Starting server at :8080")

	http.HandleFunc("/", handler)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Print("Graceful Exit...")
}
