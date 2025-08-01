package network

import (
	"context"

	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/v2/daemon/server/backend"
)

// Backend is all the methods that need to be implemented
// to provide network specific functionality.
type Backend interface {
	GetNetworks(filters.Args, backend.NetworkListConfig) ([]network.Inspect, error)
	CreateNetwork(ctx context.Context, nc network.CreateRequest) (*network.CreateResponse, error)
	ConnectContainerToNetwork(ctx context.Context, containerName, networkName string, endpointConfig *network.EndpointSettings) error
	DisconnectContainerFromNetwork(containerName string, networkName string, force bool) error
	DeleteNetwork(networkID string) error
	NetworksPrune(ctx context.Context, pruneFilters filters.Args) (*network.PruneReport, error)
}

// ClusterBackend is all the methods that need to be implemented
// to provide cluster network specific functionality.
type ClusterBackend interface {
	GetNetworks(filters.Args) ([]network.Inspect, error)
	GetNetwork(name string) (network.Inspect, error)
	GetNetworksByName(name string) ([]network.Inspect, error)
	CreateNetwork(nc network.CreateRequest) (string, error)
	RemoveNetwork(name string) error
}
