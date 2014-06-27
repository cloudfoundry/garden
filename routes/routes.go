package routes

import "github.com/tedsuo/rata"

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

var Routes = rata.Routes{
	{Path: "/ping", Method: "GET", Name: Ping},
	{Path: "/capacity", Method: "GET", Name: Capacity},

	{Path: "/containers", Method: "GET", Name: List},
	{Path: "/containers", Method: "POST", Name: Create},

	{Path: "/containers/:handle/info", Method: "GET", Name: Info},

	{Path: "/containers/:handle", Method: "DELETE", Name: Destroy},
	{Path: "/containers/:handle/stop", Method: "PUT", Name: Stop},

	{Path: "/containers/:handle/files", Method: "PUT", Name: StreamIn},
	{Path: "/containers/:handle/files", Method: "GET", Name: StreamOut},

	{Path: "/containers/:handle/limits/bandwidth", Method: "PUT", Name: LimitBandwidth},
	{Path: "/containers/:handle/limits/bandwidth", Method: "GET", Name: CurrentBandwidthLimits},

	{Path: "/containers/:handle/limits/cpu", Method: "PUT", Name: LimitCPU},
	{Path: "/containers/:handle/limits/cpu", Method: "GET", Name: CurrentCPULimits},

	{Path: "/containers/:handle/limits/disk", Method: "PUT", Name: LimitDisk},
	{Path: "/containers/:handle/limits/disk", Method: "GET", Name: CurrentDiskLimits},

	{Path: "/containers/:handle/limits/memory", Method: "PUT", Name: LimitMemory},
	{Path: "/containers/:handle/limits/memory", Method: "GET", Name: CurrentMemoryLimits},

	{Path: "/containers/:handle/net/in", Method: "POST", Name: NetIn},
	{Path: "/containers/:handle/net/out", Method: "POST", Name: NetOut},

	{Path: "/containers/:handle/processes", Method: "POST", Name: Run},
	{Path: "/containers/:handle/processes/:pid", Method: "GET", Name: Attach},
}
