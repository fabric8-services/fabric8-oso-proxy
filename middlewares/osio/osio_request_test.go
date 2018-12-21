package osio

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessAuthParam(t *testing.T) {
	target := "/test?watch=true&access_token=abcd1234"
	req := httptest.NewRequest(http.MethodGet, target, nil)

	processAuthParam(req)

	authHeader := req.Header.Get("Authorization")
	assert.Equal(t, "Bearer abcd1234", authHeader)
	assert.Empty(t, req.URL.Query().Get("access_token"))
	assert.Equal(t, "true", req.URL.Query().Get("watch"))
	assert.Equal(t, "/test?watch=true", req.RequestURI)
}
