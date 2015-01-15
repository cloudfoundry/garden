package garden

import (
	"fmt"
	"time"
)

type Client interface {
	// Pings the garden server.
	//
	// Errors:
	// * None.
	Ping() error

	// Capacity returns the physical capacity of the server's machine.
	//
	// Errors:
	// * None.
	Capacity() (Capacity, error)

	// Create creates a new container.
	//
	// Errors:
	// * When the handle, if specified, is already taken.
	// * When one of the bind_mount paths does not exist.
	// * When resource allocations fail (subnet, user ID, etc).
	Create(ContainerSpec) (Container, error)

	// Destroy destroys a container.
	//
	// When a container is destroyed, its resource allocations are released,
	// its filesystem is removed, and all references to its handle are removed.
	//
	// All resources that have been acquired during the lifetime of the container are released.
	// Examples of these resources are its subnet, its UID, and ports that were redirected to the container.
	//
	// TODO: list the resources that can be acquired during the lifetime of a container.
	//
	// Errors:
	// * TODO.
	Destroy(handle string) error

	// Containers lists all containers filtered by Properties (which are ANDed together).
	//
	// Errors:
	// * None.
	Containers(Properties) ([]Container, error)

	// Lookup returns the container with the specified handle.
	//
	// Errors:
	// * Container not found.
	Lookup(handle string) (Container, error)
}

type ContainerNotFoundError struct {
	Handle string
}

func (err ContainerNotFoundError) Error() string {
	return fmt.Sprintf("unknown handle: %s", err.Handle)
}

// ContainerSpec specifies the parameters for creating a container. All parameters are optional.
type ContainerSpec struct {

	// Handle, if specified, is used to refer to the
	// container in future requests. If it is not specified,
	// garden uses its internal container ID as the container handle.
	Handle string

	// GraceTime can be used to specify how long a container can go
	// unreferenced by any client connection. After this time, the container will
	// automatically be destroyed. If not specified, the container will be
	// subject to the globally configured grace time.
	GraceTime time.Duration

	// TODO
	RootFSPath string

	// * bind_mounts: a list of mount point descriptions which will result in corresponding mount
	// points being created in the container's file system.
	//
	// An error is returned if:
	// * one or more of the mount points has a non-existent source directory, or
	// * one or more of the mount points cannot be created.
	BindMounts []BindMount

	// Network determines the subnet and IP address of a container.
	//
	// If not specified, a /30 subnet is allocated from a default network pool.
	//
	// If specified, it takes the form a.b.c.d/n where a.b.c.d is an IP address and n is the number of
	// bits in the network prefix. a.b.c.d masked by the first n bits is the network address of a subnet
	// called the subnet address. If the remaining bits are zero (i.e. a.b.c.d *is* the subnet address),
	// the container is allocated an unused IP address from the subnet. Otherwise, the container is given
	// the IP address a.b.c.d.
	//
	// The container IP address cannot be the subnet address or the broadcast address of the subnet
	// (all non prefix bits set) or the address one less than the broadcast address (which is reserved).
	//
	// Multiple containers may share a subnet by passing the same subnet address on the corresponding
	// create calls. Containers on the same subnet can communicate with each other over IP
	// without restriction. In particular, they are not affected by packet filtering.
	//
	// An error is returned if:
	// * the IP address cannot be allocated or is already in use,
	// * the subnet specified overlaps the default network pool, or
	// * the subnet specified overlaps (but does not equal) a subnet that has
	//   already had a container allocated from it.
	Network string

	// Properties is a sequence of string key/value pairs providing arbitrary
	// data about the container. The keys are assumed to be unique but this is not
	// enforced via the protocol.
	Properties Properties

	// TODO
	Env []string

	// If Privileged is true the container does not have a user namespace and the root user in the container
	// is the same as the root user in the host. Otherwise, the container has a user namespace and the root
	// user in the container is mapped to a non-root user in the host. Defaults to false.
	Privileged bool
}

// BindMount specifies parameters for a single mount point.
//
// Each mount point is mounted (with the bind option) into the container's file system.
// The effective permissions of the mount point are the permissions of the source directory if the mode
// is read-write and the permissions of the source directory with the write bits turned off if the mode
// of the mount point is read-only.
type BindMount struct {
	// SrcPath contains the path of the directory to be mounted.
	SrcPath string

	// DstPath contains the path of the mount point in the container. If the
	// directory does not exist, it is created.
	DstPath string

	// Mode must be either "RO" or "RW". Alternatively, mode may be omitted and defaults to RO.
	// If mode is "RO", a read-only mount point is created.
	// If mode is "RW", a read-write mount point is created.
	Mode BindMountMode

	// BindMountOrigin must be either "Host" or "Container". Alternatively, origin may be omitted and
	// defaults to "Host".
	// If origin is "Host", src_path denotes a path in the host.
	// If origin is "Container", src_path denotes a path in the container.
	Origin BindMountOrigin
}

type Capacity struct {
	MemoryInBytes uint64
	DiskInBytes   uint64
	MaxContainers uint64
}

type Properties map[string]string

type BindMountMode uint8

const BindMountModeRO BindMountMode = 0
const BindMountModeRW BindMountMode = 1

type BindMountOrigin uint8

const BindMountOriginHost BindMountOrigin = 0
const BindMountOriginContainer BindMountOrigin = 1
