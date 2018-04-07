package integration

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/containous/traefik/integration/try"
	"github.com/containous/traefik/log"
	"github.com/go-check/check"
	checker "github.com/vdemeester/shakers"
)

const (
	witURL  = "http://127.0.0.1:9090"
	authURL = "http://127.0.0.1:9091"
)

// AccessLogSuite
type OSIOSuite struct{ BaseSuite }

func (s *OSIOSuite) TestOSIO(c *check.C) {
	// configure OSIO
	os.Setenv("WIT_URL", witURL)
	os.Setenv("AUTH_URL", authURL)
	witServer := startOSIOServer(9090, serveWITRequest)
	defer witServer.Close()
	authServer := startOSIOServer(9091, serverAUTHRequest)
	defer authServer.Close()

	// Start Traefik
	cmd, display := s.traefikCmd(withConfigFile("fixtures/osio_config.toml"))
	defer display(c)
	err := cmd.Start()
	c.Assert(err, checker.IsNil)
	defer cmd.Process.Kill()

	// Start OSIO servers
	ts1 := startOSIOServer(8081, nil)
	defer ts1.Close()
	ts2 := startOSIOServer(8082, nil)
	defer ts2.Close()

	// Make some requests
	req, _ := http.NewRequest("GET", "http://127.0.0.1:8000/test", nil)
	req.Header.Add("Authorization", "Bearer 1111")
	res, err := try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("req1 res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, 200)
	checkPort(c, res, 8081)

	req, _ = http.NewRequest("GET", "http://127.0.0.1:8000/test", nil)
	req.Header.Add("Authorization", "Bearer 2222")
	res, _ = try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("req2 res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, 200)
	checkPort(c, res, 8082)

	req, _ = http.NewRequest("GET", "http://127.0.0.1:8000/test", nil)
	req.Header.Add("Authorization", "Bearer 3333")
	res, _ = try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("req3 res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, 404)

	req, _ = http.NewRequest("GET", "http://127.0.0.1:8000/test", nil)
	req.Header.Add("Authorization", "Bearer 4444")
	res, _ = try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("req4 res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, 401)

	req, _ = http.NewRequest("GET", "http://127.0.0.1:8000/test", nil)
	// req.Header.Add("Authorization", "Bearer 1111")
	res, _ = try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("req5 res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, 401)

	req, _ = http.NewRequest("OPTIONS", "http://127.0.0.1:8000/test", nil)
	// req.Header.Add("Authorization", "Bearer 1111")
	res, _ = try.Response(req, 500*time.Millisecond)
	c.Assert(err, check.IsNil)
	log.Printf("req6 res.StatusCode=%d", res.StatusCode)
	c.Assert(res.StatusCode, check.Equals, 200)
	checkPort(c, res, 8081)
}

func checkPort(c *check.C, res *http.Response, expectedPort int) {
	c.Assert(res, check.NotNil)
	body := make([]byte, res.ContentLength)
	res.Body.Read(body)
	if !strings.HasSuffix(string(body), strconv.Itoa(expectedPort)) {
		c.Errorf("Served by wrong port, want:%d, got:%s", expectedPort, string(body))
	}
}

func startOSIOServer(port int, handler func(w http.ResponseWriter, r *http.Request)) (ts *httptest.Server) {
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

func serveWITRequest(rw http.ResponseWriter, req *http.Request) {
	authHeader := req.Header.Get("Authorization")

	host := ""
	switch {
	case strings.HasSuffix(authHeader, "1111"):
		host = "http://127.0.0.1:8081"
	case strings.HasSuffix(authHeader, "2222"):
		host = "http://127.0.0.1:8082"
	case strings.HasSuffix(authHeader, "3333"):
		host = "http://127.0.0.1:8083" // :8083 is not present in toml file
	case strings.HasSuffix(authHeader, "4444"):
		rw.WriteHeader(http.StatusNotFound)
		return
	}

	res := "{\"data\":{\"attributes\":{\"namespaces\":[{\"cluster-url\":\"" + host + "/\"}]}}}"
	rw.Write([]byte(res))
}

func serverAUTHRequest(rw http.ResponseWriter, req *http.Request) {
}
