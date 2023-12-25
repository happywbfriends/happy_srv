package v1

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/happywbfriends/logger/v0"
	"io"
	"net/http"
)

type RequestContext interface {
	Context() context.Context
	Log() logger.Logger
	RequestId() string
	ReadJSONBody(dest interface{}) error
	Writer() http.ResponseWriter
	// Все Send... методы не возвращают никаких ошибок, поскольку предполагается, что отправка ответа - это последний
	// этап обработки любого запроса, и в случае проблем с записью ответа пользовательский код все равно не может
	// ничего сделать кроме как отписаться в логи, что данные методы и делают за него.
	Send(status int, contentType string, dataOpt []byte)
	SendText(status int, text string)
	SendJSON(status int, obj any)
}

type requestContext struct {
	log       logger.Logger
	w         *responseStatusInterceptor
	r         *http.Request
	requestId string
}

func (m *requestContext) Context() context.Context {
	return m.r.Context()
}

func (m *requestContext) Log() logger.Logger {
	return m.log
}

func (m *requestContext) RequestId() string {
	return m.requestId
}

func (m *requestContext) Writer() http.ResponseWriter {
	return m.w
}

func (m *requestContext) Send(status int, contentType string, dataOpt []byte) {
	if contentType != "" {
		m.w.Header().Set(HeaderContentType, contentType)
	}
	m.w.WriteHeader(status)
	if len(dataOpt) > 0 {
		if _, writeErr := m.w.Write(dataOpt); writeErr != nil {
			// здесь нет смысла возвращать error, поскольку будет попытка переотправить новый ответ, а она провалится
			m.log.Warnf("Error writing: %s", writeErr.Error())
		}
	}
}

func (m *requestContext) SendText(status int, text string) {
	m.Send(status, ContentTypeText, []byte(text))
}

func (m *requestContext) SendJSON(status int, obj interface{}) {
	d, err := json.Marshal(obj)
	if err != nil {
		m.log.Warnf("%s %s: error marshalling object %v: %s", m.r.Method, m.r.URL.Path, obj, err.Error())
		m.SendText(http.StatusInternalServerError, "Marshalling error")
		return
	}
	m.Send(status, ContentTypeJSON, d)
}

// эта ошибка не моэет возникнуть в JSON RPC, поэтому там 0
var unsupportedContentType = errors.New("invalid Content-Type: expected 'application/json'")

func (m *requestContext) ReadJSONBody(dest any) error {
	if m.r.Header.Get(HeaderContentType) != ContentTypeJSON {
		return unsupportedContentType
	}

	body, err := io.ReadAll(m.r.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, dest); err != nil {
		return err
	}
	return nil
}
