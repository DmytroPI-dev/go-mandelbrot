package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
)

func startLocalHTTPServer(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/render", localRenderHTTPHandler)

	log.Printf("local backend listening on %s in %s mode", addr, currentHandlerMode())
	return http.ListenAndServe(addr, mux)
}

func localRenderHTTPHandler(w http.ResponseWriter, r *http.Request) {
	setLocalCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	request := events.APIGatewayProxyRequest{
		QueryStringParameters: queryParams(r),
		Headers:               map[string]string{"Origin": r.Header.Get("Origin")},
	}

	var (
		response events.APIGatewayProxyResponse
		err      error
	)
	switch currentHandlerMode() {
	case renderModeOrchestrator:
		response, err = orchestratorHandler(context.Background(), request)
	default:
		response, err = handler(context.Background(), request)
	}
	if err != nil && response.StatusCode == 0 {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for key, value := range response.Headers {
		w.Header().Set(key, value)
	}
	setLocalCORSHeaders(w)

	statusCode := response.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	w.WriteHeader(statusCode)

	body := []byte(response.Body)
	if response.IsBase64Encoded {
		decoded, decodeErr := base64.StdEncoding.DecodeString(response.Body)
		if decodeErr != nil {
			_, _ = w.Write([]byte(fmt.Sprintf("failed to decode base64 response: %v", decodeErr)))
			return
		}
		body = decoded
	}
	_, _ = w.Write(body)
}

func queryParams(r *http.Request) map[string]string {
	values := r.URL.Query()
	params := make(map[string]string, len(values))
	for key, value := range values {
		if len(value) > 0 {
			params[key] = value[0]
		}
	}
	return params
}

func setLocalCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}
