package exporters

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

type MasakariTestSuite struct {
	BaseOpenStackTestSuite
}

var masakariExpectedUp = `
# HELP openstack_masakari_host host
# TYPE openstack_masakari_host gauge
openstack_masakari_host{control_attributes="SSH",failover_segment_id="9e800031-6946-4b43-bf09-8b3d1cab792b",failover_segment_name="segment1",hostname="compute-01",id="1",type="COMPUTE_HOST",uuid="083a8474-22c0-407f-b89b-c569134c3bfd"} 1
openstack_masakari_host{control_attributes="SSH",failover_segment_id="9e800031-6946-4b43-bf09-8b3d1cab792b",failover_segment_name="segment1",hostname="compute-02",id="2",type="COMPUTE_HOST",uuid="1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"} 1
# HELP openstack_masakari_host_on_maintenance host_on_maintenance
# TYPE openstack_masakari_host_on_maintenance gauge
openstack_masakari_host_on_maintenance{failover_segment_id="9e800031-6946-4b43-bf09-8b3d1cab792b",hostname="compute-01",uuid="083a8474-22c0-407f-b89b-c569134c3bfd"} 0
openstack_masakari_host_on_maintenance{failover_segment_id="9e800031-6946-4b43-bf09-8b3d1cab792b",hostname="compute-02",uuid="1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"} 1
# HELP openstack_masakari_host_reserved host_reserved
# TYPE openstack_masakari_host_reserved gauge
openstack_masakari_host_reserved{failover_segment_id="9e800031-6946-4b43-bf09-8b3d1cab792b",hostname="compute-01",uuid="083a8474-22c0-407f-b89b-c569134c3bfd"} 0
openstack_masakari_host_reserved{failover_segment_id="9e800031-6946-4b43-bf09-8b3d1cab792b",hostname="compute-02",uuid="1a2b3c4d-5e6f-4a7b-8c9d-0e1f2a3b4c5d"} 1
# HELP openstack_masakari_segment segment
# TYPE openstack_masakari_segment gauge
openstack_masakari_segment{description="main segment",id="1",name="segment1",recovery_method="auto",service_type="COMPUTE",uuid="9e800031-6946-4b43-bf09-8b3d1cab792b"} 1
# HELP openstack_masakari_up up
# TYPE openstack_masakari_up gauge
openstack_masakari_up 1
`

func (suite *MasakariTestSuite) TestMasakariExporter() {
	err := testutil.CollectAndCompare(*suite.Exporter, strings.NewReader(masakariExpectedUp))
	assert.NoError(suite.T(), err)
}

func (suite *MasakariTestSuite) TestDisabledHostMetrics() {
	for _, name := range []string{"host", "host_on_maintenance", "host_reserved"} {
		suite.Run(name, func() {
			suite.teardownFixtures()
			suite.installFixtures()
			original := (*suite.Exporter).(*MasakariExporter)
			config := original.ExporterConfig
			config.DisabledMetrics = []string{"masakari-" + name}
			exporter, err := NewMasakariExporter(&config, original.logger)
			suite.Require().NoError(err)
			registry := prometheus.NewRegistry()
			suite.Require().NoError(registry.Register(exporter))
			metrics, err := registry.Gather()
			suite.Require().NoError(err)
			names := make(map[string]bool)
			for _, metric := range metrics {
				names[metric.GetName()] = true
			}
			suite.False(names["openstack_masakari_"+name])
			suite.True(names["openstack_masakari_segment"])
			suite.True(names["openstack_masakari_up"])
		})
	}
}
