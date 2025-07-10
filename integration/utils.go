package integration

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"testing"

	"github.com/go-kit/log/level"
	"github.com/openstack-exporter/openstack-exporter/exporters"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"

	"github.com/gophercloud/gophercloud/v2/openstack/baremetal/v1/nodes"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/openstack-exporter/openstack-exporter/integration/tools"
)

var DEFAULT_OS_CLIENT_CONFIG = "/etc/openstack/clouds.yaml"

// startOpenStackExporter starts an instance of the OpenStack exporter for
// testing purposes. It returns a cleanup function that should be called
// after the test is complete to shut down the exporter.
func startOpenStackExporter() (string, func(), error) {
	// Define flags (copied from main.go)
	metricsPath := "/metrics"
	listenAddress := ":9180"
	prefix := "openstack"
	endpointType := "public"
	collectTime := false
	disabledMetrics := []string{}
	disableSlowMetrics := false
	disableDeprecatedMetrics := false
	disableCinderAgentUUID := false
	cloud := "devstack" // Or any suitable default for testing
	domainID := ""
	tenantID := ""

	// Create a logger for the test
	promlogConfig := &promlog.Config{}
	logger := promlog.New(promlogConfig)

	// Create a context to control the exporter lifecycle
	_, cancel := context.WithCancel(context.Background())

	// Create a registry and handler
	registry := prometheus.NewPedanticRegistry()

	// Define services to enable. For simplicity, we'll enable a minimal
	// set. Adjust as needed for your tests.
	var enabledServices = []string{"network", "compute", "image", "volume", "identity", "object-store", "load-balancer", "container-infra", "dns", "baremetal", "gnocchi", "database", "orchestration", "placement", "sharev2"}

	// Enable exporters
	enabledExporters := 0
	for _, service := range enabledServices {
		exp, err := exporters.EnableExporter(
			service,
			prefix,
			cloud,
			disabledMetrics,
			endpointType,
			collectTime,
			disableSlowMetrics,
			disableDeprecatedMetrics,
			disableCinderAgentUUID,
			domainID,
			tenantID,
			nil,
			logger,
		)
		if err != nil {
			level.Error(logger).Log(
				"err",
				"enabling exporter for service failed",
				"service",
				service,
				"error",
				err,
			)
			continue
		}
		registry.MustRegister(*exp)
		level.Info(logger).Log("msg", "Enabled exporter for service", "service", service)
		enabledExporters++
	}

	if enabledExporters == 0 {
		cancel()
		return "", nil, fmt.Errorf("no exporter has been enabled")
	}

	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})

	// Start the HTTP server in a goroutine
	server := &http.Server{Addr: listenAddress}
	http.Handle(metricsPath, handler)

	go func() {
		level.Info(logger).Log("msg", "Starting OpenStack exporter", "address", listenAddress)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			level.Error(logger).Log("err", "HTTP server failed", "error", err)
		}
	}()

	// Define the cleanup function
	cleanup := func() {
		level.Info(logger).Log("msg", "Shutting down OpenStack exporter")
		cancel() // Cancel the context
		ctxShutdown, cancelShutdown := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancelShutdown()
		if err := server.Shutdown(ctxShutdown); err != nil {
			level.Error(logger).Log("err", "HTTP server shutdown failed", "error", err)
		}
	}

	// Wait for the server to start. A simple check is to see if we can GET
	// the metrics endpoint.
	const maxTries = 10
	for i := 0; i < maxTries; i++ {
		resp, err := http.Get("http://localhost" + listenAddress + metricsPath)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return listenAddress, cleanup, nil // Success!
		}
		time.Sleep(1 * time.Second)
	}

	// If we get here, the server didn't start in time. Clean up and return an
	// error.
	cleanup()
	return "", nil, fmt.Errorf("failed to start OpenStack exporter in time")
}

// CreateNode creates a basic node with a randomly generated name.
func CreateNode(t *testing.T, client *gophercloud.ServiceClient) (*nodes.Node, error) {
	name := tools.RandomString("ACPTTEST", 16)
	t.Logf("Attempting to create bare metal node: %s", name)

	node, err := nodes.Create(context.TODO(), client, nodes.CreateOpts{
		Name:          name,
		Driver:        "ipmi",
		BootInterface: "ipxe",
		RAIDInterface: "agent",
		DriverInfo: map[string]any{
			"ipmi_port":      "6230",
			"ipmi_username":  "admin",
			"deploy_kernel":  "http://172.22.0.1/images/tinyipa-stable-rocky.vmlinuz",
			"ipmi_address":   "192.168.122.1",
			"deploy_ramdisk": "http://172.22.0.1/images/tinyipa-stable-rocky.gz",
			"ipmi_password":  "admin",
		},
	}).Extract()

	return node, err
}

// DeleteNode deletes a bare metal node via its UUID.
func DeleteNode(t *testing.T, client *gophercloud.ServiceClient, node *nodes.Node) {
	// Force deletion of provisioned nodes requires maintenance mode.
	err := nodes.SetMaintenance(context.TODO(), client, node.UUID, nodes.MaintenanceOpts{
		Reason: "forced deletion",
	}).ExtractErr()
	if err != nil {
		t.Fatalf("Unable to move node %s into maintenance mode: %s", node.UUID, err)
	}

	err = nodes.Delete(context.TODO(), client, node.UUID).ExtractErr()
	if err != nil {
		t.Fatalf("Unable to delete node %s: %s", node.UUID, err)
	}

	t.Logf("Deleted server: %s", node.UUID)
}

func CreateFakeNode(t *testing.T, client *gophercloud.ServiceClient) (*nodes.Node, error) {
	name := tools.RandomString("ACPTTEST", 16)
	t.Logf("Attempting to create bare metal node: %s", name)

	node, err := nodes.Create(context.TODO(), client, nodes.CreateOpts{
		Name:            name,
		Driver:          "fake-hardware",
		BootInterface:   "fake",
		DeployInterface: "fake",
		DriverInfo: map[string]any{
			"ipmi_port":      "6230",
			"ipmi_username":  "admin",
			"deploy_kernel":  "http://172.22.0.1/images/tinyipa-stable-rocky.vmlinuz",
			"ipmi_address":   "192.168.122.1",
			"deploy_ramdisk": "http://172.22.0.1/images/tinyipa-stable-rocky.gz",
			"ipmi_password":  "admin",
		},
	}).Extract()

	return node, err
}

// DeployFakeNode deploys a node that uses fake-hardware.
func DeployFakeNode(t *testing.T, client *gophercloud.ServiceClient, node *nodes.Node) (*nodes.Node, error) {
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	currentState := node.ProvisionState

	if currentState == string(nodes.Enroll) {
		t.Logf("moving fake node %s to manageable", node.UUID)
		err := nodes.ChangeProvisionState(ctx, client, node.UUID, nodes.ProvisionStateOpts{
			Target: nodes.TargetManage,
		}).ExtractErr()
		if err != nil {
			return node, err
		}

		err = nodes.WaitForProvisionState(ctx, client, node.UUID, nodes.Manageable)
		if err != nil {
			return node, err
		}

		currentState = string(nodes.Manageable)
	}

	if currentState == string(nodes.Manageable) {
		t.Logf("moving fake node %s to available", node.UUID)
		err := nodes.ChangeProvisionState(ctx, client, node.UUID, nodes.ProvisionStateOpts{
			Target: nodes.TargetProvide,
		}).ExtractErr()
		if err != nil {
			return node, err
		}

		err = nodes.WaitForProvisionState(ctx, client, node.UUID, nodes.Available)
		if err != nil {
			return node, err
		}

		currentState = string(nodes.Available)
	}

	t.Logf("deploying fake node %s", node.UUID)
	return ChangeProvisionStateAndWait(ctx, client, node, nodes.ProvisionStateOpts{
		Target: nodes.TargetActive,
	}, nodes.Active)
}

func ChangeProvisionStateAndWait(ctx context.Context, client *gophercloud.ServiceClient, node *nodes.Node,
	change nodes.ProvisionStateOpts, expectedState nodes.ProvisionState) (*nodes.Node, error) {
	err := nodes.ChangeProvisionState(ctx, client, node.UUID, change).ExtractErr()
	if err != nil {
		return node, err
	}

	err = nodes.WaitForProvisionState(ctx, client, node.UUID, expectedState)
	if err != nil {
		return node, err
	}

	return nodes.Get(ctx, client, node.UUID).Extract()
}
