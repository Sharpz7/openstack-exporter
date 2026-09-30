package exporters

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	gophercloudv2 "github.com/gophercloud/gophercloud/v2"
	"github.com/jarcoal/httpmock"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type IronicTestSuite struct {
	BaseOpenStackTestSuite
}

var ironicExpectedUp = `
# HELP openstack_ironic_node node
# TYPE openstack_ironic_node gauge
openstack_ironic_node{conductor_group="",console_enabled="false",deploy_kernel="7ff5ef56-daaa-4256-9dd8-c3f1f9964ebc",deploy_ramdisk="e9c96d45-a4c8-4165-8753-9d8f32779e99",id="f50dcc35-4913-4667-a9fa-d130659c5661",instance_uuid="",last_error="",lessee="",maintenance="false",maintenance_reason="",name="r1-02",power_state="power off",provision_state="available",resource_class="baremetal",retired="true",retired_reason="No longer needed",serial_number="",traits=""} 1
openstack_ironic_node{conductor_group="",console_enabled="true",deploy_kernel="7ff5ef56-daaa-4256-9dd8-c3f1f9964ebc",deploy_ramdisk="e9c96d45-a4c8-4165-8753-9d8f32779e99",id="0129d2fc-0e5c-4b5b-a73b-01844d913957",instance_uuid="c0034f14-7937-41d5-b0f1-28d0d4e96426",last_error="",lessee="",maintenance="false",maintenance_reason="",name="r1-04",power_state="power on",provision_state="active",resource_class="baremetal",retired="true",retired_reason="No longer needed",serial_number="",traits=""} 1
openstack_ironic_node{conductor_group="rack-a",console_enabled="true",deploy_kernel="7ff5ef56-daaa-4256-9dd8-c3f1f9964ebc",deploy_ramdisk="e9c96d45-a4c8-4165-8753-9d8f32779e99",id="c9f98cc9-25e9-424e-8a89-002989054ec2",instance_uuid="",last_error="",lessee="",maintenance="true",maintenance_reason="Firmware upgrade",name="r1-05",power_state="power off",provision_state="available",resource_class="baremetal",retired="true",retired_reason="No longer needed",serial_number="",traits="CUSTOM_GPU HW_CPU_X86_VMX"} 1
openstack_ironic_node{conductor_group="",console_enabled="true",deploy_kernel="7ff5ef56-daaa-4256-9dd8-c3f1f9964ebc",deploy_ramdisk="e9c96d45-a4c8-4165-8753-9d8f32779e99",id="d381bea3-8768-4f12-a9b3-abf750ba918f",instance_uuid="31b5f585-104b-497a-bb72-b5376aaa089f",last_error="Provisioning failed Reached timeout",lessee="project-1234",maintenance="false",maintenance_reason="",name="r1-03",power_state="power on",provision_state="active",resource_class="baremetal",retired="true",retired_reason="No longer needed",serial_number="SN-1234567890",traits=""} 1
openstack_ironic_node{conductor_group="",console_enabled="true",deploy_kernel="7ff5ef56-daaa-4256-9dd8-c3f1f9964ebc",deploy_ramdisk="e9c96d45-a4c8-4165-8753-9d8f32779e99",id="d5641882-f7e5-4b92-9423-7e8157586218",instance_uuid="",last_error="",lessee="",maintenance="true",maintenance_reason="",name="r1-01",power_state="power off",provision_state="error",resource_class="baremetal",retired="true",retired_reason="No longer needed",serial_number="",traits=""} 1
# HELP openstack_ironic_node_provision_updated_at node_provision_updated_at
# TYPE openstack_ironic_node_provision_updated_at gauge
openstack_ironic_node_provision_updated_at{id="0129d2fc-0e5c-4b5b-a73b-01844d913957",name="r1-04",provision_state="active"} 1.593544011e+09
openstack_ironic_node_provision_updated_at{id="c9f98cc9-25e9-424e-8a89-002989054ec2",name="r1-05",provision_state="available"} 1.562908443e+09
openstack_ironic_node_provision_updated_at{id="d381bea3-8768-4f12-a9b3-abf750ba918f",name="r1-03",provision_state="active"} 1.593747281e+09
openstack_ironic_node_provision_updated_at{id="d5641882-f7e5-4b92-9423-7e8157586218",name="r1-01",provision_state="error"} 1.594708597e+09
openstack_ironic_node_provision_updated_at{id="f50dcc35-4913-4667-a9fa-d130659c5661",name="r1-02",provision_state="available"} 1.594740492e+09
# HELP openstack_ironic_node_updated_at node_updated_at
# TYPE openstack_ironic_node_updated_at gauge
openstack_ironic_node_updated_at{id="0129d2fc-0e5c-4b5b-a73b-01844d913957",name="r1-04",provision_state="active"} 1.593544011e+09
openstack_ironic_node_updated_at{id="c9f98cc9-25e9-424e-8a89-002989054ec2",name="r1-05",provision_state="available"} 1.592845911e+09
openstack_ironic_node_updated_at{id="d381bea3-8768-4f12-a9b3-abf750ba918f",name="r1-03",provision_state="active"} 1.594162438e+09
openstack_ironic_node_updated_at{id="d5641882-f7e5-4b92-9423-7e8157586218",name="r1-01",provision_state="error"} 1.594708598e+09
openstack_ironic_node_updated_at{id="f50dcc35-4913-4667-a9fa-d130659c5661",name="r1-02",provision_state="available"} 1.594740494e+09
# HELP openstack_ironic_up up
# TYPE openstack_ironic_up gauge
openstack_ironic_up 1
`

func (suite *IronicTestSuite) TestIronicExporter() {
	err := testutil.CollectAndCompare(*suite.Exporter, strings.NewReader(ironicExpectedUp))
	assert.NoError(suite.T(), err)
}

// TestListAllNodesFollowsMarker covers an API whose configured max_limit is
// lower than the requested page size and which omits nodes_links. Collection
// must continue through short pages until the API returns an empty page.
func TestListAllNodesFollowsMarker(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	const (
		nodesURL      = "http://ironic.test/v1/nodes/detail"
		serverMaxSize = 3
	)

	nodePage := func(first, count int) httpmock.Responder {
		entries := make([]string, count)
		for i := range entries {
			entries[i] = fmt.Sprintf(`{"uuid": "node-%04d"}`, first+i)
		}
		resp := httpmock.NewStringResponse(http.StatusOK, `{"nodes": [`+strings.Join(entries, ",")+`]}`)
		resp.Header.Set("Content-Type", "application/json")
		return httpmock.ResponderFromResponse(resp)
	}

	httpmock.RegisterResponder(http.MethodGet,
		nodesURL+"?limit=1000&sort_dir=asc&sort_key=id",
		nodePage(0, serverMaxSize))
	httpmock.RegisterResponder(http.MethodGet,
		nodesURL+"?limit=1000&marker=node-0002&sort_dir=asc&sort_key=id",
		nodePage(serverMaxSize, serverMaxSize))
	httpmock.RegisterResponder(http.MethodGet,
		nodesURL+"?limit=1000&marker=node-0005&sort_dir=asc&sort_key=id",
		nodePage(0, 0))
	httpmock.RegisterNoResponder(httpmock.NewNotFoundResponder(t.Fatal))

	client := &gophercloudv2.ServiceClient{
		ProviderClient: &gophercloudv2.ProviderClient{},
		Endpoint:       "http://ironic.test/v1/",
	}

	allNodes, err := listAllNodes(context.Background(), client)
	require.NoError(t, err)
	require.Len(t, allNodes, 2*serverMaxSize)
	assert.Equal(t, "node-0000", allNodes[0].UUID)
	assert.Equal(t, "node-0005", allNodes[len(allNodes)-1].UUID)
}

// Every subset must keep exactly its requested emitters and fetch shared data
// once per scrape (the fixture has one populated page and one empty page).
func (suite *IronicTestSuite) TestIronicDAGMetricSubsets() {
	original := (*suite.Exporter).(*IronicExporter)
	names := []string{"node", "node_updated_at", "node_provision_updated_at"}
	for mask := 0; mask < 8; mask++ {
		suite.Run(fmt.Sprintf("enabled_%03b", mask), func() {
			suite.installFixtures()
			config := original.ExporterConfig
			config.DisabledMetrics = nil
			for i, name := range names {
				if mask&(1<<i) == 0 {
					config.DisabledMetrics = append(config.DisabledMetrics, "ironic-"+name)
				}
			}
			exporter, err := NewIronicExporter(&config, original.logger)
			require.NoError(suite.T(), err)
			httpmock.ZeroCallCounters()
			registry := prometheus.NewRegistry()
			require.NoError(suite.T(), registry.Register(exporter))
			families, err := registry.Gather()
			require.NoError(suite.T(), err)
			got := make(map[string]bool)
			for _, family := range families {
				got[family.GetName()] = true
				if family.GetName() == "openstack_ironic_up" {
					wantUp := 1.0
					if mask == 0 {
						wantUp = 0
					}
					assert.Equal(suite.T(), wantUp, family.Metric[0].Gauge.GetValue())
				}
			}
			for i, name := range names {
				assert.Equal(suite.T(), mask&(1<<i) != 0, got["openstack_ironic_"+name], name)
			}
			assert.True(suite.T(), got["openstack_ironic_up"])
			calls := 0
			for request, count := range httpmock.GetCallCountInfo() {
				if strings.Contains(request, "/nodes/detail") {
					calls += count
				}
			}
			wantCalls := 2
			if mask == 0 {
				wantCalls = 0
			}
			assert.Equal(suite.T(), wantCalls, calls)
		})
	}
}

func (suite *IronicTestSuite) TestIronicDAGFetchFailure() {
	httpmock.RegisterResponder(http.MethodGet,
		suite.MakeURL("/ironic/v1/nodes/detail?limit=1000&sort_dir=asc&sort_key=id", ""),
		httpmock.NewStringResponder(http.StatusInternalServerError, "failed"))
	err := testutil.CollectAndCompare(*suite.Exporter, strings.NewReader(`
# HELP openstack_ironic_up up
# TYPE openstack_ironic_up gauge
openstack_ironic_up 0
`))
	require.NoError(suite.T(), err)
}

func (suite *IronicTestSuite) TestIronicDAGStateIsScrapeLocal() {
	exporter := (*suite.Exporter).(*IronicExporter)
	exporter.CollectTime = true
	registry := prometheus.NewRegistry()
	require.NoError(suite.T(), registry.Register(exporter))
	families, err := registry.Gather()
	require.NoError(suite.T(), err)
	timingFound := false
	for _, family := range families {
		if family.GetName() == "openstack_metric_collect_seconds" {
			timingFound = true
			assert.GreaterOrEqual(suite.T(), family.Metric[0].Gauge.GetValue(), 0.0)
			assert.Equal(suite.T(), "node", family.Metric[0].Label[0].GetValue())
		}
	}
	require.True(suite.T(), timingFound)
	// Concurrent subsequent scrapes of an empty cloud must not reuse old nodes.
	httpmock.RegisterResponder(http.MethodGet,
		suite.MakeURL("/ironic/v1/nodes/detail?limit=1000&sort_dir=asc&sort_key=id", ""),
		httpmock.NewJsonResponderOrPanic(http.StatusOK, map[string]any{"nodes": []any{}}))
	var group sync.WaitGroup
	for i := 0; i < 2; i++ {
		group.Go(func() {
			err := testutil.CollectAndCompare(exporter, strings.NewReader(`
# HELP openstack_ironic_up up
# TYPE openstack_ironic_up gauge
openstack_ironic_up 1
`), "openstack_ironic_node", "openstack_ironic_node_updated_at", "openstack_ironic_node_provision_updated_at", "openstack_ironic_up")
			assert.NoError(suite.T(), err)
		})
	}
	group.Wait()
}
