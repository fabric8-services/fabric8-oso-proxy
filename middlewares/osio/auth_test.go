package middlewares

import (
	"fmt"
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

	t.Run("UserID as part of ugly url parameter produced by kubernetes client for 'exec' calls", func(t *testing.T) {
		adhocURL := fmt.Sprintf("http://f8osoproxy.com?%s=%s/some/path/to/pod/exec?command=date&tty=true&stdin=true&stdout=true&stderr=true", UserIDParam, expectedUserID)
		req, _ := http.NewRequest(http.MethodGet, adhocURL, nil)
		userID := extractUserID(req)
		assert.Equal(t, expectedUserID, userID)
	})

	t.Run("UserID as part of ugly url segment produced by kubernetes client for 'exec' calls", func(t *testing.T) {
		adhocURL := fmt.Sprintf("http://f8osoproxy.com/?%s=%s/some/path/to/pod/exec?command=date&tty=true&stdin=true&stdout=true&stderr=true", UserIDParam, expectedUserID)
		req, _ := http.NewRequest(http.MethodGet, adhocURL, nil)
		userID := extractUserID(req)
		assert.Equal(t, expectedUserID, userID)
	})
}

func TestRemoveUserID(t *testing.T) {
	userID := "11111111-1111-1111-1111-11111111"

	t.Run("UserID as header", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "http://f8osoproxy.com", nil)
		req.Header.Set(UserIDHeader, userID)
		removeUserID(req)
		actualUserID := req.Header.Get(UserIDHeader)
		assert.Empty(t, actualUserID)
		assert.Equal(t, "http://f8osoproxy.com", req.URL.String())
	})

	t.Run("UserID as query param", func(t *testing.T) {
		urlWithParam := fmt.Sprintf("http://f8osoproxy.com/some/path?%s=%s", UserIDParam, userID)
		req, _ := http.NewRequest(http.MethodGet, urlWithParam, nil)
		removeUserID(req)
		actualUserID := req.URL.Query().Get(UserIDParam)
		assert.Empty(t, actualUserID)
		assert.Equal(t, "http://f8osoproxy.com/some/path", req.URL.String())
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
		assert.Equal(t, "http://f8osoproxy.com/some/path", req.URL.String())
	})

	t.Run("UserID as part of path with query parameters", func(t *testing.T) {
		pathWithQueryParam := "/some/path?key=value"
		adhocURL := fmt.Sprintf("http://f8osoproxy.com/?%s=%s%s", UserIDParam, userID, pathWithQueryParam)
		req, _ := http.NewRequest(http.MethodGet, adhocURL, nil)
		removeUserID(req)

		actualUserID := req.URL.Query().Get(UserIDParam)
		assert.Empty(t, actualUserID)

		queryParamValue := req.URL.Query().Get("key")
		assert.Equal(t, "value", queryParamValue)

		assert.Equal(t, "key=value", req.URL.RawQuery)
		assert.Equal(t, pathWithQueryParam, req.RequestURI)
		assert.Equal(t, "http://f8osoproxy.com/some/path?key=value", req.URL.String())
	})

	t.Run("UserID as part of ugly url produced by kubernetes client for 'exec' calls with single query parameter", func(t *testing.T) {
		pathWithParam := "/some/path/to/pod/exec?command=date"
		adhocURL := fmt.Sprintf("http://f8osoproxy.com?%s=%s%s", UserIDParam, userID, pathWithParam)
		req, _ := http.NewRequest(http.MethodGet, adhocURL, nil)
		removeUserID(req)

		actualUserID := req.URL.Query().Get(UserIDParam)
		assert.Empty(t, actualUserID)

		commandQueryParam := req.URL.Query().Get("command")
		assert.Equal(t, "date", commandQueryParam)
		assert.Equal(t, pathWithParam, req.RequestURI)
		assert.Equal(t, "http://f8osoproxy.com/some/path/to/pod/exec?command=date", req.URL.String())
	})

	t.Run("UserID as part of ugly url produced by kubernetes client for 'exec' calls with multiple query parameters", func(t *testing.T) {
		pathWithParams := "/some/path/to/pod/exec?command=date&tty=true&stdin=true&stdout=true&stderr=false"
		adhocURL := fmt.Sprintf("http://f8osoproxy.com?%s=%s%s", UserIDParam, userID, pathWithParams)
		req, _ := http.NewRequest(http.MethodGet, adhocURL, nil)
		removeUserID(req)

		actualUserID := req.URL.Query().Get(UserIDParam)
		assert.Empty(t, actualUserID)

		commandQueryParam := req.URL.Query().Get("command")
		assert.Equal(t, "date", commandQueryParam)

		ttyQueryParam := req.URL.Query().Get("tty")
		assert.Equal(t, "true", ttyQueryParam)

		stdin := req.URL.Query().Get("stdin")
		assert.Equal(t, "true", stdin)

		stdout := req.URL.Query().Get("stdout")
		assert.Equal(t, "true", stdout)

		stderr := req.URL.Query().Get("stderr")
		assert.Equal(t, "false", stderr)

		assert.Equal(t, pathWithParams, req.RequestURI)
		assert.Equal(t, "http://f8osoproxy.com/some/path/to/pod/exec?command=date&tty=true&stdin=true&stdout=true&stderr=false", req.URL.String())
	})

	t.Run("UserID as part of url produced by rh-che via kubernetes client for 'event' calls", func(t *testing.T) {
		pathWithParams := "/api/v1/namespaces/namespace-che/events\u0026watch=true"
		adhocURL := fmt.Sprintf("http://f8osoproxy.com?%s=%s%s", UserIDParam, userID, pathWithParams)
		req, _ := http.NewRequest(http.MethodGet, adhocURL, nil)
		removeUserID(req)

		actualUserID := req.URL.Query().Get(UserIDParam)
		assert.Empty(t, actualUserID)

		watchParam := req.URL.Query().Get("watch")
		assert.Equal(t, watchParam, "true")

		assert.Equal(t, "/api/v1/namespaces/namespace-che/events?watch=true", req.RequestURI)
		assert.Equal(t, "http://f8osoproxy.com/api/v1/namespaces/namespace-che/events?watch=true", req.URL.String())
	})

	t.Run("UserID as part of url produced by rh-che via kubernetes client for 'exec' calls", func(t *testing.T) {
		pathWithParams := "/api/v1/namespaces/osio-ci-ee1-preview-che/pods/workspacetest.dockerimage/exec?command=mkdir\u0026command=-p\u0026command=%2Ftmp%2Fbootstrapper%2F\u0026command=%2Fworkspace_logs%2Fbootstrapper\u0026container=container\u0026stderr=true"
		adhocURL := fmt.Sprintf("http://f8osoproxy.com?%s=%s%s", UserIDParam, userID, pathWithParams)
		req, _ := http.NewRequest(http.MethodGet, adhocURL, nil)
		removeUserID(req)

		actualUserID := req.URL.Query().Get(UserIDParam)
		assert.Empty(t, actualUserID)

		assert.Equal(t, pathWithParams, req.RequestURI)
		assert.Equal(t, req.URL.RawQuery, "command=mkdir\u0026command=-p\u0026command=%2Ftmp%2Fbootstrapper%2F\u0026command=%2Fworkspace_logs%2Fbootstrapper\u0026container=container\u0026stderr=true")
		assert.Equal(t, "http://f8osoproxy.com/api/v1/namespaces/osio-ci-ee1-preview-che/pods/workspacetest.dockerimage/exec?command=mkdir\u0026command=-p\u0026command=%2Ftmp%2Fbootstrapper%2F\u0026command=%2Fworkspace_logs%2Fbootstrapper\u0026container=container\u0026stderr=true", req.URL.String())
	})

	t.Run("UserID as part of url produced by rh-che via kubernetes client for pod 'watch' calls", func(t *testing.T) {
		pathWithParams := "/api/v1/namespaces/osio-ci-ee1-preview-che/pods\u0026fieldSelector=metadata.name%3Drm-workspace41v9261pdzqs84c4\u0026watch=true"
		adhocURL := fmt.Sprintf("http://f8osoproxy.com/?%s=%s%s", UserIDParam, userID, pathWithParams)
		req, _ := http.NewRequest(http.MethodGet, adhocURL, nil)
		removeUserID(req)

		actualUserID := req.URL.Query().Get(UserIDParam)
		assert.Empty(t, actualUserID)

		watchParam := req.URL.Query().Get("watch")
		assert.Equal(t, watchParam, "true")

		fieldSelector := req.URL.Query().Get("fieldSelector")
		assert.Equal(t, "metadata.name=rm-workspace41v9261pdzqs84c4", fieldSelector)

		assert.Equal(t, "/api/v1/namespaces/osio-ci-ee1-preview-che/pods?fieldSelector=metadata.name%3Drm-workspace41v9261pdzqs84c4\u0026watch=true", req.RequestURI)
		assert.Equal(t, "fieldSelector=metadata.name%3Drm-workspace41v9261pdzqs84c4\u0026watch=true", req.URL.RawQuery)
	})

	t.Run("UserID as part of url produced by rh-che via kubernetes client for pod 'watch' calls", func(t *testing.T) {
		pathWithParams := "/api/v1/namespaces/osio-ci-ee1-preview-che/pods\u0026fieldSelector=metadata.name%3Dworkspacertz5iv86ez29v6bp.dockerimage\u0026watch=true"
		adhocURL := fmt.Sprintf("http://f8osoproxy.com?%s=%s%s", UserIDParam, userID, pathWithParams)
		req, _ := http.NewRequest(http.MethodGet, adhocURL, nil)
		removeUserID(req)

		actualUserID := req.URL.Query().Get(UserIDParam)
		assert.Empty(t, actualUserID)

		watchParam := req.URL.Query().Get("watch")
		assert.Equal(t, watchParam, "true")

		fieldSelector := req.URL.Query().Get("fieldSelector")
		assert.Equal(t, "metadata.name=workspacertz5iv86ez29v6bp.dockerimage", fieldSelector)

		assert.Equal(t, "/api/v1/namespaces/osio-ci-ee1-preview-che/pods?fieldSelector=metadata.name%3Dworkspacertz5iv86ez29v6bp.dockerimage\u0026watch=true", req.RequestURI)
		assert.Equal(t, "fieldSelector=metadata.name%3Dworkspacertz5iv86ez29v6bp.dockerimage\u0026watch=true", req.URL.RawQuery)
	})

	t.Run("UserID as part of url produced by rh-che via kubernetes client for pod removal calls", func(t *testing.T) {
		pathWithParams := "/api/v1/namespaces/osio-ci-ee1-preview-che/pods\u0026fieldSelector=metadata.name%3Drm-workspace41v9261pdzqs84c4\u0026watch=true"
		adhocURL := fmt.Sprintf("http://f8osoproxy.com?%s=%s%s", UserIDParam, userID, pathWithParams)
		req, _ := http.NewRequest(http.MethodGet, adhocURL, nil)
		removeUserID(req)

		actualUserID := req.URL.Query().Get(UserIDParam)
		assert.Empty(t, actualUserID)

		watchParam := req.URL.Query().Get("watch")
		assert.Equal(t, watchParam, "true")

		fieldSelector := req.URL.Query().Get("fieldSelector")
		assert.Equal(t, "metadata.name=rm-workspace41v9261pdzqs84c4", fieldSelector)

		assert.Equal(t, "/api/v1/namespaces/osio-ci-ee1-preview-che/pods?fieldSelector=metadata.name%3Drm-workspace41v9261pdzqs84c4\u0026watch=true", req.RequestURI)
		assert.Equal(t, "fieldSelector=metadata.name%3Drm-workspace41v9261pdzqs84c4\u0026watch=true", req.URL.RawQuery)
		assert.Equal(t, "http://f8osoproxy.com/api/v1/namespaces/osio-ci-ee1-preview-che/pods?fieldSelector=metadata.name%3Drm-workspace41v9261pdzqs84c4\u0026watch=true", req.URL.String())
	})

	t.Run("UserID as part of url produced by rh-che via kubernetes client for routes calls", func(t *testing.T) {
		pathWithParams := "/apis/route.openshift.io/v1/namespaces/osio-ci-ee1-preview-che/routes\u0026labelSelector=che.workspace_id%3Dworkspacertz5iv86ez29v6b"
		adhocURL := fmt.Sprintf("http://f8osoproxy.com?%s=%s%s", UserIDParam, userID, pathWithParams)
		req, _ := http.NewRequest(http.MethodGet, adhocURL, nil)
		removeUserID(req)

		actualUserID := req.URL.Query().Get(UserIDParam)
		assert.Empty(t, actualUserID)

		labelSelector := req.URL.Query().Get("labelSelector")
		assert.Equal(t, "che.workspace_id=workspacertz5iv86ez29v6b", labelSelector)

		assert.Equal(t, "/apis/route.openshift.io/v1/namespaces/osio-ci-ee1-preview-che/routes?labelSelector=che.workspace_id%3Dworkspacertz5iv86ez29v6b", req.RequestURI)
		assert.Equal(t, "labelSelector=che.workspace_id%3Dworkspacertz5iv86ez29v6b", req.URL.RawQuery)
		assert.Equal(t, "http://f8osoproxy.com/apis/route.openshift.io/v1/namespaces/osio-ci-ee1-preview-che/routes?labelSelector=che.workspace_id%3Dworkspacertz5iv86ez29v6b", req.URL.String())
	})

	t.Run("UserID as part of url produced by rh-che via kubernetes client for services calls", func(t *testing.T) {
		pathWithParams := "/api/v1/namespaces/osio-ci-ee1-preview-che/services\u0026labelSelector=che.workspace_id%3Dworkspacertz5iv86ez29v6bp"
		adhocURL := fmt.Sprintf("http://f8osoproxy.com/?%s=%s%s", UserIDParam, userID, pathWithParams)
		req, _ := http.NewRequest(http.MethodGet, adhocURL, nil)
		removeUserID(req)

		actualUserID := req.URL.Query().Get(UserIDParam)
		assert.Empty(t, actualUserID)

		labelSelector := req.URL.Query().Get("labelSelector")
		assert.Equal(t, "che.workspace_id=workspacertz5iv86ez29v6bp", labelSelector)

		assert.Equal(t, "/api/v1/namespaces/osio-ci-ee1-preview-che/services?labelSelector=che.workspace_id%3Dworkspacertz5iv86ez29v6bp", req.RequestURI)
		assert.Equal(t, "labelSelector=che.workspace_id%3Dworkspacertz5iv86ez29v6bp", req.URL.RawQuery)
		assert.Equal(t, "http://f8osoproxy.com/api/v1/namespaces/osio-ci-ee1-preview-che/services?labelSelector=che.workspace_id%3Dworkspacertz5iv86ez29v6bp", req.URL.String())
	})

	t.Run("UserID as part of url produced by rh-che via kubernetes client for pod 'watch' calls", func(t *testing.T) {
		pathWithParams := "/api/v1/namespaces/osio-ci-ee1-preview-che/pods\u0026fieldSelector=metadata.name%3Drm-workspace41v9261pdzqs84c4\u0026watch=true"
		adhocURL := fmt.Sprintf("http://f8osoproxy.com/?%s=%s%s", UserIDParam, userID, pathWithParams)
		req, _ := http.NewRequest(http.MethodGet, adhocURL, nil)
		removeUserID(req)

		actualUserID := req.URL.Query().Get(UserIDParam)
		assert.Empty(t, actualUserID)

		watchParam := req.URL.Query().Get("watch")
		assert.Equal(t, watchParam, "true")

		fieldSelector := req.URL.Query().Get("fieldSelector")
		assert.Equal(t, "metadata.name=rm-workspace41v9261pdzqs84c4", fieldSelector)

		assert.Equal(t, "/api/v1/namespaces/osio-ci-ee1-preview-che/pods?fieldSelector=metadata.name%3Drm-workspace41v9261pdzqs84c4\u0026watch=true", req.RequestURI)
		assert.Equal(t, "fieldSelector=metadata.name%3Drm-workspace41v9261pdzqs84c4\u0026watch=true", req.URL.RawQuery)
		assert.Equal(t, "http://f8osoproxy.com/api/v1/namespaces/osio-ci-ee1-preview-che/pods?fieldSelector=metadata.name%3Drm-workspace41v9261pdzqs84c4\u0026watch=true", req.URL.String())
	})

	t.Run("UserID as part of url produced by rh-che via kubernetes client for pod with lablel selector", func(t *testing.T) {
		pathWithParams := "/api/v1/namespaces/osio-ci-ee1-preview-che/pods\u0026labelSelector=che.workspace_id%3Dworkspacew9zk6m4xggf0pbtk"
		adhocURL := fmt.Sprintf("http://f8osoproxy.com/?%s=%s%s", UserIDParam, userID, pathWithParams)
		req, _ := http.NewRequest(http.MethodGet, adhocURL, nil)
		removeUserID(req)

		actualUserID := req.URL.Query().Get(UserIDParam)
		assert.Empty(t, actualUserID)

		labelSelector := req.URL.Query().Get("labelSelector")
		assert.Equal(t, "che.workspace_id=workspacew9zk6m4xggf0pbtk", labelSelector)

		assert.Equal(t, "/api/v1/namespaces/osio-ci-ee1-preview-che/pods?labelSelector=che.workspace_id%3Dworkspacew9zk6m4xggf0pbtk", req.RequestURI)
		assert.Equal(t, "labelSelector=che.workspace_id%3Dworkspacew9zk6m4xggf0pbtk", req.URL.RawQuery)
		assert.Equal(t, "http://f8osoproxy.com/api/v1/namespaces/osio-ci-ee1-preview-che/pods?labelSelector=che.workspace_id%3Dworkspacew9zk6m4xggf0pbtk", req.URL.String())
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
	}

	for _, table := range tables {
		req := createRequestWithPath(table.reqPath)
		reqType := getRequestType(req)
		reqType.stripPathPrefix(req)
		assertRequestPath(t, req, table.expectedPath)
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
