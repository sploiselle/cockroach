package catalog

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/cockroachdb/cockroach/pkg/util/metric"

	prometheusgo "github.com/prometheus/client_model/go"
)

type chartDescription struct {
	Name         string
	Organization [][]string // inner array is "Level 0", "Level 1", (opt) "Level 2";
	// outer array lets you categorize the same chart in
	// multiple places
	Metrics     []string
	Units       AxisUnits // Defaults to first metric's preferred units
	AxisLabel   string    // Default to first metric's AxisLabel
	Downsampler string    // Zero value is AVG
	Aggregator  string    // AVG by default
	Rate        string    // Default depends on type
	Percentiles bool      // True only for Histograms
}

type chartDefaults struct {
	Downsampler string // AVG by default
	Aggregator  string // AVG by default
	Rate        string // Default depends on type
	Percentiles bool   // True only for Latency and Histogram metrics
}

var chartDefaultsPerMetricType = map[prometheusgo.MetricType]chartDefaults{
	prometheusgo.MetricType_COUNTER: chartDefaults{
		Downsampler: "AVG",
		Aggregator:  "AVG",
		Rate:        "Non-negative rate",
		Percentiles: false,
	},
	prometheusgo.MetricType_GAUGE: chartDefaults{
		Downsampler: "AVG",
		Aggregator:  "AVG",
		Rate:        "Normal",
		Percentiles: false,
	},
	prometheusgo.MetricType_HISTOGRAM: chartDefaults{
		Downsampler: "AVG",
		Aggregator:  "AVG",
		Rate:        "Normal",
		Percentiles: true,
	},
}

// These consts represent the highest-level of taxonomy in the catalog and
// correspond roughly to CockroachDB's architecture documentation
const (
	Process            = `Process`
	SQLLayer           = `SQL Layer`
	KVTransactionLayer = `KV Transaction Layer`
	DistributionLayer  = `Distribution Layer`
	ReplicationLayer   = `Replication Layer`
	StorageLayer       = `Storage Layer`
	Timeseries         = `Timeseries`
)

var charts = []chartDescription{
	{
		Name:         "Abandoned",
		Organization: [][]string{{KVTransactionLayer, "Transactions"}},
		Metrics:      []string{"txn.abandons"},
	},
	{
		Name:         "Aborts",
		Organization: [][]string{{KVTransactionLayer, "Transactions"}},
		Metrics:      []string{"txn.aborts"},
	},
	{
		Name:         "Add Replica Count",
		Organization: [][]string{{ReplicationLayer, "Replicate Queue"}},
		Metrics:      []string{"queue.replicate.addreplica"},
	},
	{
		Name:         "Ingestions",
		Organization: [][]string{{StorageLayer, "RocksDB", "SSTables"}},
		Metrics: []string{"addsstable.copies",
			"addsstable.applications",
			"addsstable.proposals"},
	},
	{
		Name:         "Auto Retries",
		Organization: [][]string{{KVTransactionLayer, "Transactions"}},
		Metrics:      []string{"txn.autoretries"},
	},
	{
		Name:         "Capacity",
		Organization: [][]string{{StorageLayer, "Storage", "Overview"}},
		Metrics: []string{"capacity.available",
			"capacity",
			"capacity.reserved",
			"capacity.used"},
	},
	{
		Name:         "Keys/Sec Avg.",
		Organization: [][]string{{ReplicationLayer, "Raft", "Overview"}},
		Metrics:      []string{"rebalancing.writespersecond"},
	},
	{
		Name: "Writes Waiting on Range Split",
		Organization: [][]string{
			{KVTransactionLayer, "Requests", "Backpressure"},
			{ReplicationLayer, "Requests", "Backpressure"},
		},
		Downsampler: "MAX",
		Aggregator:  "MAX",
		Rate:        "Normal",
		Percentiles: false,
		Metrics:     []string{"requests.backpressure.split"},
	},
	{
		Name:         "Backpressued Writes Waiting on Split",
		Organization: [][]string{{ReplicationLayer, "Ranges"}},
		Metrics:      []string{"requests.backpressure.split"},
	},
	{
		Name:         "Batches",
		Organization: [][]string{{DistributionLayer, "DistSender"}},
		Metrics: []string{"distsender.batches",
			"distsender.batches.partial"},
	},
	{
		Name:         "Timestamp",
		Organization: [][]string{{Process, "Build Info"}},
		Downsampler:  "MAX",
		Aggregator:   "MAX",
		Rate:         "Normal",
		Percentiles:  false,
		Metrics:      []string{"build.timestamp"},
	},
	{
		Name:         "Sizes",
		Organization: [][]string{{StorageLayer, "Storage", "Compactor"}},
		Metrics: []string{
			"compactor.suggestionbytes.compacted",
			"compactor.suggestionbytes.queued",
			"compactor.suggestionbytes.skipped",
		},
	},
	{
		Name:         "Byte I/O",
		Organization: [][]string{{SQLLayer, "SQL"}},
		Metrics: []string{"sql.bytesin",
			"sql.bytesout"},
	},
	{
		Name:         "Bytes",
		Organization: [][]string{{DistributionLayer, "Gossip"}},
		Metrics: []string{"gossip.bytes.received",
			"gossip.bytes.sent"},
	},
	// {
	// 	Name:         "CA Expiration",
	// 	Organization: [][]string{{Process, "Certificates"}},
	// 	Downsampler:  "MAX",
	// 	Aggregator:   "MAX",
	// 	Rate:         "Normal",
	// 	Percentiles:  false,
	// 	Metrics:      []string{"security.certificate.expiration.ca"},
	// },
	{
		Name:         "Memory",
		Organization: [][]string{{Process, "Server", "cgo"}},
		Metrics: []string{"sys.cgo.allocbytes",
			"sys.cgo.totalbytes"},
	},
	{
		Name:         "Calls",
		Organization: [][]string{{Process, "Server", "cgo"}},
		Metrics:      []string{"sys.cgocalls"},
	},
	{
		Name:         "Offsets",
		Organization: [][]string{{KVTransactionLayer, "Clocks"}},
		Metrics: []string{"clock-offset.meannanos",
			"clock-offset.stddevnanos"},
	},
	{
		Name:         "Offsets",
		Organization: [][]string{{Process, "Clocks"}},
		Metrics: []string{"clock-offset.meannanos",
			"clock-offset.stddevnanos"},
	},
	{
		Name:         "Counts",
		Organization: [][]string{{ReplicationLayer, "Replicas", "Command Queue"}},
		Metrics: []string{"replicas.commandqueue.combinedqueuesize",
			"replicas.commandqueue.combinedreadcount",
			"replicas.commandqueue.combinedwritecount"},
	},
	{
		Name:         "Commits",
		Organization: [][]string{{KVTransactionLayer, "Transactions"}},
		Metrics: []string{"txn.commits",
			"txn.commits1PC"},
	},
	{
		Name:         "Time",
		Organization: [][]string{{StorageLayer, "Storage", "Compactor"}},
		Metrics:      []string{"compactor.compactingnanos"},
	},
	{
		Name:         "Success",
		Organization: [][]string{{StorageLayer, "Storage", "Compactor"}},
		Metrics: []string{"compactor.compactions.failure",
			"compactor.compactions.success"},
	},
	{
		Name:         "Connections",
		Organization: [][]string{{DistributionLayer, "Gossip"}},
		Downsampler:  "MAX",
		Aggregator:   "AVG",
		Rate:         "Normal",
		Percentiles:  false,
		Metrics: []string{"gossip.connections.refused",
			"gossip.connections.incoming",
			"gossip.connections.outgoing"},
	},
	{
		Name:         "Connections",
		Organization: [][]string{{SQLLayer, "SQL"}},
		Metrics:      []string{"sql.conns"},
	},
	{
		Name:         "Count",
		Organization: [][]string{{ReplicationLayer, "Consistency Checker Queue"}},
		Metrics: []string{"queue.consistency.process.failure",
			"queue.consistency.pending",
			"queue.consistency.process.success"},
	},
	{
		Name:         "Time Spent",
		Organization: [][]string{{ReplicationLayer, "Consistency Checker Queue"}},
		Metrics:      []string{"queue.consistency.processingnanos"},
	},
	{
		Name:         "Time",
		Organization: [][]string{{Process, "CPU"}},
		Metrics: []string{"sys.cpu.sys.ns",
			"sys.cpu.user.ns"},
	},
	{
		Name:         "Percentage",
		Organization: [][]string{{Process, "CPU"}},
		Metrics: []string{"sys.cpu.sys.percent",
			"sys.cpu.user.percent"},
	},
	{
		Name:         "Current Memory Usage",
		Organization: [][]string{{SQLLayer, "DistSQL"}},
		Metrics:      []string{"sql.mem.distsql.current"},
	},
	{
		Name:         "Current",
		Organization: [][]string{{SQLLayer, "SQL Memory", "Admin"}},
		Metrics:      []string{"sql.mem.admin.current"},
	},
	{
		Name:         "Current",
		Organization: [][]string{{SQLLayer, "SQL Memory", "Internal"}},
		Metrics:      []string{"sql.mem.internal.current"},
	},
	{
		Name:         "Current",
		Organization: [][]string{{SQLLayer, "SQL Memory", "Connections"}},
		Metrics:      []string{"sql.mem.conns.current"},
	},
	{
		Name:         "DDL Count",
		Organization: [][]string{{SQLLayer, "SQL"}},
		Metrics: []string{"sql.ddl.count",
			"sql.query.count"},
	},
	{
		Name:         "DML Mix",
		Organization: [][]string{{SQLLayer, "SQL"}},
		Metrics: []string{"sql.delete.count",
			"sql.insert.count",
			"sql.misc.count",
			"sql.query.count",
			"sql.select.count",
			"sql.update.count"},
	},
	{
		Name:         "Exec Latency",
		Organization: [][]string{{SQLLayer, "DistSQL"}},
		Metrics:      []string{"sql.distsql.exec.latency"},
	},
	{
		Name:         "DML Mix",
		Organization: [][]string{{SQLLayer, "DistSQL"}},
		Metrics:      []string{"sql.distsql.select.count"},
	},
	{
		Name:         "Service Latency",
		Organization: [][]string{{SQLLayer, "DistSQL"}},
		Metrics:      []string{"sql.distsql.service.latency"},
	},
	{
		Name:         "Durations",
		Organization: [][]string{{KVTransactionLayer, "Transactions"}},
		Metrics:      []string{"txn.durations"},
	},
	{
		Name:         "Epoch Increment Count",
		Organization: [][]string{{ReplicationLayer, "Node Liveness"}},
		Metrics:      []string{"liveness.epochincrements"},
	},
	{
		Name:         "Success",
		Organization: [][]string{{KVTransactionLayer, "Requests", "Overview"}},
		Downsampler:  "MAX",
		Aggregator:   "AVG",
		Rate:         "Rate",
		Percentiles:  false,
		Metrics: []string{"exec.error",
			"exec.success"},
	},
	{
		Name:         "File Descriptors (FD)",
		Organization: [][]string{{Process, "Server", "Overview"}},
		Metrics: []string{"sys.fd.open",
			"sys.fd.softlimit"},
	},
	{
		Name:         "Active Flows",
		Organization: [][]string{{SQLLayer, "DistSQL"}},
		Metrics:      []string{"sql.distsql.flows.active"},
	},
	{
		Name:         "Total Flows",
		Organization: [][]string{{SQLLayer, "DistSQL"}},
		Metrics:      []string{"sql.distsql.flows.total"},
	},
	{
		Name:         "AbortSpan",
		Organization: [][]string{{KVTransactionLayer, "Garbage Collection (GC)", "Keys"}},
		Metrics: []string{"queue.gc.info.abortspanconsidered",
			"queue.gc.info.abortspangcnum",
			"queue.gc.info.abortspanscanned"},
	},
	{
		Name:         "Cumultative Age of Non-Live Data",
		Organization: [][]string{{KVTransactionLayer, "Storage"}},
		Metrics:      []string{"gcbytesage"},
	},
	{
		Name:         "Cumultative Age of Non-Live Data",
		Organization: [][]string{{StorageLayer, "Storage", "KV"}},
		Metrics:      []string{"gcbytesage"},
	},
	{
		Name:         "Total GC Runs",
		Organization: [][]string{{KVTransactionLayer, "Garbage Collection (GC)", "Overview"}},
		Metrics:      []string{"sys.gc.count"},
	},
	{
		Name:         "Old Intents",
		Organization: [][]string{{KVTransactionLayer, "Garbage Collection (GC)", "Keys"}},
		Metrics:      []string{"queue.gc.info.intentsconsidered"},
	},
	{
		Name:         "Distinct Txns",
		Organization: [][]string{{KVTransactionLayer, "Garbage Collection (GC)", "Keys"}},
		Metrics:      []string{"queue.gc.info.intenttxns"},
	},
	{
		Name:         "Keys with GC'able Data",
		Organization: [][]string{{KVTransactionLayer, "Garbage Collection (GC)", "Keys"}},
		Metrics:      []string{"queue.gc.info.numkeysaffected"},
	},
	{
		Name:         "Total GC Pause (NS)",
		Organization: [][]string{{KVTransactionLayer, "Garbage Collection (GC)", "Overview"}},
		Metrics:      []string{"sys.gc.pause.ns"},
	},
	{
		Name:         "Current GC Pause Percent",
		Organization: [][]string{{KVTransactionLayer, "Garbage Collection (GC)", "Overview"}},
		Metrics:      []string{"sys.gc.pause.percent"},
	},
	{
		Name:         "Pushes",
		Organization: [][]string{{KVTransactionLayer, "Garbage Collection (GC)", "Keys"}},
		Metrics:      []string{"queue.gc.info.pushtxn"},
	},
	{
		Name: "Queue Success",
		Organization: [][]string{{ReplicationLayer, "Garbage Collection"},
			{
				StorageLayer, "Garbage Collection"}},
		Metrics: []string{"queue.gc.process.failure",
			"queue.gc.pending",
			"queue.gc.process.success"},
	},
	{
		Name: "Queue Time",
		Organization: [][]string{{ReplicationLayer, "Garbage Collection"},
			{
				StorageLayer, "Garbage Collection"}},
		Metrics: []string{"queue.gc.processingnanos"},
	},
	{
		Name:         "Intents",
		Organization: [][]string{{KVTransactionLayer, "Garbage Collection (GC)", "Keys"}},
		Metrics: []string{"queue.gc.info.resolvesuccess",
			"queue.gc.info.resolvetotal"},
	},
	{
		Name:         "Txn Relationship",
		Organization: [][]string{{KVTransactionLayer, "Garbage Collection (GC)", "Keys"}},
		Metrics: []string{"queue.gc.info.transactionspangcaborted",
			"queue.gc.info.transactionspangccommitted",
			"queue.gc.info.transactionspangcpending"},
	},
	{
		Name:         "Enteries in Txn Spans",
		Organization: [][]string{{KVTransactionLayer, "Garbage Collection (GC)", "Keys"}},
		Metrics:      []string{"queue.gc.info.transactionspanscanned"},
	},
	{
		Name:         "Memory",
		Organization: [][]string{{Process, "Server", "go"}},
		Metrics: []string{"sys.go.allocbytes",
			"sys.go.totalbytes"},
	},
	{
		Name:         "goroutines",
		Organization: [][]string{{Process, "Server", "go"}},
		Metrics:      []string{"sys.goroutines"},
	},
	{
		Name:         "Heartbeats Success",
		Organization: [][]string{{ReplicationLayer, "Node Liveness"}},
		Metrics: []string{"liveness.heartbeatfailures",
			"liveness.heartbeatsuccesses"},
	},
	{
		Name:         "Heartbeat Latency",
		Organization: [][]string{{ReplicationLayer, "Node Liveness"}},
		Metrics:      []string{"liveness.heartbeatlatency"},
	},
	{
		Name:         "Infos",
		Organization: [][]string{{DistributionLayer, "Gossip"}},
		Metrics: []string{"gossip.infos.received",
			"gossip.infos.sent"},
	},
	{
		Name:         "Cumultative Intent Age",
		Organization: [][]string{{KVTransactionLayer, "Storage"}},
		Metrics:      []string{"intentage"},
	},
	{
		Name:         "Cumultative Intent Age",
		Organization: [][]string{{StorageLayer, "Storage", "KV"}},
		Metrics:      []string{"intentage"},
	},
	{
		Name:         "Size",
		Organization: [][]string{{KVTransactionLayer, "Storage"}},
		Metrics: []string{"intentbytes",
			"keybytes",
			"livebytes",
			"sysbytes",
			"totalbytes",
			"valbytes"},
	},
	{
		Name:         "Size",
		Organization: [][]string{{StorageLayer, "Storage", "KV"}},
		Metrics: []string{"intentbytes",
			"keybytes",
			"livebytes",
			"sysbytes",
			"totalbytes",
			"valbytes"},
	},
	{
		Name:         "Counts",
		Organization: [][]string{{KVTransactionLayer, "Storage"}},
		AxisLabel:    "MVCC Keys & Values",
		Metrics: []string{"intentcount",
			"keycount",
			"livecount",
			"syscount",
			"valcount"},
	},
	{
		Name:         "Counts",
		Organization: [][]string{{StorageLayer, "Storage", "KV"}},
		AxisLabel:    "MVCC Keys & Values",
		Metrics: []string{"intentcount",
			"keycount",
			"livecount",
			"syscount",
			"valcount"},
	},
	{
		Name:         "Metric Update Frequency",
		Organization: [][]string{{KVTransactionLayer, "Storage"}},
		Downsampler:  "AVG",
		Aggregator:   "AVG",
		Rate:         "Rate",
		Percentiles:  false,
		Metrics:      []string{"lastupdatenanos"},
	},
	{
		Name:         "Metric Update Frequency",
		Organization: [][]string{{StorageLayer, "Storage", "KV"}},
		Metrics:      []string{"lastupdatenanos"},
	},
	{
		Name:         "Latency",
		Organization: [][]string{{KVTransactionLayer, "Requests", "Overview"}},
		Metrics:      []string{"exec.latency"},
	},
	{
		Name:         "Roundtrip Latency",
		Organization: [][]string{{KVTransactionLayer, "Clocks"}},
		Metrics:      []string{"round-trip-latency"},
	},
	{
		Name:         "Roundtrip Latency",
		Organization: [][]string{{Process, "Clocks"}},
		Metrics:      []string{"round-trip-latency"},
	},
	{
		Name:         "Total",
		Organization: [][]string{{ReplicationLayer, "Leases"}},
		Metrics: []string{"leases.epoch",
			"leases.expiration",
			"replicas.leaseholders",
			"replicas.leaders_not_leaseholders"},
	},
	{
		Name:         "Leaseholders",
		Organization: [][]string{{ReplicationLayer, "Replicas", "Overview"}},
		Metrics:      []string{"replicas.leaseholders"},
	},
	{
		Name:         "Succcess Rate",
		Organization: [][]string{{ReplicationLayer, "Leases"}},
		Metrics: []string{"leases.error",
			"leases.success"},
	},
	{
		Name:         "Transfer Success Rate",
		Organization: [][]string{{ReplicationLayer, "Leases"}},
		Metrics: []string{"leases.transfers.error",
			"leases.transfers.success"},
	},
	{
		Name:         "Node Count",
		Organization: [][]string{{ReplicationLayer, "Node Liveness"}},
		Downsampler:  "MAX",
		Aggregator:   "MAX",
		Rate:         "Normal",
		Percentiles:  false,
		Metrics:      []string{"liveness.livenodes"},
	},
	{
		Name:         "RPCs",
		Organization: [][]string{{DistributionLayer, "DistSender"}},
		Metrics: []string{"distsender.rpc.sent.local",
			"distsender.rpc.sent"},
	},
	{
		Name:         "Memory Usage per Statement",
		Organization: [][]string{{SQLLayer, "DistSQL"}},
		Metrics:      []string{"sql.mem.distsql.max"},
	},
	{
		Name:         "All",
		Organization: [][]string{{SQLLayer, "SQL Memory", "Admin"}},
		Metrics:      []string{"sql.mem.admin.max"},
	},
	{
		Name:         "All",
		Organization: [][]string{{SQLLayer, "SQL Memory", "Internal"}},
		Metrics:      []string{"sql.mem.internal.max"},
	},
	{
		Name:         "All",
		Organization: [][]string{{SQLLayer, "SQL Memory", "Connections"}},
		Metrics:      []string{"sql.mem.conns.max"},
	},
	{
		Name:         "Command Maxes",
		Organization: [][]string{{ReplicationLayer, "Replicas", "Command Queue"}},
		Metrics: []string{
			"replicas.commandqueue.maxoverlaps",
			"replicas.commandqueue.maxreadcount",
			"replicas.commandqueue.maxsize",
			"replicas.commandqueue.maxwritecount",
		},
	},
	{
		Name:         "Tree Size Max",
		Organization: [][]string{{ReplicationLayer, "Replicas", "Command Queue"}},
		Metrics:      []string{"replicas.commandqueue.maxtreesize"},
	},
	{
		Name:         "Errors",
		Organization: [][]string{{DistributionLayer, "DistSender"}},
		Metrics: []string{
			"distsender.rpc.sent.nextreplicaerror",
			"distsender.errors.notleaseholder",
			"requests.slow.distsender",
		},
	},
	// {
	// 	Name:         "Node Cert Expiration",
	// 	Organization: [][]string{{Process, "Certificates"}},
	// 	Downsampler:  "MAX",
	// 	Aggregator:   "MAX",
	// 	Rate:         "Normal",
	// 	Percentiles:  false,
	// 	Metrics:      []string{"security.certificate.expiration.node"},
	// },
	{
		Name:         "ID",
		Organization: [][]string{{Process, "Node"}},
		Downsampler:  "MAX",
		Aggregator:   "MAX",
		Rate:         "Normal",
		Percentiles:  false,
		Metrics:      []string{"node-id"},
	},
	{
		Name:         "Page Rotations",
		Organization: [][]string{{KVTransactionLayer, "Timestamp Cache"}},
		Metrics: []string{"tscache.skl.read.rotations",
			"tscache.skl.write.rotations"},
	},
	{
		Name:         "Page Counts",
		Organization: [][]string{{KVTransactionLayer, "Timestamp Cache"}},
		Metrics: []string{"tscache.skl.read.pages",
			"tscache.skl.write.pages"},
	},
	{
		Name:         "Active Queries",
		Organization: [][]string{{SQLLayer, "DistSQL"}},
		Metrics:      []string{"sql.distsql.queries.active"},
	},
	{
		Name:         "Total Queries",
		Organization: [][]string{{SQLLayer, "DistSQL"}},
		Metrics:      []string{"sql.distsql.queries.total"},
	},
	{
		Name:         "Count",
		Organization: [][]string{{ReplicationLayer, "Replicas", "Overview"}},
		Metrics: []string{"replicas.quiescent",
			"replicas",
			"replicas.reserved"},
	},
	{
		Name:         "Pending",
		Organization: [][]string{{ReplicationLayer, "Raft", "Heartbeats"}},
		Metrics:      []string{"raft.heartbeats.pending"},
	},
	{
		Name:         "Command Commit",
		Organization: [][]string{{ReplicationLayer, "Raft", "Latency"}},
		Metrics:      []string{"raft.process.commandcommit.latency"},
	},
	{
		Name:         "Commands Count",
		Organization: [][]string{{ReplicationLayer, "Raft", "Overview"}},
		Metrics:      []string{"raft.commandsapplied"},
	},
	{
		Name:         "Enqueued",
		Organization: [][]string{{ReplicationLayer, "Raft", "Overview"}},
		Metrics:      []string{"raft.enqueued.pending"},
	},
	{
		Name:         "Leaders",
		Organization: [][]string{{ReplicationLayer, "Raft", "Overview"}},
		Metrics:      []string{"replicas.leaders"},
	},
	{
		Name:         "Log Commit",
		Organization: [][]string{{ReplicationLayer, "Raft", "Latency"}},
		Metrics:      []string{"raft.process.logcommit.latency"},
	},
	{
		Name:         "Followers Behind By...",
		Organization: [][]string{{ReplicationLayer, "Raft", "Log"}},
		Metrics:      []string{"raftlog.behind"},
	},
	{
		Name:         "Log Status",
		Organization: [][]string{{ReplicationLayer, "Raft", "Queues"}},
		Metrics: []string{"queue.raftlog.process.failure",
			"queue.raftlog.pending",
			"queue.raftlog.process.success"},
	},
	{
		Name:         "Log Processing Time Spent",
		Organization: [][]string{{ReplicationLayer, "Raft", "Queues"}},
		Metrics:      []string{"queue.raftlog.processingnanos"},
	},
	{
		Name:         "Entries Truncated",
		Organization: [][]string{{ReplicationLayer, "Raft", "Log"}},
		Metrics:      []string{"raftlog.truncated"},
	},
	{
		Name:         "MsgApp Count",
		Organization: [][]string{{ReplicationLayer, "Raft", "Received"}},
		Metrics:      []string{"raft.rcvd.app"},
	},
	{
		Name:         "MsgAppResp Count",
		Organization: [][]string{{ReplicationLayer, "Raft", "Received"}},
		Metrics:      []string{"raft.rcvd.appresp"},
	},
	{
		Name:         "Dropped",
		Organization: [][]string{{ReplicationLayer, "Raft", "Received"}},
		Metrics:      []string{"raft.rcvd.dropped"},
	},
	{
		Name:         "Heartbeat Count",
		Organization: [][]string{{ReplicationLayer, "Raft", "Received"}},
		Metrics:      []string{"raft.rcvd.heartbeat"},
	},
	{
		Name:         "MsgHeartbeatResp Count",
		Organization: [][]string{{ReplicationLayer, "Raft", "Received"}},
		Metrics:      []string{"raft.rcvd.heartbeatresp"},
	},
	{
		Name:         "MsgHeartbeatResp Count",
		Organization: [][]string{{ReplicationLayer, "Raft", "Heartbeats"}},
		Metrics:      []string{"raft.rcvd.heartbeatresp"},
	},
	{
		Name:         "MsgPreVote Count",
		Organization: [][]string{{ReplicationLayer, "Raft", "Received"}},
		Metrics:      []string{"raft.rcvd.prevote"},
	},
	{
		Name:         "MsgPreVoteResp Count",
		Organization: [][]string{{ReplicationLayer, "Raft", "Received"}},
		Metrics:      []string{"raft.rcvd.prevoteresp"},
	},
	{
		Name:         "MsgProp Count",
		Organization: [][]string{{ReplicationLayer, "Raft", "Received"}},
		Metrics:      []string{"raft.rcvd.prop"},
	},
	{
		Name:         "MsgSnap Count",
		Organization: [][]string{{ReplicationLayer, "Raft", "Received"}},
		Metrics:      []string{"raft.rcvd.snap"},
	},
	{
		Name:         "MsgTimeoutNow Count",
		Organization: [][]string{{ReplicationLayer, "Raft", "Received"}},
		Metrics:      []string{"raft.rcvd.timeoutnow"},
	},
	{
		Name:         "MsgTransferLeader Count",
		Organization: [][]string{{ReplicationLayer, "Raft", "Received"}},
		Metrics:      []string{"raft.rcvd.transferleader"},
	},
	{
		Name:         "MsgTransferLeader Count",
		Organization: [][]string{{ReplicationLayer, "Raft", "Heartbeats"}},
		Metrics:      []string{"raft.rcvd.transferleader"},
	},
	{
		Name:         "MsgVote Count",
		Organization: [][]string{{ReplicationLayer, "Raft", "Received"}},
		Metrics:      []string{"raft.rcvd.vote"},
	},
	{
		Name:         "MsgVoteResp Count",
		Organization: [][]string{{ReplicationLayer, "Raft", "Received"}},
		Metrics:      []string{"raft.rcvd.voteresp"},
	},
	{
		Name:         "Snapshot Status",
		Organization: [][]string{{ReplicationLayer, "Raft", "Queues"}},
		Metrics: []string{"queue.raftsnapshot.process.failure",
			"queue.raftsnapshot.pending",
			"queue.raftsnapshot.process.success"},
	},
	{
		Name:         "Snapshot Processing Time Spent",
		Organization: [][]string{{ReplicationLayer, "Raft", "Queues"}},
		Metrics:      []string{"queue.raftsnapshot.processingnanos"},
	},
	{
		Name:         "Working vs. Ticking TIme",
		Organization: [][]string{{ReplicationLayer, "Raft", "Overview"}},
		Metrics: []string{"raft.process.tickingnanos",
			"raft.process.workingnanos"},
	},
	{
		Name:         "Ticks Queued",
		Organization: [][]string{{ReplicationLayer, "Raft", "Overview"}},
		Metrics:      []string{"raft.ticks"},
	},
	{
		Name: "Add, Split, Remove",
		Organization: [][]string{{DistributionLayer, "Ranges"},
			{
				ReplicationLayer, "Ranges"}},
		Metrics: []string{"range.adds",
			"range.removes",
			"range.splits"},
	},
	{
		Name:         "Overview",
		Organization: [][]string{{DistributionLayer, "Ranges"}},
		Metrics:      []string{"ranges"},
	},
	{
		Name:         "Overview",
		Organization: [][]string{{ReplicationLayer, "Ranges"}},
		Metrics: []string{"ranges",
			"ranges.unavailable",
			"ranges.underreplicated"},
	},
	{
		Name:         "Leader Transfers",
		Organization: [][]string{{ReplicationLayer, "Raft", "Overview"}},
		Metrics:      []string{"range.raftleadertransfers"},
	},
	{
		Name:         "Raft Leader Transfers",
		Organization: [][]string{{ReplicationLayer, "Ranges"}},
		Metrics:      []string{"range.raftleadertransfers"},
	},
	{
		Name: "Snapshots",
		Organization: [][]string{{DistributionLayer, "Ranges"},
			{ReplicationLayer, "Ranges"}},
		Metrics: []string{"range.snapshots.generated",
			"range.snapshots.normal-applied",
			"range.snapshots.preemptive-applied"},
	},
	{
		Name:         "Success",
		Organization: [][]string{{StorageLayer, "RocksDB", "Block Cache"}},
		Metrics: []string{"rocksdb.block.cache.hits",
			"rocksdb.block.cache.misses"},
	},
	{
		Name:         "Size",
		Organization: [][]string{{StorageLayer, "RocksDB", "Block Cache"}},
		Metrics: []string{"rocksdb.block.cache.pinned-usage",
			"rocksdb.block.cache.usage"},
	},
	{
		Name:         "Bloom Filter",
		Organization: [][]string{{StorageLayer, "RocksDB", "Overview"}},
		Metrics: []string{"rocksdb.bloom.filter.prefix.checked",
			"rocksdb.bloom.filter.prefix.useful"},
	},
	{
		Name:         "Compactions",
		Organization: [][]string{{StorageLayer, "RocksDB", "Overview"}},
		Metrics:      []string{"rocksdb.compactions"},
	},
	{
		Name:         "Flushes",
		Organization: [][]string{{StorageLayer, "RocksDB", "Overview"}},
		Metrics:      []string{"rocksdb.flushes"},
	},
	{
		Name:         "Memtable",
		Organization: [][]string{{StorageLayer, "RocksDB", "Overview"}},
		Metrics:      []string{"rocksdb.memtable.total-size"},
	},
	{
		Name:         "Count",
		Organization: [][]string{{StorageLayer, "RocksDB", "SSTables"}},
		Metrics:      []string{"rocksdb.num-sstables"},
	},
	{
		Name:         "Read Amplification",
		Organization: [][]string{{StorageLayer, "RocksDB", "Overview"}},
		Metrics:      []string{"rocksdb.read-amplification"},
	},
	{
		Name:         "Index & Filter Block Size",
		Organization: [][]string{{StorageLayer, "RocksDB", "Overview"}},
		Metrics:      []string{"rocksdb.table-readers-mem-estimate"},
	},
	{
		Name:         "Reblance Count",
		Organization: [][]string{{ReplicationLayer, "Replicate Queue"}},
		Metrics:      []string{"queue.replicate.rebalancereplica"},
	},
	{
		Name:         "Remove Replica Count",
		Organization: [][]string{{ReplicationLayer, "Replicate Queue"}},
		Metrics: []string{"queue.replicate.removedeadreplica",
			"queue.replicate.removereplica"},
	},
	{
		Name:         "Removal Count",
		Organization: [][]string{{ReplicationLayer, "Replica GC Queue"}},
		Metrics:      []string{"queue.replicagc.removereplica"},
	},
	{
		Name:         "Count",
		Organization: [][]string{{ReplicationLayer, "Replica GC Queue"}},
		Metrics: []string{"queue.replicagc.process.failure",
			"queue.replicagc.pending",
			"queue.replicagc.process.success"},
	},
	{
		Name:         "Time Spent",
		Organization: [][]string{{ReplicationLayer, "Replica GC Queue"}},
		Metrics:      []string{"queue.replicagc.processingnanos"},
	},
	{
		Name:         "Count",
		Organization: [][]string{{ReplicationLayer, "Replicate Queue"}},
		Metrics: []string{"queue.replicate.process.failure",
			"queue.replicate.pending",
			"queue.replicate.purgatory",
			"queue.replicate.process.success"},
	},
	{
		Name:         "Time Spent",
		Organization: [][]string{{ReplicationLayer, "Replicate Queue"}},
		Metrics:      []string{"queue.replicate.processingnanos"},
	},
	{
		Name:         "Restarts",
		Organization: [][]string{{KVTransactionLayer, "Transactions"}},
		Downsampler:  "MAX",
		Aggregator:   "MAX",
		Rate:         "Normal",
		Percentiles:  true,
		Metrics:      []string{"txn.restarts"},
	},
	{
		Name:         "Restart Cause Mix",
		Organization: [][]string{{KVTransactionLayer, "Transactions"}},
		Metrics: []string{"txn.restarts.deleterange",
			"txn.restarts.possiblereplay",
			"txn.restarts.serializable",
			"txn.restarts.writetooold"},
	},
	{
		Name:         "RSS",
		Organization: [][]string{{Process, "Server", "Overview"}},
		Metrics:      []string{"sys.rss"},
	},
	{
		Name:         "Session Current",
		Organization: [][]string{{SQLLayer, "SQL Memory", "Admin"}},
		Metrics:      []string{"sql.mem.admin.session.current"},
	},
	{
		Name:         "Session Current",
		Organization: [][]string{{SQLLayer, "SQL Memory", "Internal"}},
		Metrics:      []string{"sql.mem.internal.session.current"},
	},
	{
		Name:         "Session Current",
		Organization: [][]string{{SQLLayer, "SQL Memory", "Connections"}},
		Metrics:      []string{"sql.mem.conns.session.current"},
	},
	{
		Name:         "Session All",
		Organization: [][]string{{SQLLayer, "SQL Memory", "Admin"}},
		Metrics:      []string{"sql.mem.admin.session.max"},
	},
	{
		Name:         "Session All",
		Organization: [][]string{{SQLLayer, "SQL Memory", "Internal"}},
		Metrics:      []string{"sql.mem.internal.session.max"},
	},
	{
		Name:         "Session All",
		Organization: [][]string{{SQLLayer, "SQL Memory", "Connections"}},
		Metrics:      []string{"sql.mem.conns.session.max"},
	},
	{
		Name: "Stuck in Command Queue",
		Organization: [][]string{{KVTransactionLayer, "Requests", "Slow"},
			{
				ReplicationLayer, "Requests", "Slow"}},
		Downsampler: "MAX",
		Aggregator:  "MAX",
		Rate:        "Normal",
		Percentiles: false,
		Metrics:     []string{"requests.slow.commandqueue"},
	},
	{
		Name: "Stuck Acquiring Lease",
		Organization: [][]string{{KVTransactionLayer, "Requests", "Slow"},
			{
				ReplicationLayer, "Requests", "Slow"}},
		Downsampler: "MAX",
		Aggregator:  "MAX",
		Rate:        "Normal",
		Percentiles: false,
		Metrics:     []string{"requests.slow.commandqueue"},
	},
	{
		Name:         "Stuck Request Count",
		Organization: [][]string{{ReplicationLayer, "Replicas", "Command Queue"}},
		Metrics:      []string{"requests.slow.commandqueue"},
	},
	{
		Name:         "Stuck Acquisition Count",
		Organization: [][]string{{ReplicationLayer, "Leases"}},
		Metrics:      []string{"requests.slow.lease"},
	},
	{
		Name: "Stuck in Raft",
		Organization: [][]string{{KVTransactionLayer, "Requests", "Slow"},
			{
				ReplicationLayer, "Requests", "Slow"}},
		Downsampler: "MAX",
		Aggregator:  "MAX",
		Rate:        "Normal",
		Percentiles: false,
		Metrics:     []string{"requests.slow.raft"},
	},
	{
		Name:         "Stuck Request Count",
		Organization: [][]string{{ReplicationLayer, "Raft", "Overview"}},
		Metrics:      []string{"requests.slow.raft"},
	},
	{
		Name: "Count",
		Organization: [][]string{{DistributionLayer, "Split Queue"},
			{
				ReplicationLayer, "Split Queue"}},
		Downsampler: "MAX",
		Aggregator:  "AVG",
		Rate:        "Rate",
		Percentiles: false,
		Metrics: []string{"queue.split.process.failure",
			"queue.split.pending",
			"queue.split.process.success"},
	},
	{
		Name: "Time Spent",
		Organization: [][]string{{DistributionLayer, "Split Queue"},
			{
				ReplicationLayer, "Split Queue"}},
		Metrics: []string{"queue.split.processingnanos"},
	},
	{
		Name:         "Exec Latency",
		Organization: [][]string{{SQLLayer, "SQL"}},
		Metrics:      []string{"sql.exec.latency"},
	},
	{
		Name:         "Service Latency",
		Organization: [][]string{{SQLLayer, "SQL"}},
		Metrics:      []string{"sql.service.latency"},
	},
	{
		Name:         "Count",
		Organization: [][]string{{Timeseries, "Maintenance Queue"}},
		Metrics: []string{"queue.tsmaintenance.process.success",
			"queue.tsmaintenance.pending",
			"queue.tsmaintenance.process.failure"},
	},
	{
		Name:         "Time Spent",
		Organization: [][]string{{Timeseries, "Maintenance Queue"}},
		Metrics:      []string{"queue.tsmaintenance.processingnanos"},
	},
	{
		Name:         "Lease Transfer Count",
		Organization: [][]string{{ReplicationLayer, "Replicate Queue"}},
		Metrics:      []string{"queue.replicate.transferlease"},
	},
	{
		Name:         "Transaction Control Mix",
		Organization: [][]string{{SQLLayer, "SQL"}},
		Metrics: []string{"sql.txn.abort.count",
			"sql.txn.begin.count",
			"sql.txn.commit.count",
			"sql.txn.rollback.count"},
	},
	{
		Name:         "Txn Current",
		Organization: [][]string{{SQLLayer, "SQL Memory", "Admin"}},
		Metrics:      []string{"sql.mem.admin.txn.current"},
	},
	{
		Name:         "Txn Current",
		Organization: [][]string{{SQLLayer, "SQL Memory", "Internal"}},
		Metrics:      []string{"sql.mem.internal.txn.current"},
	},
	{
		Name:         "Txn Current",
		Organization: [][]string{{SQLLayer, "SQL Memory", "Connections"}},
		Metrics:      []string{"sql.mem.conns.txn.current"},
	},
	{
		Name:         "Txn All",
		Organization: [][]string{{SQLLayer, "SQL Memory", "Admin"}},
		Metrics:      []string{"sql.mem.admin.txn.max"},
	},
	{
		Name:         "Txn All",
		Organization: [][]string{{SQLLayer, "SQL Memory", "Internal"}},
		Metrics:      []string{"sql.mem.internal.txn.max"},
	},
	{
		Name:         "Txn All",
		Organization: [][]string{{SQLLayer, "SQL Memory", "Connections"}},
		Metrics:      []string{"sql.mem.conns.txn.max"},
	},
	{
		Name:         "Uptime",
		Organization: [][]string{{Process, "Server", "Overview"}},
		Metrics:      []string{"sys.uptime"},
	},
	{
		Name:         "Size",
		Organization: [][]string{{Timeseries, "Overview"}},
		Metrics:      []string{"timeseries.write.bytes"},
	},
	{
		Name:         "Error Count",
		Organization: [][]string{{Timeseries, "Overview"}},
		Metrics:      []string{"timeseries.write.errors"},
	},
	{
		Name:         "Count",
		Organization: [][]string{{Timeseries, "Overview"}},
		Metrics:      []string{"timeseries.write.samples"}},
}

type ChartCatalog []ChartSection

var cc ChartCatalog = ChartCatalog{
	{
		Name:           Process,
		Longname:       Process,
		Collectionname: "process-all",
		Description: `These charts detail the overall performance of the <code>cockroach</code> 
		process running on this server.`,
		Level: 0,
	},
	{
		Name:           SQLLayer,
		Longname:       SQLLayer,
		Collectionname: "sql-layer-all",
		Description: `In the SQL layer, nodes receive commands and then parse, plan, and 
		execute them. <br/><br/><a class="catalog-link" href="https://www.cockroachlabs.com
		/docs/stable/architecture/sql-layer.html">SQL Layer Architecture Docs >></a>"`,
		Level: 0,
	},
	{
		Name:           KVTransactionLayer,
		Longname:       KVTransactionLayer,
		Collectionname: "kv-transaction-layer-all",
		Description: `The KV Transaction Layer coordinates concurrent requests as key-value 
		operations. To maintain consistency, this is also where the cluster manages time. <br/>
		<br/><a class="catalog-link" href="https://www.cockroachlabs.com/docs/stable/architecture
		/transaction-layer.html">Transaction Layer Architecture Docs >></a>`,
		Level: 0,
	},
	{
		Name:           DistributionLayer,
		Longname:       DistributionLayer,
		Collectionname: "distribution-layer-all",
		Description: `The Distribution Layer provides a unified view of your cluster’s data, 
		which are actually broken up into many key-value ranges. <br/><br/><a class="catalog-link" 
		href="https://www.cockroachlabs.com/docs/stable/architecture/distribution-layer.html"> 
		Distribution Layer Architecture Docs >></a>`,
		Level: 0,
	},
	{
		Name:           ReplicationLayer,
		Longname:       ReplicationLayer,
		Collectionname: "replication-layer-all",
		Description: `The Replication Layer maintains consistency between copies of ranges (known 
			as replicas) through our consensus algorithm, Raft. <br/><br/><a class="catalog-link" 
			href="https://www.cockroachlabs.com/docs/stable/architecture/replication-layer.html"> 
			Replication Layer Architecture Docs >></a>`,
		Level: 0,
	},
	{
		Name:           StorageLayer,
		Longname:       StorageLayer,
		Collectionname: "replication-layer-all",
		Description: `The Storage Layer reads and writes data to disk, as well as manages garbage 
		collection. <br/><br/><a class="catalog-link" href="https://www.cockroachlabs.com/docs/stable
		/architecture/storage-layer.html">Storage Layer Architecture Docs >></a>`,
		Level: 0,
	},
	{
		Name:           Timeseries,
		Longname:       Timeseries,
		Collectionname: "timeseries-all",
		Description: `Your cluster collects data about its own performance, which is used to power the 
		very charts you\'re using, among other things.`,
		Level: 0,
	},
}

var catalogKey = map[string]int{
	Process:            0,
	SQLLayer:           1,
	KVTransactionLayer: 2,
	DistributionLayer:  3,
	ReplicationLayer:   4,
	StorageLayer:       5,
	Timeseries:         6,
}

// Converts between metric.DisplayUnit and catalog.AxisUnits which is necessary
// because charts only support a subset of unit types
var metadataUnitsToChartUnits = map[metric.DisplayUnit]AxisUnits{
	metric.DisplayUnit_Bytes:       AxisUnits_Bytes,
	metric.DisplayUnit_Const:       AxisUnits_Count,
	metric.DisplayUnit_Count:       AxisUnits_Count,
	metric.DisplayUnit_Nanoseconds: AxisUnits_Duration,
	metric.DisplayUnit_Percent:     AxisUnits_Count,
	metric.DisplayUnit_Timestamp:   AxisUnits_Duration,
}

func GenerateCatalog(metadata map[string]metric.Metadata) ChartCatalog {

	// Range over all chartDescriptions
	for _, cd := range charts {
		// Range over each level of organization.
		for _, organization := range cd.Organization {

			// If the chart has no metrics, don't add it
			if len(cd.Metrics) == 0 {
				continue
			}

			// Make sure the chart is in the catalog.
			parentCatalogIndex, ok := catalogKey[organization[0]]

			if !ok {
				log.Fatal("Trying to put something where it can't exist")
			}

			// collectionNameSlugs stores cd.Organization information
			// which is used to create collection names for ChartSections
			collectionNameArr := createCollectionNameArr(organization)

			ic := createIndividualChart(metadata, cd, organization, collectionNameArr)

			cc[parentCatalogIndex].addChart(organization, collectionNameArr, ic)
		}
	}

	return cc
}

// createCollectionNameArr creates a lower-case and hyphenated version of the
// organization array, which is used for IndividualChart.Collectionname
func createCollectionNameArr(organization []string) []string {

	collectionNameArr := make([]string, len(organization))

	makeDashes := regexp.MustCompile("( )|/|,")

	for k, n := range organization {
		collectionNameArr[k] = makeDashes.ReplaceAllString(strings.ToLower(n), "-")
	}

	return collectionNameArr
}

func createIndividualChart(
	metadata map[string]metric.Metadata,
	cd chartDescription,
	organization []string,
	collectionNameArr []string) IndividualChart {

	var ic IndividualChart
	ic.addNames(cd.Name, organization, collectionNameArr)
	ic.addMetrics(metadata, cd.Metrics)

	// Get Charts Defaults from its first Metrc's MetricType, which is only available from
	// metadata
	mt := metadata[ic.Data[0].Name].MetricType

	d := chartDefaultsPerMetricType[mt]

	ic.addDisplayProperties(cd, d)

	return ic
}

func (ic *IndividualChart) addNames(
	chartTitle string, organization []string, collectionNameArr []string) {

	ic.Title = chartTitle

	for k, n := range organization {
		// Collectionnames look like "sql-layer-sql-connections".
		ic.Collectionname = ic.Collectionname + collectionNameArr[k] + "-"
		// Longnames end up looking like "SQL Layer | SQL | Connections".
		ic.Longname = ic.Longname + n + " | "
	}
	makeDashes := regexp.MustCompile("( )|/|,")
	ic.Collectionname += makeDashes.ReplaceAllString(strings.ToLower(chartTitle), "-")
	ic.Longname += chartTitle

}

func (ic *IndividualChart) addMetrics(metadata map[string]metric.Metadata, metricNames []string) {
	for _, n := range metricNames {

		md, ok := metadata[n]

		// If metric is missing from metadata, don't add it to this chart
		// because we won't be able to calculate it
		if !ok {
			fmt.Printf("Trying to use metric %v, but it doesn't exist\n", n)
		}

		ic.Data = append(ic.Data, &ChartMetric{
			Name:           md.Name,
			Help:           md.Help,
			AxisLabel:      md.Unit,
			PreferredUnits: metadataUnitsToChartUnits[md.DisplayUnit],
		})

		if metadata[ic.Data[0].Name].MetricType != metadata[md.Name].MetricType {
			fmt.Printf("%v and %v have different MetricTypes\n", ic.Data[0].Name, md.Name)
		}
	}
}

// addDisplayProperties requires addMetrics be completed first
func (ic *IndividualChart) addDisplayProperties(cd chartDescription, d chartDefaults) {

	// Set all zero values to the chartDefault's value
	if cd.Downsampler == "" {
		cd.Downsampler = d.Downsampler
	}
	if cd.Aggregator == "" {
		cd.Aggregator = d.Aggregator
	}
	if cd.Rate == "" {
		cd.Rate = d.Rate
	}
	if cd.Percentiles == false {
		cd.Percentiles = d.Percentiles
	}

	// Set unspecified AxisUnits to the first metric's value
	if cd.Units == AxisUnits_Unset {
		cd.Units = ic.Data[0].PreferredUnits
	}

	// Set unspecified AxisLabels to the first metric's value
	if cd.AxisLabel == "" {
		cd.AxisLabel = ic.Data[0].AxisLabel
	}

	// Populate rest of thisChart
	ic.Downsampler = cd.Downsampler
	ic.Aggregator = cd.Aggregator
	ic.Derivative = cd.Rate
	ic.Percentiles = cd.Percentiles
	ic.Units = cd.Units
	ic.AxisLabel = cd.AxisLabel
}

func (cs *ChartSection) addChart(organization []string, collectionNameArr []string, ic IndividualChart) {
	var childSection *ChartSection
	childLevel := int(cs.Level + 1)

	var found bool

	for _, s := range cs.Subsections {
		if s.Name == organization[childLevel] {
			found = true
			childSection = s
			break
		}
	}

	if !found {
		childSection = &ChartSection{
			Name:           organization[childLevel],
			Longname:       "All",
			Collectionname: collectionNameArr[0],
			Level:          int32(childLevel),
		}

		for i := 1; i <= childLevel; i++ {
			childSection.Longname = childSection.Longname + " " + organization[i]
			childSection.Collectionname = childSection.Collectionname + "-" + collectionNameArr[i]
		}

		cs.Subsections = append(cs.Subsections, childSection)
	}

	if childLevel == (len(organization) - 1) {
		childSection.Charts = append(childSection.Charts, &ic)
	} else {
		childSection.addChart(organization, collectionNameArr, ic)
	}
}
