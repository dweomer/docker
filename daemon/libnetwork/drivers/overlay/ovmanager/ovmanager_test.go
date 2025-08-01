package ovmanager

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/netlabel"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func parseCIDR(t *testing.T, ipnet string) *net.IPNet {
	subnet, err := types.ParseCIDR(ipnet)
	assert.NilError(t, err)
	return subnet
}

func TestNetworkAllocateFree(t *testing.T) {
	d := newDriver()

	ipamData := []driverapi.IPAMData{
		{
			Pool: parseCIDR(t, "10.1.1.0/24"),
		},
		{
			Pool: parseCIDR(t, "10.1.2.0/24"),
		},
	}

	vals, err := d.NetworkAllocate("testnetwork", nil, ipamData, nil)
	assert.NilError(t, err)

	vxlanIDs, ok := vals[netlabel.OverlayVxlanIDList]
	assert.Check(t, is.Equal(true, ok))
	assert.Check(t, is.Len(strings.Split(vxlanIDs, ","), 2))

	err = d.NetworkFree("testnetwork")
	assert.NilError(t, err)
}

func TestNetworkAllocateUserDefinedVNIs(t *testing.T) {
	d := newDriver()

	ipamData := []driverapi.IPAMData{
		{
			Pool: parseCIDR(t, "10.1.1.0/24"),
		},
		{
			Pool: parseCIDR(t, "10.1.2.0/24"),
		},
	}

	options := make(map[string]string)
	// Intentionally add mode vnis than subnets
	options[netlabel.OverlayVxlanIDList] = fmt.Sprintf("%d,%d,%d", vxlanIDStart, vxlanIDStart+1, vxlanIDStart+2)

	vals, err := d.NetworkAllocate("testnetwork", options, ipamData, nil)
	assert.NilError(t, err)

	vxlanIDs, ok := vals[netlabel.OverlayVxlanIDList]
	assert.Check(t, is.Equal(true, ok))

	// We should only get exactly the same number of vnis as
	// subnets. No more, no less, even if we passed more vnis.
	assert.Check(t, is.Len(strings.Split(vxlanIDs, ","), 2))
	assert.Check(t, is.Equal(fmt.Sprintf("%d,%d", vxlanIDStart, vxlanIDStart+1), vxlanIDs))

	err = d.NetworkFree("testnetwork")
	assert.NilError(t, err)
}
