package main

import (
	"net/http"
	"net/http/pprof"
	"sync"

	"github.com/labstack/echo"
)

type customEchoHandler struct {
	once              sync.Once
	httpHandler       http.Handler
	wrappedHandleFunc echo.HandlerFunc
}
type customHTTPHandler struct {
	serveHTTP func(w http.ResponseWriter, r *http.Request)
}

func (c *customHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.serveHTTP(w, r)
}

func (ceh *customEchoHandler) Handle(c echo.Context) error {
	ceh.once.Do(func() {
		ceh.wrappedHandleFunc = ceh.mustWrapHandleFunc(c)
	})
	return ceh.wrappedHandleFunc(c)
}

func (ceh *customEchoHandler) mustWrapHandleFunc(c echo.Context) echo.HandlerFunc {
	return echo.WrapHandler(ceh.httpHandler)
}

func fromHTTPHandler(httpHandler http.Handler) *customEchoHandler {
	return &customEchoHandler{httpHandler: httpHandler}
}

func fromHandlerFunc(serveHTTP func(w http.ResponseWriter, r *http.Request)) *customEchoHandler {
	return &customEchoHandler{httpHandler: &customHTTPHandler{serveHTTP: serveHTTP}}
}

func initPProf() {
	e.GET("/debug/pprof/", fromHandlerFunc(pprof.Index).Handle)
	e.GET("/debug/pprof/heap", fromHTTPHandler(pprof.Handler("heap")).Handle)
	e.GET("/debug/pprof/goroutine", fromHTTPHandler(pprof.Handler("goroutine")).Handle)
	e.GET("/debug/pprof/block", fromHTTPHandler(pprof.Handler("block")).Handle)
	e.GET("/debug/pprof/threadcreate", fromHTTPHandler(pprof.Handler("threadcreate")).Handle)
	e.GET("/debug/pprof/cmdline", fromHandlerFunc(pprof.Cmdline).Handle)
	e.GET("/debug/pprof/profile", fromHandlerFunc(pprof.Profile).Handle)
	e.GET("/debug/pprof/symbol", fromHandlerFunc(pprof.Symbol).Handle)
	e.GET("/debug/pprof/trace", fromHandlerFunc(pprof.Trace).Handle)
}
