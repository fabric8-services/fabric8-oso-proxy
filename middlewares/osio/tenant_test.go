package middlewares

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

/*

Test Resolvers via fake server controlling response
Test OSOOAuth with fake Resolver
*/

func TestTenantServiceClient(t *testing.T) {
	type scenario struct {
		function func(rw http.ResponseWriter, r *http.Request)
		url      string
		err      error
	}

	tests := []scenario{
		{tenantService200OK, "http://www.test.org", nil},
		{serviceError(401), "", errors.New("")},
		{serviceError(403), "", errors.New("")},
		{serviceError(500), "", errors.New("")},
	}

	for index, test := range tests {
		t.Run(fmt.Sprintf("Scenario%v", index), func(t *testing.T) {
			server := createTenantServer(test.function)
			defer server.Close()
			locator := CreateTenantLocator(
				http.DefaultClient,
				"http://"+server.Listener.Addr().String(),
			)

			ns, err := locator.GetTenant("xxxxx")
			url := ns.ClusterURL
			assert.Equal(t, test.url, url, "expected URL to be equal")
			if test.err == nil {
				assert.NoError(t, err, "expected no error")
			} else {
				assert.Error(t, err, " expected error")
			}
		})
	}
}

func createTenantServer(handler func(rw http.ResponseWriter, r *http.Request)) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/tenant", handler)
	return httptest.NewServer(mux)
}

func tenantService200OK(rw http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		rw.Header().Set("content-type", "application/json")
		rw.WriteHeader(200)
		rw.Write([]byte(`{
			"data": {
				"attributes": {
					"namespaces": [
						{
							"name": "che",
							"cluster-url": "http://www.test.org"
						}
					]
				}
			}
		}`))
	}
}

func serviceError(status int) func(rw http.ResponseWriter, r *http.Request) {
	return func(rw http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			rw.WriteHeader(status)
		}
	}
}
