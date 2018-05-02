package integration

import (
	"net/http"
	"os"
	"time"

	"github.com/containous/traefik/integration/common"
	"github.com/containous/traefik/integration/try"
	"github.com/containous/traefik/log"
	"github.com/go-check/check"
	checker "github.com/vdemeester/shakers"
)

type OSIOProviderSuite struct{ BaseSuite }

/* 	TestOSIOProvider tests OSIO provider here are the steps.
- start wit, auth server.
- start traefik server, it will call auth server to get cluster url (for two cluster) which will be dynamically configured by OSIO provider.
- start api, metrics server.
- wait for traefik to start serving requests.
- make reqeusts to traefik which will be 200 ok (both api and metrics)
- wait for traefik to call auth server and get cluster url (this time ONLY one cluster)
- make requests to traefik which will be 404 not found (both api and metrics)
*/
func (s *OSIOProviderSuite) TestOSIOProvider(c *check.C) {
	// configure OSIO
	os.Setenv("WIT_URL", common.WitURL)
	os.Setenv("AUTH_URL", common.AuthURL)
	os.Setenv("SERVICE_ACCOUNT_ID", "any-id")
	os.Setenv("SERVICE_ACCOUNT_SECRET", "anysecret")
	witServer := common.StartServer(9090, common.ServeWITRequest)
	defer witServer.Close()
	authServer := common.StartServer(9091, common.ServerAuthRequest(serveProviderCluster))
	defer authServer.Close()

	// Start Traefik
	cmd, display := s.traefikCmd(withConfigFile("fixtures/osio_provider_config.toml"))
	defer display(c)
	err := cmd.Start()
	c.Assert(err, checker.IsNil)
	defer cmd.Process.Kill()

	// Start OSIO servers
	ts1 := common.StartServer(8081, nil)
	defer ts1.Close()
	ts2 := common.StartServer(8082, nil)
	defer ts2.Close()
	ts3 := common.StartServer(7071, nil)
	defer ts3.Close()
	ts4 := common.StartServer(7072, nil)
	defer ts4.Close()

	// make multiple reqeust on some time gap
	// note, req has 'Bearer 2222' so it should go to 'http://127.0.0.1:8082' check ServeWITRequest()
	// check serveProviderCluster(), return 'http://127.0.0.1:8082' cluster for only first time
	// so first few response would be 'HTTP 200 OK' and then rest would be 'HTTP 404 not found'
	gotOk := false
	gotNotFound := false
	for i := 0; i < 8; i++ {
		time.Sleep(1 * time.Second) // need to give time to traefik to load configuration

		req, _ := http.NewRequest("GET", "http://127.0.0.1:8000/restall", nil)
		req.Header.Add("Authorization", "Bearer 2222")
		res, _ := try.Response(req, 500*time.Millisecond)
		log.Printf("req res.StatusCode=%d", res.StatusCode)

		if res.StatusCode == http.StatusOK {
			if !gotOk {
				makeRequest(c, "http://127.0.0.1:8000/api", 200)
				makeRequest(c, "http://127.0.0.1:8000/api/anything", 200)
				makeRequest(c, "http://127.0.0.1:8000/metrics", 200)
				makeRequest(c, "http://127.0.0.1:8000/metrics/anything", 200)
			}
			gotOk = true
		} else if gotOk && res.StatusCode == http.StatusNotFound {
			makeRequest(c, "http://127.0.0.1:8000/api", 404)
			makeRequest(c, "http://127.0.0.1:8000/api/anything", 404)
			makeRequest(c, "http://127.0.0.1:8000/metrics", 404)
			makeRequest(c, "http://127.0.0.1:8000/metrics/anything", 404)

			gotNotFound = true
			break
		}
	}
	c.Assert(gotOk, check.Equals, true)
	c.Assert(gotNotFound, check.Equals, true)
}

func makeRequest(c *check.C, url string, expectedStatusCode int) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "Bearer 2222")
	res, _ := try.Response(req, 500*time.Millisecond)
	log.Printf("req res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, expectedStatusCode)
}

var oneClusterExists = false

func serveProviderCluster() string {
	clustersAPIResponse := ""
	if oneClusterExists == false {
		clustersAPIResponse = common.TwoClusterData()
		oneClusterExists = true
	} else {
		clustersAPIResponse = common.OneClusterData()
	}
	return clustersAPIResponse
}
