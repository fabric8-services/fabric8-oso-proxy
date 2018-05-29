package integration

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/containous/traefik/integration/common"
	"github.com/containous/traefik/integration/try"
	"github.com/containous/traefik/log"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/go-check/check"
	checker "github.com/vdemeester/shakers"
)

type OSIOProviderSuite struct{ BaseSuite }

func (s *OSIOProviderSuite) TestOSIOProvider(c *check.C) {
	// configure OSIO
	os.Setenv("TENANT_URL", common.TenantURL)
	os.Setenv("AUTH_URL", common.AuthURL)
	os.Setenv("SERVICE_ACCOUNT_ID", "any-id")
	os.Setenv("SERVICE_ACCOUNT_SECRET", "anysecret")
	os.Setenv("AUTH_TOKEN_KEY", "secret")
	tenantServer := common.StartServer(9090, common.ServeTenantRequest)
	defer tenantServer.Close()
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

		url := "http://127.0.0.1:8000/restall"
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", common.TestTokenManager.ToTokenString(jwt.MapClaims{"sub": "2222"})))
		res, _ := try.Response(req, 500*time.Millisecond)
		log.Printf("url=%s, res.StatusCode=%d", url, res.StatusCode)

		if res.StatusCode == http.StatusOK {
			if !gotOk {
				gotOk = true
				makeRequest(c, "http://127.0.0.1:8000/api", 200)
				makeRequest(c, "http://127.0.0.1:8000/api/anything", 200)
				makeRequest(c, "http://127.0.0.1:8000/metrics", 200)
				makeRequest(c, "http://127.0.0.1:8000/metrics/anything", 200)
			}
		} else if gotOk && res.StatusCode == http.StatusNotFound {
			gotNotFound = true
			makeRequest(c, "http://127.0.0.1:8000/api", 404)
			makeRequest(c, "http://127.0.0.1:8000/api/anything", 404)
			makeRequest(c, "http://127.0.0.1:8000/metrics", 404)
			makeRequest(c, "http://127.0.0.1:8000/metrics/anything", 404)
			break
		}
	}
	c.Assert(gotOk, check.Equals, true)
	c.Assert(gotNotFound, check.Equals, true)
}

func makeRequest(c *check.C, url string, expectedStatusCode int) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", common.TestTokenManager.ToTokenString(jwt.MapClaims{"sub": "2222"})))
	res, _ := try.Response(req, 500*time.Millisecond)
	log.Printf("url=%s, res.StatusCode=%d", url, res.StatusCode)
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
