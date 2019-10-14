package main

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/labstack/echo"
)

type irisError struct {
	StatusCode    int
	Message       string
	PublicMessage string
}

func (e *irisError) Error() string {
	return e.Message
}

func newError(status int, msg string, pub string) *irisError {
	return &irisError{status, msg, pub}
}

func newUnexpectedError(err error, skip int) *irisError {
	msg := fmt.Sprintf("Unexpected error: %s\n%s", err, stacktrace(skip+1))
	return &irisError{500, msg, "Internal error"}
}

func stacktrace(skip int) string {
	callers := make([]uintptr, 10)
	n := runtime.Callers(skip+1, callers)

	lines := make([]string, n)
	for i, pc := range callers[:n] {
		f := runtime.FuncForPC(pc)
		file, line := f.FileLine(pc)
		lines[i] = fmt.Sprintf("%s:%d %s", file, line, f.Name())
	}

	return strings.Join(lines, "\n")
}

func errorsHandler(err error, c echo.Context) {
	log.Info("Start handler error: ", err)
	var (
		code = http.StatusInternalServerError
		msg  interface{}
	)

	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		msg = he.Message
	} else if e.Debug {
		msg = err.Error()
	} else {
		msg = http.StatusText(code)
	}

	if _, ok := msg.(string); ok {
		msg = echo.Map{"message": msg}
	} else {
		msg = echo.Map{"message": fmt.Sprintf("%v", msg)}
	}

	if !c.Response().Committed {
		if c.Request().Method == echo.HEAD { // Issue #608
			if err = c.NoContent(code); err != nil {
				goto ERROR
			}
		} else {
			if err = c.JSON(code, msg); err != nil {
				goto ERROR
			}
		}
	}
ERROR:
	switch {
	case code == 404: // Because there are requests which cause 404, I log it for debbuging
		log.Errorf("Context: %+v; RemoteAddr: %s; Error: %v", c, c.Request().RemoteAddr, err)
	case code >= 500: // If status code is 5xx, log it out as error
		log.Error(err)
	default: // For less noise, other codes will be debug information
		log.Debug(err)
	}
}
