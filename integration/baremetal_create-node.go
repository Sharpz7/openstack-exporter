package integration

import (
	"context"
	"testing"

	"github.com/gophercloud/gophercloud/v2/openstack/baremetal/v1/nodes"
	"github.com/openstack-exporter/openstack-exporter/integration/clients"

	th "github.com/gophercloud/gophercloud/v2/testhelper"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/openstack-exporter/openstack-exporter/integration/tools"
)

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

func TestNodesCreateDestroy(t *testing.T) {
	clients.RequireLong(t)
	clients.RequireIronicHTTPBasic(t)

	client, err := clients.NewBareMetalV1HTTPBasic()
	th.AssertNoErr(t, err)
	client.Microversion = "1.50"

	CreateNode(t, client)
	th.AssertNoErr(t, err)
}
