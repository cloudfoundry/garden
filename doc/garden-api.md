# Ping
Example: GET /ping

# Capacity
## Example
~~~~
GET /capacity

200 Ok
{
"memory_in_bytes": 123,
"disk_in_bytes": 2,
"max_containers": 5,
}
~~~~

## Description
Returns the remaining capacity of the system. Memory_in_bytes in the memory limit of the machine in bytes.
Disk_limit_in_bytes is the disk limit of the machine in bytes.

# List Containers
## Example
~~~~
GET /containers?prop2=bar&prop1=bing

200 Ok
{ handles: [ "match-1", "match-2" ] }
~~~~

## Description
Gets a list of containers and returns their handles. With no query string, gets all containers,
otherwise each key/value pair in the query string is interpreted as a container property to filter by.

# Create a new Container
## Example
~~~~
POST /containers
{
 "bind_mounts": [],
 "grace_time": 1200,
 "handle": 'user-supplied-handle',
 "network": 'network',
 "rootfs": 'rootfs',
 "properties": [],
 "env": [] }

200 Ok
{ handle: 'handle-of-created-container' }
~~~~

## Description

All parameters are optional.

### Request Parameters:

* `bind_mounts`: Contains the paths that should be mounted in the
container's filesystem. The `src_path` field for every bind mount holds the
path as seen from the host, where the `dst_path` field holds the path as
seem from the container.

* `grace_time`: Can be used to specify how long a container can go
 unreferenced by any client connection. After this time, the container will
 automatically be destroyed. If not specified, the container will be
 subject to the globally configured grace time.
 
* `handle`: If specified, its value must be used to refer to the
 container in future requests. If it is not specified,
 garden uses its internal container ID as the container handle.

* `network`: Determines the subnet and IP address of a container.
    
    If not specified, a `/30` subnet is allocated from a default network pool.
    
    If specified, it takes the form `a.b.c.d/n` where `a.b.c.d` is an IP address and `n` is the number of
    bits in the network prefix. `a.b.c.d`
    masked by the first `n` bits is the network address of a subnet called the subnet address. If
    the remaining bits are zero (i.e. `a.b.c.d` *is* the subnet address),
    the container is allocated an unused IP address from the
    subnet. Otherwise, the container is given the IP address `a.b.c.d`.
    
    The container IP address cannot be the subnet address or
    the broadcast address of the subnet (all non prefix bits set) or the address
    one less than the broadcast address (which is reserved). 

    Multiple containers may share a subnet by passing the same subnet address on the corresponding
    create calls. Containers on the same subnet can communicate with each other over IP 
    without restriction. In particular, they are not affected by packet filtering.

    An error is returned if:

    * the IP address cannot be allocated or is already in use,
    * the subnet specified overlaps the default network pool, or
    * the subnet specified overlaps (but does not equal) a subnet that has
      already had a container allocated from it.

* `properties`: A sequence of string key/value pairs providing arbitrary
 data about the container. The keys are assumed to be unique but this is not
 enforced via the protocol.

> **TODO**: `rootfs`

# Get Info for a Container
## Example
~~~~
GET /containers/:handle/info

200 Ok
{ MemoryStat: .., CpuStat: .., PortMapping: .. }
~~~~

## Description
Returns information about the given container.

### Response Parameters:

* `state`: Either "active" or "stopped".
* `events`: List of events that occurred for the container. It currently includes only "oom" (Out Of Memory) event if it occurred.
* `host_ip`: IP address of the host side of the container's virtual ethernet pair.
* `container_ip`: IP address of the container side of the container's virtual ethernet pair.
* `container_path`: Path to the directory holding the container's files (both its control scripts and filesystem).
* `process_ids`: List of running process.
* `properties`: List of properties defined for the container.

# Destroy a Container
## Example
~~~~
DELETE /containers/:handle
~~~~

## Description
When a container is destroyed, its resource allocations are released,
its filesystem is removed, and all references to its handle are removed.

All resources that have been acquired during the lifetime of the container are released.
Examples of these resources are its subnet, its UID, and ports that were redirected to the container.

# Stop a Container
## Example
~~~~
PUT /containers/:handle/stop
{ "kill":true }
~~~~

## Description
Once a container is stopped, garden does not allow spawning new processes inside the container.
It is possible to copy files in to and out of a stopped container.
It is only when a container is destroyed that its filesystem is cleaned up.

### Request Parameters:

* `kill`: If true, send SIGKILL instead of SIGTERM. (optional)

# Add files to a Container
## Example
~~~~
PUT /containers/:handle/files?destination=/foo/bar/baz
contents
~~~~

## Description
Sets the contents of a file in the container. The path to the file is specified by the `?destination`
query parameter. The body of the request becoems the body of the file in the container.

# Get files from a Container
## Example
~~~~
GET /containers/:handle/files?source=/foo/bar/baz

200 Ok
contents
~~~~

## Description
Retrieves the contents of a file inside the container, specified by the `source` query parameter.

# Run a process inside a Container
## Example
~~~~
POST /containers/:handle/processes
{
"path": "/path/to/exe",
"privileged": false,
 ..
}
~~~~

## Description
Run a script inside a container.

This request is equivalent to atomically spawning a process and immediately
attaching to it.

### Request Parameters

The specified script is interpreted by `/bin/bash` inside the container.

* `handle`: Container handle.
* `path`: Path to command to execute.
* `args`: Arguments to pass to command.
* `privileged`: Whether to run the script as root or not.
* `rlimits`: Resource limits (see `ResourceLimits`).
* `env`: Environment Variables (see `EnvironmentVariable`).
* `dir`: Working directory (default: home directory).
* `tty`: Execute with a TTY for stdio.

### Response Parameters

A series of ProcessPayloads are sent as the output is streamed back to the client. Each payload
is a JSON structure with the following fields:

* `process_id`: The process id for the given data
* `source`: The stream source - one of stdin, stdout and stderr
* `data`: The data payload for the given stream source
* `exit_status`: Exit status of the process -- only present if the process has exited

# Attach to a running process inside a container
## Example
~~~~
GET /containers/:handle/processes/:pid
~~~~

## Description

Attaches to a running process and returns the output as a series of ProcessPayloads. Each payload
is a JSON structure with the following fields:

* `process_id`: The process id for the given data
* `source`: The stream source - one of stdin, stdout and stderr
* `data`: The data payload for the given stream source
* `exit_status`: Exit status of the process -- only present if the process has exited

# Limit container bandwidth
Example: PUT /containers/:handle/limits/bandwidth

# Get current container bandwidth limit
Example: GET /containers/:handle/limits/bandwidth

# Limit container cpu
## Example
~~~~
PUT /containers/:handle/limits/cpu
{ "limit_in_shares": 2 }
~~~~

Limits container CPU

The field `limit_in_shares` is optional. When it is not specified, the cpu limit will not be changed.

# Get current container cpu limit
## Example
~~~~
GET /containers/:handle/limits/cpu

200 Ok
{ "limit_in_shares": 2 }
~~~~

# Limit container memory
## Example
~~~~
PUT /containers/:handle/limits/memory
{ "limit_in_bytes": 2 }
~~~~

Limits container memory
The limit applies to all process in the container. When the limit is
exceeded, the container will be automatically stopped.

If no limit is given, the current value is returned, and no change is made.

# Get current container memory limit
## Example
~~~~
GET /containers/:handle/limits/memory

200 Ok
{ "limit_in_bytes": 2 }
~~~~

# Limit container disk
## Example
~~~~
PUT /containers/:handle/limits/disk
{ "block_soft": 2, "block_hard": 2, .. }
~~~~

Limits the disk usage for a container.

The disk limits that are set by this command only have effect for the container's unprivileged user.
Files/directories created by its privileged user are not subject to these limits.

> **TODO**: Link to page explaining how disk management works.

### Request Parameters

* `block_soft`: New soft block limit.
* `block_hard`: New hard block limit.
* `inode_soft`: New soft inode limit.
* `inode_hard`: New hard inode limit.
* `byte_soft`: New soft block limit specified in bytes. Only has effect when `block_soft` is not specified.
* `byte_hard`: New hard block limit specified in bytes. Only has effect when `block_hard` is not specified.

# Get current container disk limit
## Example
~~~~
GET /containers/:handle/limits/disk

200 Ok
{ "block_soft": 2, .. }
~~~~

### Response Parameters

* `block_soft`: New soft block limit.
* `block_hard`: New hard block limit.
* `inode_soft`: New soft inode limit.
* `inode_hard`: New hard inode limit.
* `byte_soft`: New soft block limit specified in bytes.
* `byte_hard`: New hard block limit specified in bytes.

# Allow a container port to be accessed externally
Example: POST /containers/:handle/net/in

# Allow a container to access external networks and ports
Example: POST /containers/:handle/net/out

# Get a container metadata property
Example: GET /containers/:handle/properties/:key

# Set a container metadata property
Example: PUT /containers/:handle/properties/:key

# Delete a container metadata property
Example: DELETE /containers/:handle/properties/:key
