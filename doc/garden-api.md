# Ping  
Example: GET /ping
	
# Capacity
##Example
~~~~
GET /capacity

200 Ok
{
"memory_in_bytes": 123,
"disk_in_bytes": 2,
"max_containers": 5,
}
~~~~

##Description
Returns the remaining capacity of the system. Memory_in_bytes in the memory limit of the machine in bytes.
Disk_limit_in_bytes is the disk limit of the machine in bytes.
	
# List Containers
##Example 
~~~~
GET /containers?prop2=bar&prop1=bing

200 Ok
{ handles: [ "match-1", "match-2" ] }
~~~~

##Description
Gets a list of containers and returns their handles. With no query string, gets all containers,
otherwise each key/value pair in the query string is interpreted as a container property to filter by.

# Create a new Container
##Example 
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

##Description

 All parameters are optional.

Parameters:
`bind_mounts`: Contains the paths that should be mounted in the
 container's filesystem. The `src_path` field for every bind mount holds the
 path as seen from the host, where the `dst_path` field holds the path as
 seem from the container.

`grace_time`: Can be used to specify how long a container can go
 unreferenced by any client connection. After this time, the container will
 automatically be destroyed. If not specified, the container will be
 subject to the globally configured grace time.

`handle`: If specified, its value must be used to refer to the
 container in future requests. If it is not specified,
 garden uses its internal container ID as the container handle.

`properties`: A sequence of string key/value pairs providing arbitrary
 data about the container. The keys are assumed to be unique but this is not
 enforced via the protocol.

 **TODO**: `network` and `rootfs`

# Get Info for a Container
##Example
~~~~
GET /containers/:handle/info

200 Ok
{ MemoryStat: .., CpuStat: .., PortMapping: .. }
~~~~

##Description
Returns information about the given container.

Response Parameters:
`state`: Either "active" or "stopped".
`events`: List of events that occurred for the container. It currently includes only "oom" (Out Of Memory) event if it occurred.
`host_ip`: IP address of the host side of the container's virtual ethernet pair.
`container_ip`: IP address of the container side of the container's virtual ethernet pair.
`container_path`: Path to the directory holding the container's files (both its control scripts and filesystem).
`process_ids`: List of running process.
`properties`: List of properties defined for the container.

# Destroy a Container
##Example 
~~~~
DELETE /containers/:handle
~~~~

##Description
When a container is destroyed, its resource allocations are released,
its filesystem is removed, and all references to its handle are removed.

All resources that have been acquired during the lifetime of the container are released.
Examples of these resources are its subnet, its UID, and ports that were redirected to the container.

# Stop a Container
##Example
~~~~
PUT /containers/:handle/stop
{ "kill":true }
~~~~

##Description
Once a container is stopped, garden does not allow spawning new processes inside the container.
It is possible to copy files in to and out of a stopped container.
It is only when a container is destroyed that its filesystem is cleaned up.

Parameters:
`kill`: If true, send SIGKILL instead of SIGTERM. (optional)

# Add files to a Container
##Example
~~~~
PUT /containers/:handle/files?destination=/foo/bar/baz
contents
~~~~

##Description
Sets the contents of a file in the container. The path to the file is specified by the ?destination 
query parameter. The body of the request becoems the body of the file in the container.

# Get files from a Container
##Example
~~~~
GET /containers/:handle/files?source=/foo/bar/baz

200 Ok
contents
~~~~

##Description
Retrieves the contents of a file inside the container, specified by the `source` query parameter.

# Run a process inside a Container
##Example
~~~~
POST /containers/:handle/processes
{
"path": "/path/to/exe",
"privileged": false,
 ..
}
~~~~

##Description
Run a script inside a container.

This request is equivalent to atomically spawning a process and immediately
attaching to it.

### Request

The specified script is interpreted by `/bin/bash` inside the container.

* `handle`: Container handle.
* `path`: Path to command to execute.
* `args`: Arguments to pass to command.
* `privileged`: Whether to run the script as root or not.
* `rlimits`: Resource limits (see `ResourceLimits`).
* `env`: Environment Variables (see `EnvironmentVariable`).
* `dir`: Working directory (default: home directory).
* `tty`: Execute with a TTY for stdio.

### Response

A series of ProcessPayloads are sent as the output is streamed back to the client. Each payload
is a JSON structure with the following fields:

* `process_id`: The process id for the given data
* `source`: The stream source - one of stdin, stdout and stderr
* `data`: The data payload for the given stream source
* `exit_status`: Exit status of the process -- only present if the process has exited

# Attach to a running process inside a container
##Example
~~~~
GET /containers/:handle/processes/:pid
~~~~

##Description

Attaches to a running process and returns the output as a series of ProcessPayloads. Each payload
is a JSON structure with the following fields:

* `process_id`: The process id for the given data
* `source`: The stream source - one of stdin, stdout and stderr
* `data`: The data payload for the given stream source
* `exit_status`: Exit status of the process -- only present if the process has exited

# Limit container bandwidth
Example: PUT /containers/:handle/limits/bandwidth

# Get current container bandwidth limit
Example: PUT /containers/:handle/limits/bandwidth

# Limit container cpu
Example: PUT /containers/:handle/limits/cpu

# Get current container cpu limit
Example: PUT /containers/:handle/limits/cpu

# Limit container disk
Example: PUT /containers/:handle/limits/disk

# Get current container disk limit
Example: PUT /containers/:handle/limits/disk

# Limit container memory
Example: PUT /containers/:handle/limits/memory

# Get current container memory limit
Example: PUT /containers/:handle/limits/memory

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
