// Package backend includes types to send information to server backends.
package backend

import (
	"io"
	"time"

	"github.com/distribution/reference"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ContainerCreateConfig is the parameter set to ContainerCreate()
type ContainerCreateConfig struct {
	Name                        string
	Config                      *container.Config
	HostConfig                  *container.HostConfig
	NetworkingConfig            *network.NetworkingConfig
	Platform                    *ocispec.Platform
	DefaultReadOnlyNonRecursive bool
}

// ContainerRmConfig holds arguments for the container remove
// operation. This struct is used to tell the backend what operations
// to perform.
type ContainerRmConfig struct {
	ForceRemove, RemoveVolume, RemoveLink bool
}

// ContainerAttachConfig holds the streams to use when connecting to a container to view logs.
type ContainerAttachConfig struct {
	GetStreams func(multiplexed bool, cancel func()) (io.ReadCloser, io.Writer, io.Writer, error)
	UseStdin   bool
	UseStdout  bool
	UseStderr  bool
	Logs       bool
	Stream     bool
	DetachKeys string
	// Used to signify that streams must be multiplexed by producer as endpoint can't manage multiple streams.
	// This is typically set by HTTP endpoint, while websocket can transport raw streams
	MuxStreams bool
}

// PartialLogMetaData provides meta data for a partial log message. Messages
// exceeding a predefined size are split into chunks with this metadata. The
// expectation is for the logger endpoints to assemble the chunks using this
// metadata.
type PartialLogMetaData struct {
	Last    bool   // true if this message is last of a partial
	ID      string // identifies group of messages comprising a single record
	Ordinal int    // ordering of message in partial group
}

// LogMessage is datastructure that represents piece of output produced by some
// container.  The Line member is a slice of an array whose contents can be
// changed after a log driver's Log() method returns.
type LogMessage struct {
	Line         []byte
	Source       string
	Timestamp    time.Time
	Attrs        []LogAttr
	PLogMetaData *PartialLogMetaData

	// Err is an error associated with a message. Completeness of a message
	// with Err is not expected, tho it may be partially complete (fields may
	// be missing, gibberish, or nil)
	Err error
}

// LogAttr is used to hold the extra attributes available in the log message.
type LogAttr struct {
	Key   string
	Value string
}

// LogSelector is a list of services and tasks that should be returned as part
// of a log stream. It is similar to swarmapi.LogSelector, with the difference
// that the names don't have to be resolved to IDs; this is mostly to avoid
// accidents later where a swarmapi LogSelector might have been incorrectly
// used verbatim (and to avoid the handler having to import swarmapi types)
type LogSelector struct {
	Services []string
	Tasks    []string
}

// ContainerStatsConfig holds information for configuring the runtime
// behavior of a backend.ContainerStats() call.
type ContainerStatsConfig struct {
	Stream    bool
	OneShot   bool
	OutStream func() io.Writer
}

// ContainerInspectOptions defines options for the backend.ContainerInspect
// call.
type ContainerInspectOptions struct {
	// Size controls whether to propagate the container's size fields.
	Size bool
}

// ExecStartConfig holds the options to start container's exec.
type ExecStartConfig struct {
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
	ConsoleSize *[2]uint `json:",omitempty"`
}

// ExecInspect holds information about a running process started
// with docker exec.
type ExecInspect = container.ExecInspectResponse

// ExecProcessConfig holds information about the exec process
// running on the host.
type ExecProcessConfig = container.ExecProcessConfig

// CreateImageConfig is the configuration for creating an image from a
// container.
type CreateImageConfig struct {
	Tag     reference.NamedTagged
	Pause   bool
	Author  string
	Comment string
	Config  *container.Config
	Changes []string
}

// GetImageOpts holds parameters to retrieve image information
// from the backend.
type GetImageOpts struct {
	Platform *ocispec.Platform
}

// ImageInspectOpts holds parameters to inspect an image.
type ImageInspectOpts struct {
	Manifests bool
	Platform  *ocispec.Platform
}

// CommitConfig is the configuration for creating an image as part of a build.
type CommitConfig struct {
	Author              string
	Comment             string
	Config              *container.Config // TODO(thaJeztah); change this to [dockerspec.DockerOCIImageConfig]
	ContainerConfig     *container.Config
	ContainerID         string
	ContainerMountLabel string
	ContainerOS         string
	ParentImageID       string
}

// PluginCreateConfig hold all options to plugin create.
type PluginCreateConfig struct {
	RepoName string
}

// PluginRmConfig holds arguments for plugin remove.
type PluginRmConfig struct {
	ForceRemove bool
}

// PluginEnableConfig holds arguments for plugin enable
type PluginEnableConfig struct {
	Timeout int
}

// PluginDisableConfig holds arguments for plugin disable.
type PluginDisableConfig struct {
	ForceDisable bool
}

// NetworkListConfig stores the options available for listing networks
type NetworkListConfig struct {
	// TODO(@cpuguy83): naming is hard, this is pulled from what was being used in the router before moving here
	Detailed bool
	Verbose  bool
}
