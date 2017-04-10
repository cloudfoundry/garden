package routes

import "github.com/tedsuo/rata"

const (
	Ping     = "Ping"
	Capacity = "Capacity"

	List        = "List"
	Create      = "Create"
	Info        = "Info"
	BulkInfo    = "BulkInfo"
	BulkMetrics = "BulkMetrics"
	Destroy     = "Destroy"

	Stop = "Stop"

	StreamIn  = "StreamIn"
	StreamOut = "StreamOut"

	Stdout = "Stdout"
	Stderr = "Stderr"

	CurrentBandwidthLimits = "CurrentBandwidthLimits"
	CurrentCPULimits       = "CurrentCPULimits"
	CurrentDiskLimits      = "CurrentDiskLimits"
	CurrentMemoryLimits    = "CurrentMemoryLimits"

	NetIn      = "NetIn"
	NetOut     = "NetOut"
	BulkNetOut = "BulkNetOut"

	Run    = "Run"
	Attach = "Attach"

	SetGraceTime = "SetGraceTime"

	Properties  = "Properties"
	Property    = "Property"
	SetProperty = "SetProperty"

	Metrics = "Metrics"

	RemoveProperty = "RemoveProperty"
)

var Routes = rata.Routes{
	{Path: "/ping", Method: "GET", Name: Ping},
	{Path: "/capacity", Method: "GET", Name: Capacity},

	{Path: "/containers", Method: "GET", Name: List},
	{Path: "/containers", Method: "POST", Name: Create},

	{Path: "/containers/:handle/info", Method: "GET", Name: Info},
	{Path: "/containers/bulk_info", Method: "GET", Name: BulkInfo},
	{Path: "/containers/bulk_metrics", Method: "GET", Name: BulkMetrics},

	{Path: "/containers/:handle", Method: "DELETE", Name: Destroy},
	{Path: "/containers/:handle/stop", Method: "PUT", Name: Stop},

	{Path: "/containers/:handle/files", Method: "PUT", Name: StreamIn},
	{Path: "/containers/:handle/files", Method: "GET", Name: StreamOut},

	{Path: "/containers/:handle/limits/bandwidth", Method: "GET", Name: CurrentBandwidthLimits},
	{Path: "/containers/:handle/limits/cpu", Method: "GET", Name: CurrentCPULimits},
	{Path: "/containers/:handle/limits/disk", Method: "GET", Name: CurrentDiskLimits},
	{Path: "/containers/:handle/limits/memory", Method: "GET", Name: CurrentMemoryLimits},

	{Path: "/containers/:handle/net/in", Method: "POST", Name: NetIn},
	{Path: "/containers/:handle/net/out", Method: "POST", Name: NetOut},
	{Path: "/containers/:handle/net/out/bulk", Method: "POST", Name: BulkNetOut},

	{Path: "/containers/:handle/processes/:pid/attaches/:streamid/stdout", Method: "GET", Name: Stdout},
	{Path: "/containers/:handle/processes/:pid/attaches/:streamid/stderr", Method: "GET", Name: Stderr},
	{Path: "/containers/:handle/processes", Method: "POST", Name: Run},
	{Path: "/containers/:handle/processes/:pid", Method: "GET", Name: Attach},

	{Path: "/containers/:handle/grace_time", Method: "PUT", Name: SetGraceTime},

	{Path: "/containers/:handle/properties", Method: "GET", Name: Properties},
	{Path: "/containers/:handle/properties/:key", Method: "GET", Name: Property},
	{Path: "/containers/:handle/properties/:key", Method: "PUT", Name: SetProperty},
	{Path: "/containers/:handle/properties/:key", Method: "DELETE", Name: RemoveProperty},

	{Path: "/containers/:handle/metrics", Method: "GET", Name: Metrics},
}
