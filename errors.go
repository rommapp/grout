package grout

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/url"
	"os"
	"syscall"

	"github.com/rommapp/grout/core/romm"
	"modernc.org/sqlite"
)

type Code string

const (
	CodeNotConfigured     Code = "not_configured"
	CodeBadDefinition     Code = "bad_definition"
	CodeServerUnreachable Code = "server_unreachable"
	CodeUnauthorized      Code = "unauthorized"
	CodeServerTooOld      Code = "server_too_old"
	CodeBusy              Code = "busy"
	CodeDataDirLocked     Code = "data_dir_locked"
	CodeBadRequest        Code = "bad_request"
	CodeCancelled         Code = "cancelled"
	CodeStorage           Code = "storage"
	CodeInternal          Code = "internal"
	CodeInvalidCode       Code = "invalid_code"
	CodeRateLimited       Code = "rate_limited"
	CodeBadArchive        Code = "bad_archive"
	CodeMissingOnServer   Code = "missing_on_server"
)

type Error struct {
	Code Code
	Err  error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return string(e.Code)
	}
	return e.Err.Error()
}

func (e *Error) Unwrap() error {
	return e.Err
}

func CodeOf(err error) Code {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return CodeInternal
}

func newError(code Code, err error) error {
	return &Error{Code: code, Err: err}
}

func classify(ctx context.Context, err error, fallback Code) error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return err
	}
	return &Error{Code: codeFor(ctx, err, fallback), Err: err}
}

func codeFor(ctx context.Context, err error, fallback Code) Code {
	switch {
	case ctx.Err() != nil, errors.Is(err, context.Canceled):
		return CodeCancelled
	case romm.IsUnauthorized(err):
		return CodeUnauthorized
	case romm.IsRateLimited(err):
		return CodeRateLimited
	case isWriteFailure(err):
		return CodeStorage
	case isTransport(err):
		return CodeServerUnreachable
	case isStorage(err):
		return CodeStorage
	}
	return fallback
}

func isTransport(err error) bool {
	var status *romm.StatusError
	var urlErr *url.Error
	var netErr net.Error
	var syntax *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	return errors.As(err, &status) || errors.As(err, &urlErr) || errors.As(err, &netErr) ||
		errors.As(err, &syntax) || errors.As(err, &typeErr)
}

func isWriteFailure(err error) bool {
	var pathErr *fs.PathError
	var linkErr *os.LinkError
	return errors.As(err, &pathErr) || errors.As(err, &linkErr) || errors.Is(err, syscall.ENOSPC)
}

func isStorage(err error) bool {
	var dbErr *sqlite.Error
	var sysErr *os.SyscallError
	return errors.As(err, &dbErr) || isWriteFailure(err) || errors.As(err, &sysErr) ||
		errors.Is(err, sql.ErrConnDone) || errors.Is(err, sql.ErrTxDone)
}
