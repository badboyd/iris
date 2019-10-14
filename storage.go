package main

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"time"
)

var (
	storageClient *http.Client
)

type limitReader struct {
	r    io.ReadCloser
	left int
}

func (lr *limitReader) Read(p []byte) (n int, err error) {
	n, err = lr.r.Read(p)
	lr.left = lr.left - n

	if err == nil && lr.left < 0 {
		err = errSourceFileTooBig
	}

	return
}

func (lr *limitReader) Close() error {
	return lr.r.Close()
}

func initStorage() {
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        config.Iris.Concurrency,
		MaxIdleConnsPerHost: config.Iris.Concurrency,
		DisableCompression:  true,
		Dial:                (&net.Dialer{KeepAlive: 600 * time.Second}).Dial,
	}

	if config.Iris.IgnoreSslVerification {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	if config.Storage.GCS.Enabled {
		transport.RegisterProtocol("gcs", newGCSTransport())
	}

	storageClient = &http.Client{
		Timeout:   config.Iris.Timeout,
		Transport: transport,
	}
}
