package api

import (
	"encoding/json"
	"errors"

	"github.com/rommapp/grout"
	"github.com/rommapp/grout/core/library"
)

type EventType string

const EventProgress EventType = "progress"

type Event struct {
	Type  EventType     `json:"type"`
	Stage library.Stage `json:"stage,omitempty"`
	Done  int           `json:"done"`
	Total int           `json:"total"`
}

type Code = grout.Code

const (
	CodeNotConfigured     = grout.CodeNotConfigured
	CodeBadDefinition     = grout.CodeBadDefinition
	CodeServerUnreachable = grout.CodeServerUnreachable
	CodeUnauthorized      = grout.CodeUnauthorized
	CodeServerTooOld      = grout.CodeServerTooOld
	CodeBusy              = grout.CodeBusy
	CodeDataDirLocked     = grout.CodeDataDirLocked
	CodeBadRequest        = grout.CodeBadRequest
	CodeCancelled         = grout.CodeCancelled
	CodeStorage           = grout.CodeStorage
	CodeInternal          = grout.CodeInternal
	CodeInvalidCode       = grout.CodeInvalidCode
	CodeRateLimited       = grout.CodeRateLimited
)

func Codes() []Code {
	return []Code{
		CodeNotConfigured,
		CodeBadDefinition,
		CodeServerUnreachable,
		CodeUnauthorized,
		CodeServerTooOld,
		CodeBusy,
		CodeDataDirLocked,
		CodeBadRequest,
		CodeCancelled,
		CodeStorage,
		CodeInternal,
		CodeInvalidCode,
		CodeRateLimited,
	}
}

type ErrorBody struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

type Envelope struct {
	OK     bool       `json:"ok"`
	Result any        `json:"result,omitempty"`
	Error  *ErrorBody `json:"error,omitempty"`
}

type requestError struct {
	err error
}

func (e *requestError) Error() string {
	return e.err.Error()
}

func (e *requestError) Unwrap() error {
	return e.err
}

func badRequest(err error) error {
	return &requestError{err: err}
}

func codeOf(err error) Code {
	var re *requestError
	if errors.As(err, &re) {
		return CodeBadRequest
	}
	return grout.CodeOf(err)
}

func errorBody(err error) *ErrorBody {
	return &ErrorBody{Code: codeOf(err), Message: err.Error()}
}

func ErrorEnvelope(err error) []byte {
	if err == nil {
		err = errors.New("unknown error")
	}
	return encode(Envelope{Error: errorBody(err)})
}

func success(result any) []byte {
	return encode(Envelope{OK: true, Result: result})
}

func failure(result any, err error) []byte {
	return encode(Envelope{Result: result, Error: errorBody(err)})
}

func encode(e Envelope) []byte {
	b, err := json.Marshal(e)
	if err != nil {
		b, _ = json.Marshal(Envelope{Error: &ErrorBody{Code: CodeInternal, Message: "encoding result: " + err.Error()}})
	}
	return b
}
