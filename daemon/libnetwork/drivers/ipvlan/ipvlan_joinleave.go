//go:build linux

package ipvlan

import (
	"context"
	"fmt"
	"net"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/netlabel"
	"github.com/moby/moby/v2/daemon/libnetwork/netutils"
	"github.com/moby/moby/v2/daemon/libnetwork/ns"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type staticRoute struct {
	Destination *net.IPNet
	RouteType   int
	NextHop     net.IP
}

const (
	defaultV4RouteCidr = "0.0.0.0/0"
	defaultV6RouteCidr = "::/0"
)

// Join method is invoked when a Sandbox is attached to an endpoint.
func (d *driver) Join(ctx context.Context, nid, eid string, sboxKey string, jinfo driverapi.JoinInfo, epOpts, _ map[string]interface{}) error {
	ctx, span := otel.Tracer("").Start(ctx, "libnetwork.drivers.ipvlan.Join", trace.WithAttributes(
		attribute.String("nid", nid),
		attribute.String("eid", eid),
		attribute.String("sboxKey", sboxKey)))
	defer span.End()

	n, err := d.getNetwork(nid)
	if err != nil {
		return err
	}
	endpoint := n.endpoint(eid)
	if endpoint == nil {
		return fmt.Errorf("could not find endpoint with id %s", eid)
	}
	// generate a name for the iface that will be renamed to eth0 in the sbox
	containerIfName, err := netutils.GenerateIfaceName(ns.NlHandle(), vethPrefix, vethLen)
	if err != nil {
		return fmt.Errorf("error generating an interface name: %v", err)
	}
	// create the netlink ipvlan interface
	vethName, err := createIPVlan(containerIfName, n.config.Parent, n.config.IpvlanMode, n.config.IpvlanFlag)
	if err != nil {
		return err
	}
	// bind the generated iface name to the endpoint
	endpoint.srcName = vethName
	ep := n.endpoint(eid)
	if ep == nil {
		return fmt.Errorf("could not find endpoint with id %s", eid)
	}
	if !n.config.Internal {
		switch n.config.IpvlanMode {
		case modeL3, modeL3S:
			// disable gateway services to add a default gw using dev eth0 only
			jinfo.DisableGatewayService()
			if ep.addr != nil {
				defaultRoute, err := ifaceGateway(defaultV4RouteCidr)
				if err != nil {
					return err
				}
				if err := jinfo.AddStaticRoute(defaultRoute.Destination, defaultRoute.RouteType, defaultRoute.NextHop); err != nil {
					return fmt.Errorf("failed to set an ipvlan l3/l3s mode ipv4 default gateway: %v", err)
				}
				log.G(ctx).Debugf("Ipvlan Endpoint Joined with IPv4_Addr: %s, Ipvlan_Mode: %s, Parent: %s",
					ep.addr.IP.String(), n.config.IpvlanMode, n.config.Parent)
			}
			// If the endpoint has a v6 address, set a v6 default route
			if ep.addrv6 != nil {
				default6Route, err := ifaceGateway(defaultV6RouteCidr)
				if err != nil {
					return err
				}
				if err = jinfo.AddStaticRoute(default6Route.Destination, default6Route.RouteType, default6Route.NextHop); err != nil {
					return fmt.Errorf("failed to set an ipvlan l3/l3s mode ipv6 default gateway: %v", err)
				}
				log.G(ctx).Debugf("Ipvlan Endpoint Joined with IPv6_Addr: %s, Ipvlan_Mode: %s, Parent: %s",
					ep.addrv6.IP.String(), n.config.IpvlanMode, n.config.Parent)
			}
		case modeL2:
			// parse and correlate the endpoint v4 address with the available v4 subnets
			if len(n.config.Ipv4Subnets) > 0 {
				s := n.getSubnetforIPv4(ep.addr)
				if s == nil {
					return fmt.Errorf("could not find a valid ipv4 subnet for endpoint %s", eid)
				}
				v4gw, _, err := net.ParseCIDR(s.GwIP)
				if err != nil {
					return fmt.Errorf("gateway %s is not a valid ipv4 address: %v", s.GwIP, err)
				}
				err = jinfo.SetGateway(v4gw)
				if err != nil {
					return err
				}
				log.G(ctx).Debugf("Ipvlan Endpoint Joined with IPv4_Addr: %s, Gateway: %s, Ipvlan_Mode: %s, Parent: %s",
					ep.addr.IP.String(), v4gw.String(), n.config.IpvlanMode, n.config.Parent)
			}
			// parse and correlate the endpoint v6 address with the available v6 subnets
			if len(n.config.Ipv6Subnets) > 0 {
				s := n.getSubnetforIPv6(ep.addrv6)
				if s == nil {
					return fmt.Errorf("could not find a valid ipv6 subnet for endpoint %s", eid)
				}
				v6gw, _, err := net.ParseCIDR(s.GwIP)
				if err != nil {
					return fmt.Errorf("gateway %s is not a valid ipv6 address: %v", s.GwIP, err)
				}
				err = jinfo.SetGatewayIPv6(v6gw)
				if err != nil {
					return err
				}
				log.G(ctx).Debugf("Ipvlan Endpoint Joined with IPv6_Addr: %s, Gateway: %s, Ipvlan_Mode: %s, Parent: %s",
					ep.addrv6.IP.String(), v6gw.String(), n.config.IpvlanMode, n.config.Parent)
			}
			if len(n.config.Ipv4Subnets) == 0 && len(n.config.Ipv6Subnets) == 0 {
				// With no addresses, don't need a gateway.
				jinfo.DisableGatewayService()
			}
		}
	} else {
		if len(n.config.Ipv4Subnets) > 0 {
			log.G(ctx).Debugf("Ipvlan Endpoint Joined with IPv4_Addr: %s, IpVlan_Mode: %s, Parent: %s",
				ep.addr.IP.String(), n.config.IpvlanMode, n.config.Parent)
		}
		if len(n.config.Ipv6Subnets) > 0 {
			log.G(ctx).Debugf("Ipvlan Endpoint Joined with IPv6_Addr: %s IpVlan_Mode: %s, Parent: %s",
				ep.addrv6.IP.String(), n.config.IpvlanMode, n.config.Parent)
		}
		// If n.config.Internal was set locally by the driver because there's no parent
		// interface, libnetwork doesn't know the network is internal. So, stop it from
		// adding a gateway endpoint.
		jinfo.DisableGatewayService()
	}
	iNames := jinfo.InterfaceName()
	err = iNames.SetNames(vethName, containerVethPrefix, netlabel.GetIfname(epOpts))
	if err != nil {
		return err
	}
	if err = d.storeUpdate(ep); err != nil {
		return fmt.Errorf("failed to save ipvlan endpoint %.7s to store: %v", ep.id, err)
	}

	return nil
}

// Leave method is invoked when a Sandbox detaches from an endpoint.
func (d *driver) Leave(nid, eid string) error {
	network, err := d.getNetwork(nid)
	if err != nil {
		return err
	}
	endpoint, err := network.getEndpoint(eid)
	if err != nil {
		return err
	}
	if endpoint == nil {
		return fmt.Errorf("could not find endpoint with id %s", eid)
	}

	return nil
}

// ifaceGateway returns a static route for either v4/v6 to be set to the container eth0
func ifaceGateway(dfNet string) (*staticRoute, error) {
	nh, dst, err := net.ParseCIDR(dfNet)
	if err != nil {
		return nil, fmt.Errorf("unable to parse default route %v", err)
	}
	defaultRoute := &staticRoute{
		Destination: dst,
		RouteType:   types.CONNECTED,
		NextHop:     nh,
	}

	return defaultRoute, nil
}

// getSubnetforIPv4 returns the ipv4 subnet to which the given IP belongs
func (n *network) getSubnetforIPv4(ip *net.IPNet) *ipSubnet {
	return getSubnetForIP(ip, n.config.Ipv4Subnets)
}

// getSubnetforIPv6 returns the ipv6 subnet to which the given IP belongs
func (n *network) getSubnetforIPv6(ip *net.IPNet) *ipSubnet {
	return getSubnetForIP(ip, n.config.Ipv6Subnets)
}

func getSubnetForIP(ip *net.IPNet, subnets []*ipSubnet) *ipSubnet {
	for _, s := range subnets {
		_, snet, err := net.ParseCIDR(s.SubnetIP)
		if err != nil {
			return nil
		}
		// first check if the mask lengths are the same
		i, _ := snet.Mask.Size()
		j, _ := ip.Mask.Size()
		if i != j {
			continue
		}
		if snet.Contains(ip.IP) {
			return s
		}
	}

	return nil
}
