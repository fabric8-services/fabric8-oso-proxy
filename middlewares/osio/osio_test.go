package osio

import (
	"net/http"
	"net/url"
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
	expectedUserID := "11111111-1111-1111-1111-11111111"

	t.Run("UserID as header", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "http://f8osoproxy.com", nil)
		req.Header.Set(UserIDHeader, expectedUserID)
		userID := extractUserID(req)
		assert.Equal(t, expectedUserID, userID)
	})
}

func TestRemoveUserID(t *testing.T) {
	userID := "11111111-1111-1111-1111-11111111"
	impersonateGroup := "dummyGroup"

	t.Run("UserID as header", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "http://f8osoproxy.com", nil)
		req.Header.Set(UserIDHeader, userID)
		removeUserID(req)
		actualUserID := req.Header.Get(UserIDHeader)
		assert.Empty(t, actualUserID)
		assert.Equal(t, "http://f8osoproxy.com", req.URL.String())
	})

	// See https://github.com/fabric8-services/fabric8-oso-proxy/pull/43
	t.Run("UserID as header with 'Impersonate-Group'", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "http://f8osoproxy.com", nil)
		req.Header.Set(UserIDHeader, userID)
		req.Header.Set(ImpersonateGroupHeader, impersonateGroup)
		removeUserID(req)
		actualUserID := req.Header.Get(UserIDHeader)
		actualImpersonateGroup := req.Header.Get(ImpersonateGroupHeader)
		assert.Empty(t, actualUserID)
		assert.Empty(t, actualImpersonateGroup)
		assert.Equal(t, "http://f8osoproxy.com", req.URL.String())
	})
}

func TestStripPathPrefix(t *testing.T) {
	tables := []struct {
		reqPath      string
		expectedPath string
	}{
		{"/api/api/anything", "/api/anything"},
		{"/api/anything", "/api/anything"},
		{"/api/oapi/anything", "/oapi/anything"},
		{"/oapi/anything", "/oapi/anything"},
		{"/metrics/anything", "/anything"},
		{"/console/anything", "/anything"},
		{"/logs/anything", "/anything"},
		{"/anything", "/anything"},
		{"/", "/"},
		{"/apis/apps/v1/namespaces/k8s-image-puller/daemonsets", "/apis/apps/v1/namespaces/k8s-image-puller/daemonsets"},
	}

	for _, table := range tables {
		req := createRequestWithPath(table.reqPath)
		reqType := getRequestType(req)
		reqType.stripPathPrefix(req)
		assertRequestPath(t, req, table.expectedPath)
	}
}

func TestGetNamespaceName(t *testing.T) {
	tables := []struct {
		reqPath    string
		wantNsName string
	}{
		{"/apis/apps/v1/namespaces/k8s-image-puller/daemonsets", "k8s-image-puller"},
		{"/apis/apps/v1/namespaces/k8s-image-puller/anything/namespaces/second-namespace/daemonsets", "k8s-image-puller"},
		{"", ""},
		{"/apis/apps/v1/ns/k8s-image-puller/daemonsets", ""},
		{"/apis/apps/v1/namespaces/", ""},
		{"/apis/apps/v1/namespaces/k8s-image-puller", "k8s-image-puller"},
	}
	for _, table := range tables {
		gotNsName := getNamespaceName(table.reqPath)
		assert.Equal(t, table.wantNsName, gotNsName)
	}
}

func createRequestWithPath(path string) *http.Request {
	req := &http.Request{}
	req.URL = &url.URL{Path: path}
	req.RequestURI = req.URL.RequestURI()
	return req
}

func assertRequestPath(t *testing.T, req *http.Request, expectedPath string) {
	t.Helper()
	assert.Equal(t, expectedPath, req.URL.Path)
	assert.Equal(t, expectedPath, req.RequestURI)
}
