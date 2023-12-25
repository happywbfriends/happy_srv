package v1

import (
	"fmt"
	metrics "github.com/happywbfriends/metrics/v1"
	"math/rand"
	"net/http"
	"net/url"
	"testing"
)

type TestHandler struct {
	t                  *testing.T
	expectedMethod     string
	expectedParamName  RouterParamName
	expectedParamValue string
}

func (h *TestHandler) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	if r.Method != h.expectedMethod {
		h.t.Fatalf("Expected method '%s' got '%s'", h.expectedMethod, r.Method)
	}
	if h.expectedParamName != "" {
		val := r.Context().Value(h.expectedParamName)
		sval, ok := val.(string)
		if !ok {
			h.t.Fatalf("Expected param '%s' to be a valid string, got '%v'", h.expectedParamName, val)
		}
		if sval != h.expectedParamValue {
			h.t.Fatalf("Expected param %s='%s' got '%s'", h.expectedParamName, h.expectedParamValue, sval)
		}
	}
}

func NewRequestMock(method, pathAndQuery string) *http.Request {

	parsedUrl, err := url.Parse(fmt.Sprintf("https://example.com%s", pathAndQuery))
	if err != nil {
		panic(err)
	}

	return &http.Request{
		Method:           method,
		URL:              parsedUrl,
		Proto:            "HTTP/1.0",
		ProtoMajor:       1,
		ProtoMinor:       0,
		Body:             nil,
		GetBody:          nil,
		ContentLength:    0,
		TransferEncoding: []string{},
		Close:            true,
		Host:             "",
		RequestURI:       parsedUrl.RequestURI(),
	}
}

type FailTestHandler struct {
	t   *testing.T
	msg string
}

func (h *FailTestHandler) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	h.t.Fatalf("%s %s failed: %s", r.Method, r.RequestURI, h.msg)
}

// Примерное соотношение:
//	NanoRouter:	370 ns/op
//	ServeMux:	2800 ns/op
func BenchmarkNanoVsServeMux(b *testing.B) {
	voidHandleFunc := func(writer http.ResponseWriter, request *http.Request) {}

	nano := NewRouter(&metrics.NoHTTPServerMetrics{})
	mux := http.NewServeMux()

	// Запросы, на которых будем смотреть производительность
	var requests []*http.Request

	// добавляем N рандомных роутов
	for i := 0; i < 20; i++ {
		path := fmt.Sprintf("/some/path/%d/%d", rand.Int(), rand.Int())

		nano.HandleFunc("GET", path, voidHandleFunc)
		mux.HandleFunc(path, voidHandleFunc)

		uri := "http://example.com" + path + "?a=b&c=d"
		parsedUrl, _ := url.Parse(uri)
		req := &http.Request{
			Method:           "GET",
			URL:              parsedUrl,
			Proto:            "HTTP/1.0",
			ProtoMajor:       1,
			ProtoMinor:       0,
			Body:             nil,
			GetBody:          nil,
			ContentLength:    0,
			TransferEncoding: []string{},
			Close:            true,
			Host:             "",
			RequestURI:       parsedUrl.RequestURI(),
		}
		requests = append(requests, req)
	}

	benchmarks := []struct {
		name   string
		router http.Handler
	}{
		{"nano", nano},
		{"ServeMux", mux},
	}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for _, r := range requests {
					bm.router.ServeHTTP(nil, r)
				}
			}
		})
	}
}

func TestNanoRouter_ServeHTTP(t *testing.T) {
	tests := []struct {
		templateMethod     string
		templatePath       string
		callUri            string
		expectedParamName  RouterParamName
		expectedParamValue string
	}{
		{"GET", "/", "/", "", ""},
		{"GET", "/a/b/c", "/a/b/c?x=y", "", ""},
		{"GET", "/api/:id", "/api/123", "id", "123"},
	}
	for _, tt := range tests {
		t.Run(tt.templatePath, func(t *testing.T) {
			nano := NewRouter(&metrics.NoHTTPServerMetrics{})

			nano.NotFound = &FailTestHandler{t, "unexpected NotFound handler call"}
			nano.MethodNotAllowed = &FailTestHandler{t, "unexpected MethodNotAllowed handler call"}

			nano.Handle(tt.templateMethod, tt.templatePath, &TestHandler{
				t:                  t,
				expectedMethod:     tt.templateMethod,
				expectedParamName:  tt.expectedParamName,
				expectedParamValue: tt.expectedParamValue,
			})

			nano.ServeHTTP(nil, NewRequestMock(tt.templateMethod, tt.callUri))
		})
	}
}
