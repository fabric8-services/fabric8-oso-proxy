package integration

import (
	"net/http"
	"os"
	"time"

	"github.com/containous/traefik/integration/try"
	"github.com/containous/traefik/log"
	"github.com/go-check/check"
	checker "github.com/vdemeester/shakers"
)

// OSIOProviderSuite
type OSIOProviderSuite struct{ BaseSuite }

func (s *OSIOProviderSuite) TestOSIOProvider(c *check.C) {
	// configure OSIO
	os.Setenv("WIT_URL", witURL)
	os.Setenv("AUTH_URL", authURL)
	witServer := startOSIOServer(9090, serveWITRequest)
	defer witServer.Close()
	authServer := startOSIOServer(9091, serverClusterAPIRequest)
	defer authServer.Close()

	// Start Traefik
	cmd, display := s.traefikCmd(withConfigFile("fixtures/osio_provider_config.toml"))
	defer display(c)
	err := cmd.Start()
	c.Assert(err, checker.IsNil)
	defer cmd.Process.Kill()

	// Start OSIO servers
	ts1 := startOSIOServer(8081, nil)
	defer ts1.Close()
	ts2 := startOSIOServer(8082, nil)
	defer ts2.Close()

	// make multiple reqeust on some time gap
	// note, req has 'Bearer 2222' so it should go to 'http://127.0.0.1:8082' check serveWITRequest()
	// check serverClusterAPIRequest(), return 'http://127.0.0.1:8082' cluster in alternative reqeust
	// so few response would be 'HTTP 200 OK' and few would be 'HTTP 404 not found'
	for i := 0; i < 10; i++ {
		time.Sleep(3 * time.Second)

		req, _ := http.NewRequest("GET", "http://127.0.0.1:8000/test", nil)
		req.Header.Add("Authorization", "Bearer 2222")
		res, _ := try.Response(req, 500*time.Millisecond)
		log.Printf("req res.StatusCode=%d", res.StatusCode)
	}
}

var called = 0

func serverClusterAPIRequest(rw http.ResponseWriter, req *http.Request) {
	clustersAPIResponse := ""
	called++
	if called%2 == 0 {
		clustersAPIResponse = getOneCluster()
	} else {
		clustersAPIResponse = getTwoCluster()
	}
	rw.Write([]byte(clustersAPIResponse))
}

func getTwoCluster() string {
	// "api-url": "https://api.starter-us-east-2.openshift.com/",
	// "api-url": "https://api.starter-us-east-2a.openshift.com/",
	res := `{
		"data": [
			{
				"api-url": "http://127.0.0.1:8081/",
				"app-dns": "8a09.starter-us-east-2.openshiftapps.com",
				"console-url": "https://console.starter-us-east-2.openshift.com/console/",
				"metrics-url": "https://metrics.starter-us-east-2.openshift.com/",
				"name": "us-east-2"
			},
			{
				"api-url": "http://127.0.0.1:8082/",
				"app-dns": "b542.starter-us-east-2a.openshiftapps.com",
				"console-url": "https://console.starter-us-east-2a.openshift.com/console/",
				"metrics-url": "https://metrics.starter-us-east-2a.openshift.com/",
				"name": "us-east-2a"
			}
		]
	}`
	return res
}

func getOneCluster() string {
	// "api-url": "https://api.starter-us-east-2.openshift.com/",
	res := `{
		"data": [
			{
				"api-url": "http://localhost:8081/",
				"app-dns": "8a09.starter-us-east-2.openshiftapps.com",
				"console-url": "https://console.starter-us-east-2.openshift.com/console/",
				"metrics-url": "https://metrics.starter-us-east-2.openshift.com/",
				"name": "us-east-2"
			}
		]
	}`
	return res
}
