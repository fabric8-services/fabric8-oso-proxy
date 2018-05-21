package common

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
)

const (
	TenantURL = "http://127.0.0.1:9090/api"
	AuthURL   = "http://127.0.0.1:9091/api"
)

func StartServer(port int, handler func(w http.ResponseWriter, r *http.Request)) (ts *httptest.Server) {
	if handler == nil {
		handler = func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "port=%d", port)
		}
	}
	if listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port)); err != nil {
		panic(err)
	} else {
		ts = &httptest.Server{
			Listener: listener,
			Config:   &http.Server{Handler: http.HandlerFunc(handler)},
		}
		ts.Start()
	}
	return
}

func ServeTenantRequest(rw http.ResponseWriter, req *http.Request) {
	authHeader := req.Header.Get("Authorization")

	metricsHost := ""
	apiHost := ""
	switch {
	case strings.HasSuffix(authHeader, "1111"):
		metricsHost = "http://127.0.0.1:7071"
		apiHost = "http://127.0.0.1:8081"
	case strings.HasSuffix(authHeader, "2222"):
		metricsHost = "http://127.0.0.1:7072"
		apiHost = "http://127.0.0.1:8082"
	case strings.HasSuffix(authHeader, "3333"):
		metricsHost = "http://127.0.0.1:7073"
		apiHost = "http://127.0.0.1:8083" // :8083 is not present in toml file
	case strings.HasSuffix(authHeader, "4444"):
		rw.WriteHeader(http.StatusNotFound)
		return
	}

	res := fmt.Sprintf(`{
		"data": {
			"attributes": {
				"namespaces": [
					{
						"name": "myuser-preview-stage",
						"cluster-metrics-url": "%s",
						"cluster-url": "%s"
					}
				]
			}
		}
	}`, metricsHost, apiHost)
	rw.Write([]byte(res))
}

func ServerAuthRequest(serverClusterAPI func() string) func(rw http.ResponseWriter, req *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		path := req.URL.Path
		if path == "/api/token" {
			tokenAPIResponse := `{"access_token": "1111","token_type": "bearer"}`
			rw.Write([]byte(tokenAPIResponse))
		} else if path == "/api/clusters" {
			clustersAPIResponse := serverClusterAPI()
			rw.Write([]byte(clustersAPIResponse))
		}
	}
}

func TwoClusterData() string {
	// "api-url": "https://api.starter-us-east-2.openshift.com/",
	// "api-url": "https://api.starter-us-east-2a.openshift.com/",
	res := `{
		"data": [
			{
				"api-url": "http://127.0.0.1:8081/",
				"app-dns": "8a09.starter-us-east-2.openshiftapps.com",
				"console-url": "https://console.starter-us-east-2.openshift.com/console/",
				"metrics-url": "http://127.0.0.1:7071/",
				"name": "us-east-2"
			},
			{
				"api-url": "http://127.0.0.1:8082/",
				"app-dns": "b542.starter-us-east-2a.openshiftapps.com",
				"console-url": "https://console.starter-us-east-2a.openshift.com/console/",
				"metrics-url": "http://127.0.0.1:7072/",
				"name": "us-east-2a"
			}
		]
	}`
	return res
}

func OneClusterData() string {
	// "api-url": "https://api.starter-us-east-2.openshift.com/",
	res := `{
		"data": [
			{
				"api-url": "http://localhost:8081/",
				"app-dns": "8a09.starter-us-east-2.openshiftapps.com",
				"console-url": "https://console.starter-us-east-2.openshift.com/console/",
				"metrics-url": "http://127.0.0.1:7071/",
				"name": "us-east-2"
			}
		]
	}`
	return res
}
