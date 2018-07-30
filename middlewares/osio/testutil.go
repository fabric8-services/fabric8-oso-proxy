package osio

import (
	"net/http"
	"net/http/httptest"
	"strings"
)

func createServer(handle func(http.ResponseWriter, *http.Request)) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handle)
	return httptest.NewServer(mux)
}

func serverOSIORequest(osio *OSIOAuth, varifyHandler http.HandlerFunc) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		osio.ServeHTTP(rw, req, varifyHandler)
	}
}

func getTokenFromRequest(req *http.Request) string {
	return strings.TrimPrefix(req.Header.Get(Authorization), "Bearer ")
}
