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

func (s *OSIOProviderSuite) TestOSIOProvider(c *check.C) {
	// configure OSIO
	os.Setenv("WIT_URL", common.WitURL)
	os.Setenv("AUTH_URL", common.AuthURL)
	os.Setenv("SERVICE_ACCOUNT_ID", "any-id")
	os.Setenv("SERVICE_ACCOUNT_SECRET", "anysecret")
	witServer := common.StartOSIOServer(9090, common.ServeWITRequest)
	defer witServer.Close()
	authServer := common.StartOSIOServer(9091, common.ServerAuthRequest(serveProviderCluster))
	defer authServer.Close()

	// Start Traefik
	cmd, display := s.traefikCmd(withConfigFile("fixtures/osio_provider_config.toml"))
	defer display(c)
	err := cmd.Start()
	c.Assert(err, checker.IsNil)
	defer cmd.Process.Kill()

	// Start OSIO servers
	ts1 := common.StartOSIOServer(8081, nil)
	defer ts1.Close()
	ts2 := common.StartOSIOServer(8082, nil)
	defer ts2.Close()

	// make multiple reqeust on some time gap
	// note, req has 'Bearer 2222' so it should go to 'http://127.0.0.1:8082' check serverAUTHRequest2()
	// check serverAUTHRequest2(), return 'http://127.0.0.1:8082' cluster for only first time
	// so first few response would be 'HTTP 200 OK' and then rest would be 'HTTP 404 not found'
	gotOk := false
	gotNotFound := false
	for i := 0; i < 8; i++ {
		time.Sleep(1 * time.Second)
		req, _ := http.NewRequest("GET", "http://127.0.0.1:8000/test", nil)
		req.Header.Add("Authorization", "Bearer 2222")
		res, _ := try.Response(req, 500*time.Millisecond)
		log.Printf("req res.StatusCode=%d", res.StatusCode)
		if res.StatusCode == http.StatusOK {
			gotOk = true
		} else if gotOk && res.StatusCode == http.StatusNotFound {
			gotNotFound = true
			break
		}
	}
	c.Assert(gotOk, check.Equals, true)
	c.Assert(gotNotFound, check.Equals, true)
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
