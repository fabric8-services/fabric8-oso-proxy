package osio

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessAuthParam(t *testing.T) {
	url := "http://osoproxy.com?watch=true&access_token=abcd1234"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)

	processAuthParam(req)

	authHeader := req.Header.Get("Authorization")
	assert.Equal(t, "Bearer abcd1234", authHeader)
	assert.Empty(t, req.URL.Query().Get("access_token"))
	assert.Equal(t, "true", req.URL.Query().Get("watch"))
}
