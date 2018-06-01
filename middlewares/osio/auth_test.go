package middlewares

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtracToken(t *testing.T) {
	tables := []struct {
		authHeader    string
		expectedToken string
	}{
		{"Bear 1111", "1111"},
		{"1111", "1111"},
	}

	for _, table := range tables {
		actualToken, err := extractToken(table.authHeader)
		assert.Nil(t, err)
		if actualToken != table.expectedToken {
			t.Errorf("Incorrect token, want:%s, got:%s", table.expectedToken, actualToken)
		}
	}
}

func TestExtractUserID(t *testing.T) {
	expectedUserID := "john"

	t.Run("UserID as header", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "http://f8osoproxy.com", nil)
		req.Header.Set(UserIDHeader, expectedUserID)
		userID := extractUserID(req)
		assert.Equal(t, expectedUserID, userID)
	})

	t.Run("UserID as query param test1", func(t *testing.T) {
		urlWithParam := fmt.Sprintf("http://f8osoproxy.com/some/path?%s=%s", UserIDParam, expectedUserID)
		req, _ := http.NewRequest(http.MethodGet, urlWithParam, nil)
		userID := extractUserID(req)
		assert.Equal(t, expectedUserID, userID)
	})

	t.Run("UserID as query param test2", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "http://f8osoproxy.com", nil)
		q := req.URL.Query()
		q.Set(UserIDParam, expectedUserID)
		req.URL.RawQuery = q.Encode()
		userID := extractUserID(req)
		assert.Equal(t, expectedUserID, userID)
	})

	t.Run("UserID as part of path", func(t *testing.T) {
		adhocURL := fmt.Sprintf("http://f8osoproxy.com/?%s=%s/some/path", UserIDParam, expectedUserID)
		req, _ := http.NewRequest(http.MethodGet, adhocURL, nil)
		userID := extractUserID(req)
		assert.Equal(t, expectedUserID, userID)
	})
}

func TestRemoveUserID(t *testing.T) {
	userID := "john"

	t.Run("UserID as header", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "http://f8osoproxy.com", nil)
		req.Header.Set(UserIDHeader, userID)
		removeUserID(req)
		actualUserID := req.Header.Get(UserIDHeader)
		assert.Empty(t, actualUserID)
	})

	t.Run("UserID as query param", func(t *testing.T) {
		urlWithParam := fmt.Sprintf("http://f8osoproxy.com/some/path?%s=%s", UserIDParam, userID)
		req, _ := http.NewRequest(http.MethodGet, urlWithParam, nil)
		removeUserID(req)
		actualUserID := req.URL.Query().Get(UserIDParam)
		assert.Empty(t, actualUserID)
	})

	t.Run("UserID as part of path", func(t *testing.T) {
		path := "/some/path"
		adhocURL := fmt.Sprintf("http://f8osoproxy.com/?%s=%s%s", UserIDParam, userID, path)
		req, _ := http.NewRequest(http.MethodGet, adhocURL, nil)
		removeUserID(req)
		actualUserID := req.URL.Query().Get(UserIDParam)
		assert.Empty(t, actualUserID)
		assert.Equal(t, path, req.URL.Path)
		assert.Equal(t, path, req.RequestURI)
	})
}
