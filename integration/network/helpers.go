//go:build !windows

package network

import (
	"context"
	"fmt"
	"testing"

	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/testutil"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
)

// CreateMasterDummy creates a dummy network interface
func CreateMasterDummy(ctx context.Context, t *testing.T, master string) {
	// ip link add <dummy_name> type dummy
	testutil.RunCommand(ctx, "ip", "link", "add", master, "type", "dummy").Assert(t, icmd.Success)
	testutil.RunCommand(ctx, "ip", "link", "set", master, "up").Assert(t, icmd.Success)
}

// CreateVlanInterface creates a vlan network interface
func CreateVlanInterface(ctx context.Context, t *testing.T, master, slave, id string) {
	// ip link add link <master> name <master>.<VID> type vlan id <VID>
	testutil.RunCommand(ctx, "ip", "link", "add", "link", master, "name", slave, "type", "vlan", "id", id).Assert(t, icmd.Success)
	// ip link set <sub_interface_name> up
	testutil.RunCommand(ctx, "ip", "link", "set", slave, "up").Assert(t, icmd.Success)
}

// DeleteInterface deletes a network interface
func DeleteInterface(ctx context.Context, t *testing.T, ifName string) {
	testutil.RunCommand(ctx, "ip", "link", "delete", ifName).Assert(t, icmd.Success)
	testutil.RunCommand(ctx, "iptables", "-t", "nat", "--flush").Assert(t, icmd.Success)
	testutil.RunCommand(ctx, "iptables", "--flush").Assert(t, icmd.Success)
}

// LinkExists verifies that a link exists
func LinkExists(ctx context.Context, t *testing.T, master string) {
	// verify the specified link exists, ip link show <link_name>
	testutil.RunCommand(ctx, "ip", "link", "show", master).Assert(t, icmd.Success)
}

// LinkDoesntExist verifies that a link doesn't exist
func LinkDoesntExist(ctx context.Context, t *testing.T, master string) {
	// verify the specified link doesn't exist, ip link show <link_name>.
	testutil.RunCommand(ctx, "ip", "link", "show", master).Assert(t, icmd.Expected{
		ExitCode: 1,
		Err:      "does not exist",
	})
}

// IsNetworkAvailable provides a comparison to check if a docker network is available
func IsNetworkAvailable(ctx context.Context, c client.NetworkAPIClient, name string) is.Comparison {
	return func() is.Result {
		networks, err := c.NetworkList(ctx, network.ListOptions{})
		if err != nil {
			return is.ResultFromError(err)
		}
		for _, network := range networks {
			if network.Name == name {
				return is.ResultSuccess
			}
		}
		return is.ResultFailure(fmt.Sprintf("could not find network %s", name))
	}
}

// IsNetworkNotAvailable provides a comparison to check if a docker network is not available
func IsNetworkNotAvailable(ctx context.Context, c client.NetworkAPIClient, name string) is.Comparison {
	return func() is.Result {
		networks, err := c.NetworkList(ctx, network.ListOptions{})
		if err != nil {
			return is.ResultFromError(err)
		}
		for _, network := range networks {
			if network.Name == name {
				return is.ResultFailure(fmt.Sprintf("network %s is still present", name))
			}
		}
		return is.ResultSuccess
	}
}
