package routes

import "github.com/tedsuo/router"

const (
	Ping     = "Ping"
	Capacity = "Capacity"

	List    = "List"
	Create  = "Create"
	Info    = "Info"
	Destroy = "Destroy"

	Stop = "Stop"

	StreamIn  = "StreamIn"
	StreamOut = "StreamOut"

	LimitBandwidth         = "LimitBandwidth"
	CurrentBandwidthLimits = "CurrentBandwidthLimits"

	LimitCPU         = "LimitCPU"
	CurrentCPULimits = "CurrentCPULimits"

	LimitDisk         = "LimitDisk"
	CurrentDiskLimits = "CurrentDiskLimits"

	LimitMemory         = "LimitMemory"
	CurrentMemoryLimits = "CurrentMemoryLimits"

	NetIn  = "NetIn"
	NetOut = "NetOut"

	Run    = "Run"
	Attach = "Attach"
)

var Routes = router.Routes{
	{Path: "/ping", Method: "GET", Handler: Ping},
	{Path: "/capacity", Method: "GET", Handler: Capacity},

	{Path: "/containers", Method: "GET", Handler: List},
	{Path: "/containers", Method: "POST", Handler: Create},

	{Path: "/containers/:handle/info", Method: "GET", Handler: Info},

	{Path: "/containers/:handle", Method: "DELETE", Handler: Destroy},
	{Path: "/containers/:handle/stop", Method: "PUT", Handler: Stop},

	{Path: "/containers/:handle/files", Method: "PUT", Handler: StreamIn},
	{Path: "/containers/:handle/files", Method: "GET", Handler: StreamOut},

	{Path: "/containers/:handle/limits/bandwidth", Method: "PUT", Handler: LimitBandwidth},
	{Path: "/containers/:handle/limits/bandwidth", Method: "GET", Handler: CurrentBandwidthLimits},

	{Path: "/containers/:handle/limits/cpu", Method: "PUT", Handler: LimitCPU},
	{Path: "/containers/:handle/limits/cpu", Method: "GET", Handler: CurrentCPULimits},

	{Path: "/containers/:handle/limits/disk", Method: "PUT", Handler: LimitDisk},
	{Path: "/containers/:handle/limits/disk", Method: "GET", Handler: CurrentDiskLimits},

	{Path: "/containers/:handle/limits/memory", Method: "PUT", Handler: LimitMemory},
	{Path: "/containers/:handle/limits/memory", Method: "GET", Handler: CurrentMemoryLimits},

	{Path: "/containers/:handle/net/in", Method: "POST", Handler: NetIn},
	{Path: "/containers/:handle/net/out", Method: "POST", Handler: NetOut},

	{Path: "/containers/:handle/processes", Method: "POST", Handler: Run},
	{Path: "/containers/:handle/processes/:pid", Method: "GET", Handler: Attach},
}
