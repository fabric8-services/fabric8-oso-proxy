package osio

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testCheCtx struct {
	tables          []testCheData
	authCallCount   int
	tenantCallCount int
	currInd         int
}

type testCheData struct {
	inputPath      string
	userID         string
	expectedTarget string
	expectedToken  string
}

/*
Test case details

call				user								userId			namesapce						cluster					details
------------------------------------------------------------------------------------------------------------------------------------
call-1,2		john-preview				11111111		john-preview-che		cluster1.com		std usecase, user & ns for user 11111111 on cluster1
call-3,4		osio-test-preview		22222222		k8s-image-puller		cluster1.com		daemonset usecase with user 22222222 on cluster1
call-5			osio-test2-preview	33333333		k8s-image-puller		cluster2.com		daemonset usecase with user 33333333 on cluster2
call-6			osio-test-preview		22222222		osio-test-preview		cluster1.com		std usecase, user & ns for user 22222222 on cluster1

- between call3 and call5, daemonset usecase with same namespace but different user on different cluster
- between call3 and call6, same user but call3 is damenset usecase while call6 is std usecase
*/
var cheCtx = testCheCtx{tables: []testCheData{
	{
		"/api/v1/namespaces/john-preview-che/pods",
		"11111111-4c6d-498c-97d0-cc7f2abcaca6",
		"127.0.0.1:9091",
		"1000_che_secret",
	},
	{
		"/api/v1/namespaces/john-preview-che/pods", // same test data to check cache
		"11111111-4c6d-498c-97d0-cc7f2abcaca6",
		"127.0.0.1:9091",
		"1000_che_secret",
	},
	{
		"/apis/apps/v1/namespaces/k8s-image-puller/daemonsets",
		"22222222-1874-4de5-9c62-602634cb5cc2",
		"127.0.0.1:9091",
		"2000_che_secret",
	},
	{
		"/apis/apps/v1/namespaces/k8s-image-puller/daemonsets", // same test data to check cache
		"22222222-1874-4de5-9c62-602634cb5cc2",
		"127.0.0.1:9091",
		"2000_che_secret",
	},
	{
		// same ns=k8s-image-puller but different user=33333333-*
		"/apis/apps/v1/namespaces/k8s-image-puller/daemonsets",
		"33333333-1874-4de5-9c62-602634cb5cc2",
		"127.0.0.1:9092",
		"3000_che_secret",
	},
	{
		// user=22222222-* wants to access its own ns=osio-test-preview-che resuorces
		"/apis/apps/v1/namespaces/osio-test-preview-che/pods",
		"22222222-1874-4de5-9c62-602634cb5cc2",
		"127.0.0.1:9091",
		"4000_che_secret",
	},
}}

func TestChe(t *testing.T) {
	os.Setenv("AUTH_TOKEN_KEY", "foo")

	authServer := cheCtx.createServer(cheCtx.serveAuthRequest)
	defer authServer.Close()
	tenantServer := cheCtx.createServer(cheCtx.serveTenantRequest)
	defer tenantServer.Close()

	authURL := "http://" + authServer.Listener.Addr().String()
	tenantURL := "http://" + tenantServer.Listener.Addr().String()
	srvAccID := "sa1"
	srvAccSecret := "secret"

	osio := NewOSIOAuth(tenantURL, authURL, srvAccID, srvAccSecret)
	osio.RequestTokenType = cheCtx.testTokenTypeLocator
	osioServer := cheCtx.createServer(cheCtx.serverOSIORequest(osio))
	defer osioServer.Close()
	osioURL := osioServer.Listener.Addr().String()

	for ind, table := range cheCtx.tables {
		cheCtx.currInd = ind
		cluster := cheCtx.startServer(table.expectedTarget, cheCtx.serveClusterRequest)

		currReqPath := table.inputPath
		cheSAToken := "1000_che_sa_token"

		req, _ := http.NewRequest("GET", "http://"+osioURL+currReqPath, nil)
		req.Header.Set(Authorization, "Bearer "+cheSAToken)
		req.Header.Set(UserIDHeader, table.userID)
		res, _ := http.DefaultClient.Do(req)
		assert.NotNil(t, res)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		errMsg := res.Header.Get("err")
		assert.Empty(t, errMsg, errMsg)

		cluster.Close()
	}
	expecteTenantCalls := 4
	assert.Equal(t, expecteTenantCalls, cheCtx.tenantCallCount, "Number of time Tenant server called was incorrect, want:%d, got:%d", expecteTenantCalls, cheCtx.tenantCallCount)
}

func (t testCheCtx) createServer(handle func(http.ResponseWriter, *http.Request)) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handle)
	return httptest.NewServer(mux)
}

func (t testCheCtx) serveAuthRequest(rw http.ResponseWriter, req *http.Request) {
	var res string
	if strings.HasSuffix(req.URL.Path, "/token") && req.Method == "POST" {
		res = `{
			"access_token": "1000_oso_proxy_sa_token",
			"token_type": "bearer"
		}`
	} else if strings.HasSuffix(req.URL.Path, "/token") && req.Method == "GET" {
		res = `{
			"access_token": "jA0ECQMCtCG1bfGEQbxg0kABEQ6nh/A4tMGGkHMHJtLDtFLyXh28IuLvoyGjsZtWPV0LHwN+EEsTtu90BQGbWFdBv+2Wiedk9eE3h08lwA8m",
			"scope": "<unknown>",
			"token_type": "bearer",
			"username": "dsaas"
		}`
	}
	rw.Write([]byte(res))
}

func (t testCheCtx) serveTenantRequest(rw http.ResponseWriter, req *http.Request) {
	cheCtx.tenantCallCount++
	var res string
	if strings.HasSuffix(req.URL.Path, "/tenants/11111111-4c6d-498c-97d0-cc7f2abcaca6") {
		res = `{
			"data": {
			  "attributes": {
				"created-at": "2018-03-21T11:28:22.042269Z",
				"namespaces": [
				  {
					"cluster-app-domain": "b542.starter-us-east-2a.openshiftapps.com",
					"cluster-console-url": "https://console.starter-us-east-2a.openshift.com/console/",
					"cluster-logging-url": "https://console.starter-us-east-2a.openshift.com/console/",
					"cluster-metrics-url": "https://metrics.starter-us-east-2a.openshift.com/",
					"cluster-url": "http://127.0.0.1:9091/",
					"created-at": "2018-03-21T11:28:22.299195Z",
					"name": "john-preview-stage",
					"state": "created",
					"type": "stage",
					"updated-at": "2018-03-21T11:28:22.299195Z",
					"version": "2.0.11"
				  },
				  {
					"cluster-app-domain": "b542.starter-us-east-2a.openshiftapps.com",
					"cluster-console-url": "https://console.starter-us-east-2a.openshift.com/console/",
					"cluster-logging-url": "https://console.starter-us-east-2a.openshift.com/console/",
					"cluster-metrics-url": "https://metrics.starter-us-east-2a.openshift.com/",
					"cluster-url": "http://127.0.0.1:9091/",
					"created-at": "2018-03-21T11:28:22.372172Z",
					"name": "john-preview-run",
					"state": "created",
					"type": "run",
					"updated-at": "2018-03-21T11:28:22.372172Z",
					"version": "2.0.11"
				  },
				  {
					"cluster-app-domain": "b542.starter-us-east-2a.openshiftapps.com",
					"cluster-console-url": "https://console.starter-us-east-2a.openshift.com/console/",
					"cluster-logging-url": "https://console.starter-us-east-2a.openshift.com/console/",
					"cluster-metrics-url": "https://metrics.starter-us-east-2a.openshift.com/",
					"cluster-url": "http://127.0.0.1:9091/",
					"created-at": "2018-03-21T11:28:22.401522Z",
					"name": "john-preview-jenkins",
					"state": "created",
					"type": "jenkins",
					"updated-at": "2018-03-21T11:28:22.401522Z",
					"version": "2.0.11"
				  },
				  {
					"cluster-app-domain": "b542.starter-us-east-2a.openshiftapps.com",
					"cluster-console-url": "https://console.starter-us-east-2a.openshift.com/console/",
					"cluster-logging-url": "https://console.starter-us-east-2a.openshift.com/console/",
					"cluster-metrics-url": "https://metrics.starter-us-east-2a.openshift.com/",
					"cluster-url": "http://127.0.0.1:9091/",
					"created-at": "2018-03-21T11:28:22.413148Z",
					"name": "john-preview-che",
					"state": "created",
					"type": "che",
					"updated-at": "2018-03-21T11:28:22.413148Z",
					"version": "2.0.11"
				  },
				  {
					"cluster-app-domain": "b542.starter-us-east-2a.openshiftapps.com",
					"cluster-console-url": "https://console.starter-us-east-2a.openshift.com/console/",
					"cluster-logging-url": "https://console.starter-us-east-2a.openshift.com/console/",
					"cluster-metrics-url": "https://metrics.starter-us-east-2a.openshift.com/",
					"cluster-url": "http://127.0.0.1:9091/",
					"created-at": "2018-03-21T11:28:22.421707Z",
					"name": "john-preview",
					"state": "created",
					"type": "user",
					"updated-at": "2018-03-21T11:28:22.421707Z",
					"version": "1.0.91"
				  }
				]
			  },
			  "id": "11111111-4c6d-498c-97d0-cc7f2abcaca6",
			  "type": "userservices"
			}
		  }`
	} else if strings.HasSuffix(req.URL.Path, "/tenants/22222222-1874-4de5-9c62-602634cb5cc2") {
		res = `{
			"data": {
				"attributes": {
					"created-at": "2018-03-21T11:28:22.042269Z",
					"namespaces": [
						{
							"cluster-app-domain": "b542.starter-us-east-2a.openshiftapps.com",
							"cluster-console-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-logging-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-metrics-url": "https://metrics.starter-us-east-2a.openshift.com/",
							"cluster-url": "http://127.0.0.1:9091/",
							"created-at": "2018-03-21T11:28:22.299195Z",
							"name": "osio-test-preview-stage",
							"state": "created",
							"type": "stage",
							"updated-at": "2018-03-21T11:28:22.299195Z",
							"version": "2.0.11"
						},
						{
							"cluster-app-domain": "b542.starter-us-east-2a.openshiftapps.com",
							"cluster-console-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-logging-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-metrics-url": "https://metrics.starter-us-east-2a.openshift.com/",
							"cluster-url": "http://127.0.0.1:9091/",
							"created-at": "2018-03-21T11:28:22.372172Z",
							"name": "osio-test-preview-run",
							"state": "created",
							"type": "run",
							"updated-at": "2018-03-21T11:28:22.372172Z",
							"version": "2.0.11"
						},
						{
							"cluster-app-domain": "b542.starter-us-east-2a.openshiftapps.com",
							"cluster-console-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-logging-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-metrics-url": "https://metrics.starter-us-east-2a.openshift.com/",
							"cluster-url": "http://127.0.0.1:9091/",
							"created-at": "2018-03-21T11:28:22.401522Z",
							"name": "osio-test-preview-jenkins",
							"state": "created",
							"type": "jenkins",
							"updated-at": "2018-03-21T11:28:22.401522Z",
							"version": "2.0.11"
						},
						{
							"cluster-app-domain": "b542.starter-us-east-2a.openshiftapps.com",
							"cluster-console-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-logging-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-metrics-url": "https://metrics.starter-us-east-2a.openshift.com/",
							"cluster-url": "http://127.0.0.1:9091/",
							"created-at": "2018-03-21T11:28:22.413148Z",
							"name": "osio-test-preview-che",
							"state": "created",
							"type": "che",
							"updated-at": "2018-03-21T11:28:22.413148Z",
							"version": "2.0.11"
						},
						{
							"cluster-app-domain": "b542.starter-us-east-2a.openshiftapps.com",
							"cluster-console-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-logging-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-metrics-url": "https://metrics.starter-us-east-2a.openshift.com/",
							"cluster-url": "http://127.0.0.1:9091/",
							"created-at": "2018-03-21T11:28:22.421707Z",
							"name": "osio-test-preview",
							"state": "created",
							"type": "user",
							"updated-at": "2018-03-21T11:28:22.421707Z",
							"version": "1.0.91"
						}
					]
				},
				"id": "22222222-1874-4de5-9c62-602634cb5cc2",
				"type": "userservices"
			}
		}`
	} else if strings.HasSuffix(req.URL.Path, "/tenants/33333333-1874-4de5-9c62-602634cb5cc2") {
		res = `{
			"data": {
				"attributes": {
					"created-at": "2018-03-21T11:28:22.042269Z",
					"namespaces": [
						{
							"cluster-app-domain": "b542.starter-us-east-2a.openshiftapps.com",
							"cluster-console-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-logging-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-metrics-url": "https://metrics.starter-us-east-2a.openshift.com/",
							"cluster-url": "http://127.0.0.1:9092/",
							"created-at": "2018-03-21T11:28:22.299195Z",
							"name": "osio-test2-preview-stage",
							"state": "created",
							"type": "stage",
							"updated-at": "2018-03-21T11:28:22.299195Z",
							"version": "2.0.11"
						},
						{
							"cluster-app-domain": "b542.starter-us-east-2a.openshiftapps.com",
							"cluster-console-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-logging-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-metrics-url": "https://metrics.starter-us-east-2a.openshift.com/",
							"cluster-url": "http://127.0.0.1:9092/",
							"created-at": "2018-03-21T11:28:22.372172Z",
							"name": "osio-test2-preview-run",
							"state": "created",
							"type": "run",
							"updated-at": "2018-03-21T11:28:22.372172Z",
							"version": "2.0.11"
						},
						{
							"cluster-app-domain": "b542.starter-us-east-2a.openshiftapps.com",
							"cluster-console-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-logging-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-metrics-url": "https://metrics.starter-us-east-2a.openshift.com/",
							"cluster-url": "http://127.0.0.1:9092/",
							"created-at": "2018-03-21T11:28:22.401522Z",
							"name": "osio-test2-preview-jenkins",
							"state": "created",
							"type": "jenkins",
							"updated-at": "2018-03-21T11:28:22.401522Z",
							"version": "2.0.11"
						},
						{
							"cluster-app-domain": "b542.starter-us-east-2a.openshiftapps.com",
							"cluster-console-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-logging-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-metrics-url": "https://metrics.starter-us-east-2a.openshift.com/",
							"cluster-url": "http://127.0.0.1:9092/",
							"created-at": "2018-03-21T11:28:22.413148Z",
							"name": "osio-test2-preview-che",
							"state": "created",
							"type": "che",
							"updated-at": "2018-03-21T11:28:22.413148Z",
							"version": "2.0.11"
						},
						{
							"cluster-app-domain": "b542.starter-us-east-2a.openshiftapps.com",
							"cluster-console-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-logging-url": "https://console.starter-us-east-2a.openshift.com/console/",
							"cluster-metrics-url": "https://metrics.starter-us-east-2a.openshift.com/",
							"cluster-url": "http://127.0.0.1:9092/",
							"created-at": "2018-03-21T11:28:22.421707Z",
							"name": "osio-test2-preview",
							"state": "created",
							"type": "user",
							"updated-at": "2018-03-21T11:28:22.421707Z",
							"version": "1.0.91"
						}
					]
				},
				"id": "33333333-1874-4de5-9c62-602634cb5cc2",
				"type": "userservices"
			}
		}`
	} else {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
	rw.Write([]byte(res))
}

func (t testCheCtx) serveClusterRequest(rw http.ResponseWriter, req *http.Request) {
	res := ""
	host := req.Host

	if strings.Contains(host, "127.0.0.1:9091") {

		if strings.HasSuffix(req.URL.Path, "api/v1/namespaces/john-preview-che/serviceaccounts/che") {
			res = `{
			"kind": "ServiceAccount",
			"apiVersion": "v1",
			"metadata": {
			  "name": "che",
			  "namespace": "john-preview-che",
			  "selfLink": "/api/v1/namespaces/john-preview-che/serviceaccounts/che",
			  "uid": "f9dfcc84-2cfa-11e8-a71f-024db754f2d2",
			  "resourceVersion": "117908057",
			  "creationTimestamp": "2018-03-21T11:28:28Z",
			  "labels": {
				"app": "fabric8-tenant-che-mt",
				"group": "io.fabric8.tenant.packages",
				"provider": "fabric8",
				"version": "2.0.82"
			  }
			},
			"secrets": [
			  {
				"name": "che-dockercfg-x8xx7"
			  },
			  {
				"name": "che-token-x6x6x"
			  }
			],
			"imagePullSecrets": [
			  {
				"name": "che-dockercfg-x8xx7"
			  }
			]
		  }
		  `
		} else if strings.HasSuffix(req.URL.Path, "api/v1/namespaces/k8s-image-puller/serviceaccounts/che") {
			res = `{
			"kind": "ServiceAccount",
			"apiVersion": "v1",
			"metadata": {
			  "name": "che",
			  "namespace": "k8s-image-puller",
			  "selfLink": "/api/v1/namespaces/k8s-image-puller/serviceaccounts/che",
			  "uid": "f9dfcc84-2cfa-11e8-a71f-024db754f2d2",
			  "resourceVersion": "117908057",
			  "creationTimestamp": "2018-03-21T11:28:28Z",
			  "labels": {
				"app": "fabric8-tenant-che-mt",
				"group": "io.fabric8.tenant.packages",
				"provider": "fabric8",
				"version": "2.0.82"
			  }
			},
			"secrets": [
			  {
				"name": "che-dockercfg-x8xx7"
			  },
			  {
				"name": "che-token-x2x2x"
			  }
			],
			"imagePullSecrets": [
			  {
				"name": "che-dockercfg-x8xx7"
			  }
			]
		  }
		  `
		} else if strings.HasSuffix(req.URL.Path, "api/v1/namespaces/osio-test-preview-che/serviceaccounts/che") {
			res = `{
			"kind": "ServiceAccount",
			"apiVersion": "v1",
			"metadata": {
			  "name": "che",
			  "namespace": "osio-test-preview-che",
			  "selfLink": "/api/v1/namespaces/osio-test-preview-che/serviceaccounts/che",
			  "uid": "f9dfcc84-2cfa-11e8-a71f-024db754f2d2",
			  "resourceVersion": "117908057",
			  "creationTimestamp": "2018-03-21T11:28:28Z",
			  "labels": {
				"app": "fabric8-tenant-che-mt",
				"group": "io.fabric8.tenant.packages",
				"provider": "fabric8",
				"version": "2.0.82"
			  }
			},
			"secrets": [
			  {
				"name": "che-dockercfg-x8xx7"
			  },
			  {
				"name": "che-token-x4x4x"
			  }
			],
			"imagePullSecrets": [
			  {
				"name": "che-dockercfg-x8xx7"
			  }
			]
		  }
		  `
		} else if strings.HasSuffix(req.URL.Path, "api/v1/namespaces/john-preview-che/secrets/che-token-x6x6x") {
			res = `{
			"kind": "Secret",
			"apiVersion": "v1",
			"metadata": {
			  "name": "che-token-x6x6x",
			  "namespace": "john-preview-che",
			  "selfLink": "/api/v1/namespaces/john-preview-che/secrets/che-token-x6x6x",
			  "uid": "f9e3f05e-a71f-024db754f2d2",
			  "resourceVersion": "117908051",
			  "creationTimestamp": "2018-03-21T11:28:28Z",
			  "annotations": {
				"kubernetes.io/service-account.name": "che",
				"kubernetes.io/service-account.uid": "f9dfcc84-xxx-024db754f2d2"
			  }
			},
			"data": {
			  "ca.crt": "xxxxx=",
			  "namespace": "xxxxx==",
			  "service-ca.crt": "xxxxx=",
			  "token": "MTAwMF9jaGVfc2VjcmV0"
			},
			"type": "kubernetes.io/service-account-token"
		  }`
		} else if strings.HasSuffix(req.URL.Path, "api/v1/namespaces/k8s-image-puller/secrets/che-token-x2x2x") {
			res = `{
			"kind": "Secret",
			"apiVersion": "v1",
			"metadata": {
			  "name": "che-token-x2x2x",
			  "namespace": "k8s-image-puller",
			  "selfLink": "/api/v1/namespaces/k8s-image-puller/secrets/che-token-x2x2x",
			  "uid": "f9e3f05e-a71f-024db754f2d2",
			  "resourceVersion": "117908051",
			  "creationTimestamp": "2018-03-21T11:28:28Z",
			  "annotations": {
				"kubernetes.io/service-account.name": "che",
				"kubernetes.io/service-account.uid": "f9dfcc84-xxx-024db754f2d2"
			  }
			},
			"data": {
			  "ca.crt": "xxxxx=",
			  "namespace": "xxxxx==",
			  "service-ca.crt": "xxxxx=",
			  "token": "MjAwMF9jaGVfc2VjcmV0"
			},
			"type": "kubernetes.io/service-account-token"
		  }`
		} else if strings.HasSuffix(req.URL.Path, "api/v1/namespaces/osio-test-preview-che/secrets/che-token-x4x4x") {
			res = `{
			"kind": "Secret",
			"apiVersion": "v1",
			"metadata": {
			  "name": "che-token-x4x4x",
			  "namespace": "osio-test-preview-che",
			  "selfLink": "/api/v1/namespaces/osio-test-preview-che/secrets/che-token-x4x4x",
			  "uid": "f9e3f05e-a71f-024db754f2d2",
			  "resourceVersion": "117908051",
			  "creationTimestamp": "2018-03-21T11:28:28Z",
			  "annotations": {
				"kubernetes.io/service-account.name": "che",
				"kubernetes.io/service-account.uid": "f9dfcc84-xxx-024db754f2d2"
			  }
			},
			"data": {
			  "ca.crt": "xxxxx=",
			  "namespace": "xxxxx==",
			  "service-ca.crt": "xxxxx=",
			  "token": "NDAwMF9jaGVfc2VjcmV0"
			},
			"type": "kubernetes.io/service-account-token"
		  }`
		}

	} else if strings.Contains(host, "127.0.0.1:9092") {

		if strings.HasSuffix(req.URL.Path, "api/v1/namespaces/k8s-image-puller/serviceaccounts/che") {
			res = `{
			"kind": "ServiceAccount",
			"apiVersion": "v1",
			"metadata": {
			  "name": "che",
			  "namespace": "k8s-image-puller",
			  "selfLink": "/api/v1/namespaces/k8s-image-puller/serviceaccounts/che",
			  "uid": "f9dfcc84-2cfa-11e8-a71f-024db754f2d2",
			  "resourceVersion": "117908057",
			  "creationTimestamp": "2018-03-21T11:28:28Z",
			  "labels": {
				"app": "fabric8-tenant-che-mt",
				"group": "io.fabric8.tenant.packages",
				"provider": "fabric8",
				"version": "2.0.82"
			  }
			},
			"secrets": [
			  {
				"name": "che-dockercfg-x8xx7"
			  },
			  {
				"name": "che-token-x3x3x"
			  }
			],
			"imagePullSecrets": [
			  {
				"name": "che-dockercfg-x8xx7"
			  }
			]
		  }
		  `
		} else if strings.HasSuffix(req.URL.Path, "api/v1/namespaces/k8s-image-puller/secrets/che-token-x3x3x") {
			res = `{
			"kind": "Secret",
			"apiVersion": "v1",
			"metadata": {
			  "name": "che-token-x3x3x",
			  "namespace": "k8s-image-puller",
			  "selfLink": "/api/v1/namespaces/k8s-image-puller/secrets/che-token-x3x3x",
			  "uid": "f9e3f05e-a71f-024db754f2d2",
			  "resourceVersion": "117908051",
			  "creationTimestamp": "2018-03-21T11:28:28Z",
			  "annotations": {
				"kubernetes.io/service-account.name": "che",
				"kubernetes.io/service-account.uid": "f9dfcc84-xxx-024db754f2d2"
			  }
			},
			"data": {
			  "ca.crt": "xxxxx=",
			  "namespace": "xxxxx==",
			  "service-ca.crt": "xxxxx=",
			  "token": "MzAwMF9jaGVfc2VjcmV0"
			},
			"type": "kubernetes.io/service-account-token"
		  }`
		}
	}

	if len(res) == 0 {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
	rw.Write([]byte(res))
}

func (t testCheCtx) serverOSIORequest(osio *OSIOAuth) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, req *http.Request) {
		osio.ServeHTTP(rw, req, cheCtx.varifyHandler)
	}
}

func (t testCheCtx) varifyHandler(rw http.ResponseWriter, req *http.Request) {
	expectedTarget := cheCtx.tables[cheCtx.currInd].expectedTarget
	actualTarget := req.Header.Get("Target")
	if !strings.HasSuffix(actualTarget, expectedTarget) {
		rw.Header().Set("err", fmt.Sprintf("Target was incorrect, want:%s, got:%s", expectedTarget, actualTarget))
		return
	}
	expectedToken := cheCtx.tables[cheCtx.currInd].expectedToken
	actualToken := req.Header.Get(Authorization)
	if !strings.HasSuffix(actualToken, expectedToken) {
		rw.Header().Set("err", fmt.Sprintf("Token was incorrect, want:%s, got:%s", expectedToken, actualToken))
		return
	}
	userID := req.Header.Get(UserIDHeader)
	if userID != "" {
		rw.Header().Set("err", fmt.Sprintf("%s header should not be set, want:%s, got:%s", UserIDHeader, "", userID))
		return
	}
}

func (t testCheCtx) startServer(url string, handler func(w http.ResponseWriter, r *http.Request)) (ts *httptest.Server) {
	if listener, err := net.Listen("tcp", strings.Replace(url, "/", "", -1)); err != nil {
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

func (t testCheCtx) testTokenTypeLocator(token string) (TokenType, error) {
	return CheToken, nil
}
