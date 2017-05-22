package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
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

func handler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// Set CORS headers if requested
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", "ksonnet.heptio.com")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers",
			"Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token")
	}
	// And if this is an OPTIONS request, stop here (don't process the body)
	if r.Method == http.MethodOptions {
		return
	}

	decoder := json.NewDecoder(r.Body)
	var req JsonnetRequest
	err := decoder.Decode(&req)
	if err != nil {
		fmt.Fprint(w, errorResponse(err))
		return
	}

	// NOTE: To be honest, I (hausdorff) do not really know anything
	// about HTTP, so it's not clear to me that the right thing to do is
	// to return a 200 OK here.
	//
	// TODO: Kill off Jsonnet jobs that are more than a couple seconds
	// long.
	//
	outBytes, err := exec.Command("jsonnet", "-e", req.Code).Output()
	if err != nil {
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
