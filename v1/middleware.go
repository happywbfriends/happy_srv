package v1

import (
	"context"
	"github.com/google/uuid"
	"github.com/happywbfriends/logger/v0"
	metrics "github.com/happywbfriends/metrics/v1"
	"net/http"
	"time"
)

// Ошибка в ответе имеет тип IHttpError, потому что для error всегда будет непонятно, считать ее 400 или 500
type HttpHandler func(r *http.Request, rc RequestContext) (proceed bool)

type Values map[string]interface{}

type Middleware interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

type middleware struct {
	log                logger.Logger
	handlers           []HttpHandler
	requestTimeout     time.Duration
	withMetrics        bool
	metrics            metrics.HTTPServerMetrics
	metricsMethod      string
	requestIdGenerator func(*http.Request) string
	maxReadBytes       int64
}

func NewMiddleware(log logger.Logger) Middleware {
	return &middleware{
		log:         log,
		withMetrics: false,
	}
}

func (mw *middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Надо сделать обработку 404, 405, иначе мы про них не узнаем, а нам надо банить тех, кто часто 400-ит
	var startTm time.Time
	statusCode := 0 // 0 чтобы точно увидеть в графане, если забыли выставить код в 200
	if mw.withMetrics {
		startTm = time.Now()
		defer func() {
			mw.metrics.ObserveRequestDuration(mw.metricsMethod, statusCode, 0, time.Since(startTm))
			mw.metrics.IncNbRequest(mw.metricsMethod, statusCode, 0)
		}()
	}

	// RequestId
	requestId := r.Header.Get(HeaderRequestId)
	if requestId == "" && mw.requestIdGenerator != nil {
		requestId = mw.requestIdGenerator(r)
	}
	if requestId != "" {
		w.Header().Set(HeaderRequestId, requestId)
	}

	// Timeout
	if mw.requestTimeout > 0 {
		// For incoming server requests, the context is canceled when the client's connection closes,
		// the request is canceled (with HTTP/2), or when the ServeHTTP method returns.
		newCtx, cancel := context.WithTimeout(r.Context(), mw.requestTimeout)
		defer cancel()

		r = r.WithContext(newCtx)
	}

	// Max bytes
	if mw.maxReadBytes > 0 {
		// https://www.alexedwards.net/blog/how-to-properly-parse-a-json-request-body
		r.Body = http.MaxBytesReader(w, r.Body, mw.maxReadBytes)
	}

	interceptor := newResponseStatusInterceptor(w)

	rc := requestContext{
		log:       mw.log.With("request-id", requestId),
		w:         interceptor,
		r:         r,
		requestId: requestId,
	}

	for _, h := range mw.handlers {
		proceed := h(r, &rc)
		if !proceed {
			break
		}
	}

	statusCode = rc.w.statusCode
}

func (mw *middleware) Use(h HttpHandler) *middleware {
	mw.handlers = append(mw.handlers, h)
	return mw
}

func (mw *middleware) WithMetrics(m metrics.HTTPServerMetrics, method string) *middleware {
	mw.metrics = m
	mw.metricsMethod = method
	return mw
}

func (mw *middleware) WithTimeoutContext(timeout time.Duration) *middleware {
	mw.requestTimeout = timeout
	return mw
}

func (mw *middleware) WithRequestIdGenerator(g func(*http.Request) string) *middleware {
	mw.requestIdGenerator = g
	return mw
}

func RequestIdGeneratorUUID(*http.Request) string {
	UUID, err := uuid.NewRandom()
	if err == nil {
		return UUID.String()
	} else {
		return ""
	}
}

func (mw *middleware) WithMaxBytesReader(maxBytes int64) *middleware {
	mw.maxReadBytes = maxBytes
	return mw
}
