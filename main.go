package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"time"

	"golang.org/x/time/rate"
)

const (
	// rateLimit is the number of requests per second we want to allow
	rateLimit rate.Limit = 100
)

var limiter *rate.Limiter
var busyMessage = "Server is busy, please try again"

func init() {
	limiter = rate.NewLimiter(rateLimit, 1)
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
func errorResponse(err error) string {
	errorString := err.Error()
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
func runJsonnet(code string) ([]byte, bool, error) {
	var err error
	cmd := exec.Command("jsonnet", "-e", code)

	// Don't let the command run for more than 1s
	var timer *time.Timer
	wasKilled := false
	timer = time.AfterFunc(time.Second, func() {
		timer.Stop()
		if cmd.ProcessState == nil {
			log.Printf("Killing jsonnet after 1s deadline\n")
			cmd.Process.Kill()
		}
	})
	defer timer.Stop()

	outBytes, err := cmd.CombinedOutput()

	// Before we check err: if we killed it ourselves, just return that we're
	// busy.
	if wasKilled {
		return nil, true, fmt.Errorf(busyMessage)
	}

	// If we got an error running, return the stdout/stderr, if available, as the error.
	if err != nil {
		err = fmt.Errorf(string(outBytes))
	}

	return outBytes, false, err
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
		w.Write([]byte(errorResponse(fmt.Errorf(busyMessage))))
		return
	}
	limiter.Wait(context.TODO())

	// Decode the body and convert it
	decoder := json.NewDecoder(r.Body)
	var req JsonnetRequest
	err := decoder.Decode(&req)
	if err != nil {
		fmt.Fprint(w, errorResponse(err))
		return
	}
	outBytes, wasKilled, err := runJsonnet(req.Code)

	if err != nil {
		// If we killed the process, return too many requests... else, just have it
		// be 400 bad request, since it should only be possible to fail if their
		// jsonnet input had something wrong with it.)
		if wasKilled {
			w.WriteHeader(http.StatusTooManyRequests)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		fmt.Fprint(w, errorResponse(err))
	} else {
		fmt.Fprint(w, successResponse(string(outBytes)))
	}
	return
}

func main() {
	log.Println("Starting server")

	http.HandleFunc("/", handler)
	http.ListenAndServe(":8080", nil)
}
