package osio

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cenk/backoff"
	"github.com/containous/traefik/job"
	"github.com/containous/traefik/log"
	"github.com/containous/traefik/provider"
	"github.com/containous/traefik/safe"
	"github.com/containous/traefik/types"
)

const (
	providerName  = "osio"
	authorization = "Authorization"
)

// Provider holds configurations of the provider.
type Provider struct {
	provider.BaseProvider `mapstructure:",squash" export:"true"`

	RefreshSeconds       int    `description:"Polling interval (in seconds)" export:"true"`
	ServiceAccountID     string `description:"Service Account ID" export:"true"`
	ServiceAccountSecret string `description:"Service Account Secret" export:"true"`
	TokenAPI             string `description:"Auth token API" export:"true"`
	ClusterAPI           string `description:"Cluster data API" export:"true"`

	client            client
	tokenResp         *tokenResponse
	defaultBackendURL string
}

// Provide allows the osio provider to provide configurations to traefik
// using the given configuration channel.
func (p *Provider) Provide(configChan chan<- types.ConfigMessage, pool *safe.Pool, constraints types.Constraints) error {
	log.Debugf("Configuring %s provider", providerName)
	p.init(configChan)
	p.schedule(configChan, pool)
	return nil
}

func (p *Provider) schedule(configChan chan<- types.ConfigMessage, pool *safe.Pool) {
	handleCanceled := func(ctx context.Context, err error) error {
		if ctx.Err() == context.Canceled || err == context.Canceled {
			return nil
		}
		return err
	}

	pool.Go(func(stop chan bool) {
		ctx, cancel := context.WithCancel(context.Background())
		safe.Go(func() {
			select {
			case <-stop:
				cancel()
			}
		})

		operation := func() error {
			err := p.fetchToken()
			if err != nil {
				return handleCanceled(ctx, err)
			}

			config, err := p.loadConfig()
			if err != nil {
				return handleCanceled(ctx, err)
			}
			if config != nil {
				configChan <- types.ConfigMessage{
					ProviderName:  providerName,
					Configuration: config,
				}
			}

			reload := time.NewTicker(time.Second * time.Duration(p.RefreshSeconds))
			defer reload.Stop()
			for {
				select {
				case <-reload.C:
					config, err := p.loadConfig()
					if err != nil {
						return handleCanceled(ctx, err)
					}
					if config != nil {
						configChan <- types.ConfigMessage{
							ProviderName:  providerName,
							Configuration: config,
						}
					}
				case <-ctx.Done():
					return handleCanceled(ctx, ctx.Err())
				}
			}
		}

		notify := func(err error, time time.Duration) {
			log.Errorf("%s Provider connection error %+v, retrying in %s", providerName, err, time)
		}
		err := backoff.RetryNotify(safe.OperationWithRecover(operation), job.NewBackOff(backoff.NewExponentialBackOff()), notify)
		if err != nil {
			log.Errorf("Cannot connect to %s Provider api %+v", providerName, err)
		}
	})
}

func (p *Provider) init(configChan chan<- types.ConfigMessage) {
	if p.RefreshSeconds <= 0 {
		p.RefreshSeconds = 60
	}
	p.client = &authClient{Client: http.DefaultClient}
}

func (p *Provider) fetchToken() error {
	if p.tokenResp != nil {
		return nil
	}
	tokenReq := &tokenRequest{GrantType: "client_credentials", ClientID: p.ServiceAccountID, ClientSecret: p.ServiceAccountSecret}
	tokenResp, err := p.client.callTokenAPI(p.TokenAPI, tokenReq)
	if err != nil {
		return err
	}
	p.tokenResp = tokenResp
	return nil
}

func (p *Provider) loadConfig() (*types.Configuration, error) {
	clusterResponse, err := p.client.callClusterAPI(p.ClusterAPI, p.tokenResp)
	if err != nil {
		return nil, err
	}
	return p.loadRules(clusterResponse), nil
}

func (p *Provider) loadRules(clusterResp *clusterResponse) *types.Configuration {
	config := &types.Configuration{
		Frontends: make(map[string]*types.Frontend),
		Backends:  make(map[string]*types.Backend),
	}
	if len(clusterResp.Clusters) <= 0 {
		return config
	}

	defaultBackendExist := false
	for ind, cluster := range clusterResp.Clusters {
		if p.defaultBackendURL != "" && p.defaultBackendURL == cluster.APIURL {
			defaultBackendExist = true
		}
		configInd := fmt.Sprintf("%d", ind+1)
		config.Frontends["frontend"+configInd] = createFrontend(cluster.APIURL, "backend"+configInd)
		config.Backends["backend"+configInd] = createBackend(cluster.APIURL)
	}
	if !defaultBackendExist {
		p.defaultBackendURL = clusterResp.Clusters[0].APIURL
	}
	if p.defaultBackendURL != "" {
		config.Frontends["default"] = createFrontend("default", "default")
		config.Backends["default"] = createBackend(p.defaultBackendURL)
	}

	return config
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
