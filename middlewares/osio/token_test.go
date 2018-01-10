package middlewares

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTokenServiceClient(t *testing.T) {
	type scenario struct {
		function func(rw http.ResponseWriter, r *http.Request)
		url      string
		err      error
	}

	tests := []scenario{
		{tokenService200OK, "yyyyyy", nil},
		{serviceError(401), "", errors.New("")},
		{serviceError(403), "", errors.New("")},
		{serviceError(500), "", errors.New("")},
	}

	for index, test := range tests {
		t.Run(fmt.Sprintf("Scenario%v", index), func(t *testing.T) {
			server := createTokenServer(test.function)
			locator := CreateTenantTokenLocator(
				http.DefaultClient,
				"http://"+server.Listener.Addr().String()+"/",
			)

			url, err := locator("xxxxx", "http://x.com")
			server.Close()

			assert.Equal(t, test.url, url, "expected URL to be equal")
			if test.err == nil {
				assert.NoError(t, err, "expected no error")
			} else {
				assert.Error(t, err, " expected error")
			}
		})
	}
}

func createTokenServer(handler func(rw http.ResponseWriter, r *http.Request)) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", handler)
	return httptest.NewServer(mux)
}

func tokenService200OK(rw http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		rw.Header().Set("content-type", "application/json")
		rw.WriteHeader(200)
		rw.Write([]byte(`{
			"access_token": "yyyyyy",
			"scope": "read,write",
			"type": "bearer"
		}`))
	}
}
