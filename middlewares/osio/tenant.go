package osio

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type tenantData struct {
}

type tenantLocator struct {
	client    *http.Client
	tenantURL string
}

func (t *tenantLocator) GetTenant(token string, params paramMap) (namespace, error) {
	url := fmt.Sprintf("%s/tenant", t.tenantURL)
	return locateTenant(t.client, url, token, params)
}

func (t *tenantLocator) GetTenantById(token string, userID string, params paramMap) (namespace, error) {
	url := fmt.Sprintf("%s/tenants/%s", t.tenantURL, userID)
	return locateTenant(t.client, url, token, params)
}

func CreateTenantLocator(client *http.Client, tenantBaseURL string) TenantLocator {
	return &tenantLocator{client: client, tenantURL: tenantBaseURL}
}

type response struct {
	Data data `json:"data"`
}

type data struct {
	Attributes attributes `json:"attributes"`
}

type attributes struct {
	Namespaces []namespace `json:"namespaces"`
}

type namespace struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	ClusterURL        string `json:"cluster-url"`
	ClusterMetricsURL string `json:"cluster-metrics-url,omitempty"`
	ClusterConsoleURL string `json:"cluster-console-url,omitempty"`
	ClusterLoggingURL string `json:"cluster-logging-url,omitempty"`
}

func getNamespace(resp response, nsType string) (ns namespace, err error) {
	if len(resp.Data.Attributes.Namespaces) == 0 {
		return ns, fmt.Errorf("no namespace found")
	}
	for _, namespace := range resp.Data.Attributes.Namespaces {
		if namespace.Type == nsType {
			return namespace, nil
		}
	}
	return ns, fmt.Errorf("no namespace matched")
}

func locateTenant(client *http.Client, url, token string, params paramMap) (ns namespace, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ns, err
	}
	req.Header.Set(Authorization, "Bearer "+token)
	setParams(req, params)
	resp, err := client.Do(req)
	if resp.StatusCode != http.StatusOK {
		return ns, httpErr{
			err:        fmt.Sprintf("Call to '%s' failed with status '%s'", url, resp.Status),
			statusCode: resp.StatusCode,
		}
	}
	defer resp.Body.Close()

	var r response
	err = json.NewDecoder(resp.Body).Decode(&r)
	if err != nil {
		return ns, err
	}
	return getNamespace(r, params[NamespaceType])
}

func setParams(req *http.Request, params paramMap) {
	if params == nil || len(params) == 0 {
		return
	}
	q := req.URL.Query()
	for key, value := range params {
		q.Set(key, value)
	}
	req.URL.RawQuery = q.Encode()
}
