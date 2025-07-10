package integration

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	th "github.com/gophercloud/gophercloud/v2/testhelper"
	"github.com/openstack-exporter/openstack-exporter/integration/clients"
)

func TestIntegration(t *testing.T) {
	clients.RequireLong(t)

	client, err := clients.NewBareMetalV1Client()
	th.AssertNoErr(t, err)
	client.Microversion = "1.87"

	node, err := CreateFakeNode(t, client)
	th.AssertNoErr(t, err)

	node, err = DeployFakeNode(t, client, node)
	th.AssertNoErr(t, err)

	// Start the OpenStack exporter
	_, cleanup, err := startOpenStackExporter()
	if err != nil {
		t.Fatalf("Failed to start OpenStack exporter: %v", err)
	}
	defer cleanup()

	// Construct the metrics URL
	metricsURL := "http://localhost:9180/metrics"

	// Helper function to fetch metrics with retries
	fetchMetrics := func(
		url string,
		maxTries int,
	) (resp *http.Response, body []byte, err error) {
		for i := 0; i < maxTries; i++ {
			resp, err = http.Get(url)
			if err == nil && resp.StatusCode == http.StatusOK {
				defer resp.Body.Close()
				body, err = io.ReadAll(resp.Body)
				if err == nil {
					return resp, body, nil // Success!
				}
				t.Logf("Attempt %d: Failed to read response body: %v", i+1, err)
			} else {
				var statusCode int
				if resp != nil {
					statusCode = resp.StatusCode
				}
				t.Logf(
					"Attempt %d: Failed to get metrics, status code: %d, error: %v",
					i+1,
					statusCode,
					err,
				)
			}
			if resp != nil && resp.Body != nil {
				resp.Body.Close() // Close the body on each retry
			}
			time.Sleep(1 * time.Second)
		}
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get metrics after %d retries: %w", maxTries, err)
		}

		return nil, nil, fmt.Errorf(
			"failed to get metrics after %d retries, "+
				"but the error is nil (this should not happen)",
			maxTries,
		)
	}

	time.Sleep(10 * time.Second)

	// Fetch the metrics
	const maxTriesFetch = 10
	resp, body, err := fetchMetrics(metricsURL, maxTriesFetch)
	if err != nil {
		t.Fatalf("Failed to fetch metrics after multiple retries: %v", err)
	}

	// Convert the response body to a string for easier handling
	bodyString := string(body)

	// Check for the expected metric and provide a clearer error message
	expectedMetric := "openstack_ironic_node"
	if !strings.Contains(bodyString, expectedMetric) {
		t.Errorf(
			"Metric '%s' not found in metrics response.\n\n"+
				"Status Code: %d\n\n"+
				"Metrics Endpoint: %s\n\n"+
				"Response Body:\n%s\n",
			expectedMetric,
			resp.StatusCode,
			metricsURL,
			bodyString,
		)
	}
}
