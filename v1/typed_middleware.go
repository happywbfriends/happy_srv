package v1

import (
	"errors"
	"net/http"
	"reflect"
)

const (
	tagQuery   = "query"
	tagHeader  = "header"
	tagCookie  = "cookie"
	tagContext = "context"
	tagBody    = "json"
)

type TypedMiddleware interface {
	Handler() HttpHandler
}

type typedMiddleware[RequestT, ResponseT any] struct {
	readBody    bool
	readHeaders bool
	readCookie  bool
	readContext bool
	readQuery   bool
	handler     func(*RequestT, RequestContext) (*ResponseT, error)
}

func NewTypedMiddleware[RequestT, ResponseT any](h func(*RequestT, RequestContext) (*ResponseT, error)) TypedMiddleware {

	var req RequestT
	refType := reflect.TypeOf(req)

	var query, header, cookie, context, body bool
	for i := 0; i < refType.NumField(); i++ {
		tf := refType.Field(i)
		if _, ok := tf.Tag.Lookup(tagQuery); ok {
			query = true
		}
		if _, ok := tf.Tag.Lookup(tagHeader); ok {
			header = true
		}
		if _, ok := tf.Tag.Lookup(tagCookie); ok {
			cookie = true
		}
		if _, ok := tf.Tag.Lookup(tagContext); ok {
			context = true
		}
		if _, ok := tf.Tag.Lookup(tagBody); ok {
			body = true
		}
	}

	return &typedMiddleware[RequestT, ResponseT]{
		readBody:    body,
		readHeaders: header,
		readCookie:  cookie,
		readContext: context,
		readQuery:   query,
		handler:     h,
	}
}

func (c *typedMiddleware[RequestT, ResponseT]) Handler() HttpHandler {
	return func(r *http.Request, rc RequestContext) (proceed bool) {
		var req RequestT

		if c.readBody {
			if err := rc.ReadJSONBody(&req); err != nil {
				rc.SendText(500, "Error parsing body")
				return false
			}
		}

		if c.readHeaders {
			_, err := Enrich(&req, tagHeader, func(name string) (value any, found bool) {
				value = r.Header.Get(name)
				return value, value != ""
			})
			if err != nil {
				//return false, xerror.NewBadRequestDetailed("error compiling request", err.Error())
				return false
			}
		}

		if c.readCookie {
			_, err := Enrich(&req, tagCookie, func(name string) (value any, found bool) {
				cookie, err := r.Cookie(name)
				if err != nil {
					if errors.Is(err, http.ErrNoCookie) {
						return nil, false
					}
					return nil, false // как бы других ошибок вроде и нет
				}
				if cookie == nil {
					return nil, false // вроде не должно быть такого, но мало ли
				}
				return cookie.Value, true
			})
			if err != nil {
				//return false, xerror.NewBadRequestDetailed("error compiling request", err.Error())
				return false
			}
		}

		/*
			if c.readContext {
				_, err := Enrich(&req, tagContext, func(name string) (value any, found bool) {
					value, found = mw.Values()[name]
					return
				})
				if err != nil {
					//return false, xerror.NewBadRequestDetailed("error compiling request", err.Error())
					return false
				}
			}
		*/

		if c.readQuery {
			_, err := Enrich(&req, tagQuery, func(name string) (value any, found bool) {
				values, _found := r.URL.Query()[name]
				if _found && len(values) > 0 {
					return values[0], true
				}
				return nil, false
			})
			if err != nil {
				//return false, xerror.NewBadRequestDetailed("error compiling request", err.Error())
				return false
			}
		}

		var result *ResponseT
		var err error

		result, err = c.handler(&req, rc)

		if err != nil {
			return false
		}
		rc.SendJSON(http.StatusOK, result)

		return false
	}
}
