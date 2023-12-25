package v1

import "net/http"

type responseStatusInterceptor struct {
	http.ResponseWriter
	statusCode int
}

func newResponseStatusInterceptor(w http.ResponseWriter) *responseStatusInterceptor {
	return &responseStatusInterceptor{w, http.StatusOK}
}

func (rsi *responseStatusInterceptor) WriteHeader(code int) {
	rsi.statusCode = code
	rsi.ResponseWriter.WriteHeader(code)
}
