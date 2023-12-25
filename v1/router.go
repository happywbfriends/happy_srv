package v1

import (
	"context"
	"fmt"
	metrics "github.com/happywbfriends/metrics/v1"
	"net/http"
	"strings"
)

type _Route struct {
	paramName string // используется только в param-routes
	get       http.Handler
	post      http.Handler
	put       http.Handler
	delete    http.Handler
	patch     http.Handler
	options   http.Handler
}

func (rt *_Route) ptr(method string) *http.Handler {
	switch method {
	case http.MethodGet:
		return &rt.get
	case http.MethodPost:
		return &rt.post
	case http.MethodPut:
		return &rt.put
	case http.MethodDelete:
		return &rt.delete
	case http.MethodPatch:
		return &rt.patch
	case http.MethodOptions:
		return &rt.options
	default:
		return nil
	}
}

type RouterParamName string

func isMethodSupported(method string) bool {
	var rt _Route
	return rt.ptr(method) != nil
}

/*
	NanoRouter is thread-unsafe. After starting http listen no changes are allowed
*/
type NanoRouter struct {
	staticRoutes     map[string]*_Route // полностью статичные роуты
	paramRoutes      map[string]*_Route // роуты с поддержкой одного path param, nullable
	NotFound         http.Handler       // если роут не найден
	MethodNotAllowed http.Handler       // если роут найден, но не поддерживает указанный метод
}

func NewRouter(serverMetrics metrics.HTTPServerMetrics) *NanoRouter {
	return &NanoRouter{
		staticRoutes:     make(map[string]*_Route),
		paramRoutes:      nil,
		NotFound:         &handler404{m: serverMetrics},
		MethodNotAllowed: &handler405{m: serverMetrics},
	}
}

func (router *NanoRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// сперва ищем статический роут
	if rt := router.staticRoutes[r.URL.Path]; rt != nil {
		if m := rt.ptr(r.Method); m != nil {
			if *m != nil {
				(*m).ServeHTTP(w, r)
			} else {
				// здесь мы окажемся, если метод не определен для указанного path
				router.MethodNotAllowed.ServeHTTP(w, r)
			}
		} else {
			// здесь мы окажемся, если такой метод вообще не знаком роутеру
			router.MethodNotAllowed.ServeHTTP(w, r)
		}
		return
	}

	// Если не найден статический, смотрим среди роутов с одним path param
	if router.paramRoutes != nil {
		lastSlash := strings.LastIndexByte(r.URL.Path, '/')
		if lastSlash > 0 && lastSlash < (len(r.URL.Path)-1) {
			searchedPath := r.URL.Path[:lastSlash]
			param := r.URL.Path[lastSlash+1:]

			if rt := router.paramRoutes[searchedPath]; rt != nil {
				if m := rt.ptr(r.Method); m != nil {
					if *m != nil {
						// положим значение параметра в контекст
						newCtx := context.WithValue(r.Context(), RouterParamName(rt.paramName), param)
						r = r.WithContext(newCtx)

						(*m).ServeHTTP(w, r)
					} else {
						// здесь мы окажемся, если метод не определен для указанного path
						router.MethodNotAllowed.ServeHTTP(w, r)
					}
				} else {
					// здесь мы окажемся, если такой метод вообще не знаком роутеру
					router.MethodNotAllowed.ServeHTTP(w, r)
				}
				return
			}
		}
	}

	router.NotFound.ServeHTTP(w, r)
}

func (router *NanoRouter) Handle(method, path string, h http.Handler) {
	if h == nil {
		panic("handler is nil")
	}
	if path == "" {
		panic("path is empty")
	}
	if !isMethodSupported(method) {
		panic("method " + method + " not supported by router")
	}

	// статик или с параметром?
	if index := strings.Index(path, "/:"); index != -1 {
		newPath := path[:index]
		paramName := path[index+2:]

		if router.paramRoutes == nil {
			router.paramRoutes = make(map[string]*_Route)
		}

		rt := router.paramRoutes[newPath]
		if rt == nil {
			rt = &_Route{paramName: paramName}
			router.paramRoutes[newPath] = rt
		} else {
			if rt.paramName != paramName {
				panic(fmt.Sprintf("%s %s parameter %s differs from %s declared earlier", method, path, paramName, rt.paramName))
			}
		}
		m := rt.ptr(method)
		*m = h

	} else {

		rt := router.staticRoutes[path]
		if rt == nil {
			rt = &_Route{}
			router.staticRoutes[path] = rt
		}
		m := rt.ptr(method)
		*m = h
	}
}

func (router *NanoRouter) HandleFunc(method, path string, h http.HandlerFunc) {
	router.Handle(method, path, &wrapFunc{h})
}

func (router *NanoRouter) HandleNotFound(h http.Handler) {
	if h == nil {
		panic("handler is nil")
	}
	router.NotFound = h
}

func (router *NanoRouter) HandleNotFoundFunc(h http.HandlerFunc) {
	router.HandleNotFound(&wrapFunc{h})
}

func (router *NanoRouter) HandleMethodNotAllowed(h http.Handler) {
	if h == nil {
		panic("handler is nil")
	}
	router.MethodNotAllowed = h
}

func (router *NanoRouter) HandleMethodNotAllowedFunc(h http.HandlerFunc) {
	router.HandleMethodNotAllowed(&wrapFunc{h})
}

type wrapFunc struct {
	f http.HandlerFunc
}

func (h *wrapFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.f(w, r)
}

/*
type defaultHandler struct {
	code int
}

func (h *defaultHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(h.code)
}

var (
	defaultNotFoundHandler         = defaultHandler{code: http.StatusNotFound}
	defaultMethodNotAllowedHandler = defaultHandler{code: http.StatusMethodNotAllowed}
)
*/

type handler404 struct {
	m metrics.HTTPServerMetrics
}

func (h *handler404) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.m.IncNbRequest("/404", 404, 0)
	w.WriteHeader(http.StatusNotFound)
}

type handler405 struct {
	m metrics.HTTPServerMetrics
}

func (h *handler405) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.m.IncNbRequest("/405", 405, 0)
	w.WriteHeader(http.StatusMethodNotAllowed)
}
