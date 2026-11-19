package main

import (
	"net/http"
	"testing"
)

func TestEmbeddedRootCAsInstalled(t *testing.T) {
	tr := http.DefaultTransport.(*http.Transport)
	if tr.TLSClientConfig == nil || tr.TLSClientConfig.RootCAs == nil {
		t.Fatal("no embedded root CA pool on the default transport")
	}
}
