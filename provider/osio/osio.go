package osio

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/containous/traefik/log"
	"github.com/containous/traefik/provider"
	"github.com/containous/traefik/safe"
	"github.com/containous/traefik/types"
)

// Provider holds configurations of the provider.
type Provider struct {
	provider.BaseProvider `mapstructure:",squash" export:"true"`
	ClustersAPI           string `description:"URL to get Clusters data" export:"true"`
}

type cluster struct {
	APIURL     string `json:"api-url"`
	AppDNS     string `json:"app-dns"`
	ConsoleURL string `json:"console-url"`
	MetricsURL string `json:"metrics-url"`
	Name       string `json:"name"`
}

type clustersAPIResponse struct {
	Clusters []cluster `json:"data"`
}

// Provide allows the osio provider to provide configurations to traefik
// using the given configuration channel.
func (p *Provider) Provide(configChan chan<- types.ConfigMessage, pool *safe.Pool, constraints types.Constraints) error {
	config, err := p.loadConfig()
	if err != nil {
		return err
	}
	go p.runPeriodically(configChan)
	sendConfigToChannel(configChan, config)
	return nil
}

func (p *Provider) loadConfig() (*types.Configuration, error) {
	clustersAPIResponse := callClustersAPI(p.ClustersAPI)
	return loadRules(clustersAPIResponse)
}

func (p *Provider) runPeriodically(configChan chan<- types.ConfigMessage) {
	for {
		time.Sleep(5 * time.Second)
		p.checkConfig(configChan)
	}
}

func (p *Provider) checkConfig(configChan chan<- types.ConfigMessage) {
	config, err := p.loadConfig()
	if err != nil {
		log.Errorf("Error occurred while loading config: %s", err)
		return
	}
	sendConfigToChannel(configChan, config)
}

func callClustersAPI(apiURL string) *clustersAPIResponse {
	resp, err := http.Get(apiURL)
	if err != nil {
		log.Errorf("Error occurred while calling cluster API: %s", err)
		return nil
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Error occurred while reading cluster API response: %s", err)
		return nil
	}
	var clusters = new(clustersAPIResponse)
	err = json.Unmarshal(body, &clusters)
	if err != nil {
		log.Errorf("Error occurred while parsing cluster API response: %s", err)
		return nil
	}
	return clusters
}

func loadRules(clustersAPIResponse *clustersAPIResponse) (*types.Configuration, error) {
	config := &types.Configuration{
		Frontends: make(map[string]*types.Frontend),
		Backends:  make(map[string]*types.Backend),
	}
	for ind, cluster := range clustersAPIResponse.Clusters {
		noStr := fmt.Sprintf("%d", ind+1)
		config.Frontends["frontend"+noStr] = createFrontend(cluster.APIURL, "backend"+noStr)
		config.Backends["backend"+noStr] = createBackend(cluster.APIURL)
	}
	return config, nil
}

func createFrontend(clusterURL string, backend string) *types.Frontend {
	routes := make(map[string]types.Route)
	routes["test_1"] = types.Route{Rule: "HeadersRegexp:Target," + clusterURL}
	return &types.Frontend{Backend: backend, Routes: routes}
}

func createBackend(clusterURL string) *types.Backend {
	servers := make(map[string]types.Server)
	servers["server1"] = types.Server{URL: clusterURL}
	return &types.Backend{Servers: servers}
}

func sendConfigToChannel(configChan chan<- types.ConfigMessage, config *types.Configuration) {
	log.Printf("osio provider sending %d config", len(config.Frontends))
	configChan <- types.ConfigMessage{
		ProviderName:  "osio",
		Configuration: config,
	}
}
