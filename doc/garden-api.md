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

# List Containers
## Example
~~~~
GET /containers?prop2=bar&prop1=bing

200 Ok
{ handles: [ "match-1", "match-2" ] }
~~~~

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

# Get Info for a Container
## Example
~~~~
GET /containers/:handle/info

200 Ok
{ MemoryStat: .., CpuStat: .., PortMapping: .. }
~~~~

# Destroy a Container
## Example
~~~~
DELETE /containers/:handle
~~~~

# Stop a Container
## Example
~~~~
PUT /containers/:handle/stop
{ "kill":true }
~~~~

# Add files to a Container
## Example
~~~~
PUT /containers/:handle/files?destination=/foo/bar/baz
contents
~~~~

# Get files from a Container
## Example
~~~~
GET /containers/:handle/files?source=/foo/bar/baz

200 Ok
contents
~~~~

# Run a process inside a Container
## Example
~~~~
POST /containers/:handle/processes
{
"path": "/path/to/exe",
"user": "vcap",
 ..
}
~~~~

# Attach to a running process inside a container
## Example
~~~~
GET /containers/:handle/processes/:pid
~~~~

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

# Get current container disk limit
## Example
~~~~
GET /containers/:handle/limits/disk

200 Ok
{ "block_soft": 2, .. }
~~~~

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
