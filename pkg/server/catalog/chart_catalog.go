package catalog

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/cockroachdb/cockroach/pkg/util/metric"

	prometheusgo "github.com/prometheus/client_model/go"
)

type ChartDescription struct {
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
	Percentiles bool      // True only for Latency and Histogram metrics
}

const (
	KVTransactionLayer = `KV Transaction Layer`
)

var KVTransactionLayerTransactions = []string{KVTransactionLayer, `Transactions`}

func GetAllCharts() []ChartDescription {
	var charts = []ChartDescription{
		{
			Name:         "Abandoned",
			Organization: [][]string{KVTransactionLayerTransactions},
			Metrics:      []string{"txn.abandons"},
		},
		{
			Name:         "Aborts",
			Organization: [][]string{{"KV Transaction Layer", "Transactions"}},
			Metrics:      []string{"txn.aborts"},
		},
		{
			Name:         "Add Replica Count",
			Organization: [][]string{{"Replication Layer", "Replicate Queue"}},
			Metrics:      []string{"queue.replicate.addreplica"},
		},
		{
			Name:         "Ingestions",
			Organization: [][]string{{"Storage Layer", "RocksDB", "SSTables"}},
			Metrics: []string{"addsstable.copies",
				"addsstable.applications",
				"addsstable.proposals"},
		},
		{
			Name:         "Auto Retries",
			Organization: [][]string{{"KV Transaction Layer", "Transactions"}},
			Metrics:      []string{"txn.autoretries"},
		},
		{
			Name:         "Capacity",
			Organization: [][]string{{"Storage Layer", "Storage", "Overview"}},
			Metrics: []string{"capacity.available",
				"capacity",
				"capacity.reserved",
				"capacity.used"},
		},
		{
			Name:         "Keys/Sec Avg.",
			Organization: [][]string{{"Replication Layer", "Raft", "Overview"}},
			Metrics:      []string{"rebalancing.writespersecond"},
		},
		{
			Name: "Writes Waiting on Range Split",
			Organization: [][]string{
				{"KV Transaction Layer", "Requests", "Backpressure"},
				{"Replication Layer", "Requests", "Backpressure"},
			},
			Downsampler: "MAX",
			Aggregator:  "MAX",
			Rate:        "Normal",
			Percentiles: false,
			Metrics:     []string{"requests.backpressure.split"},
		},
		{
			Name:         "Backpressued Writes Waiting on Split",
			Organization: [][]string{{"Replication Layer", "Ranges"}},
			Metrics:      []string{"requests.backpressure.split"},
		},
		{
			Name:         "Batches",
			Organization: [][]string{{"Distribution Layer", "DistSender"}},
			Metrics: []string{"distsender.batches",
				"distsender.batches.partial"},
		},
		{
			Name:         "Timestamp",
			Organization: [][]string{{"Process", "Build Info"}},
			Downsampler:  "MAX",
			Aggregator:   "MAX",
			Rate:         "Normal",
			Percentiles:  false,
			Metrics:      []string{"build.timestamp"},
		},
		{
			Name:         "Sizes",
			Organization: [][]string{{"Storage Layer", "Storage", "Compactor"}},
			Metrics: []string{
				"compactor.suggestionbytes.compacted",
				"compactor.suggestionbytes.queued",
				"compactor.suggestionbytes.skipped",
			},
		},
		{
			Name:         "Byte I/O",
			Organization: [][]string{{"SQL Layer", "SQL"}},
			Metrics: []string{"sql.bytesin",
				"sql.bytesout"},
		},
		{
			Name:         "Bytes",
			Organization: [][]string{{"Distribution Layer", "Gossip"}},
			Metrics: []string{"gossip.bytes.received",
				"gossip.bytes.sent"},
		},
		// {
		// 	Name:         "CA Expiration",
		// 	Organization: [][]string{{"Process", "Certificates"}},
		// 	Downsampler:  "MAX",
		// 	Aggregator:   "MAX",
		// 	Rate:         "Normal",
		// 	Percentiles:  false,
		// 	Metrics:      []string{"security.certificate.expiration.ca"},
		// },
		{
			Name:         "Memory",
			Organization: [][]string{{"Process", "Server", "cgo"}},
			Metrics: []string{"sys.cgo.allocbytes",
				"sys.cgo.totalbytes"},
		},
		{
			Name:         "Calls",
			Organization: [][]string{{"Process", "Server", "cgo"}},
			Metrics:      []string{"sys.cgocalls"},
		},
		{
			Name:         "Offsets",
			Organization: [][]string{{"KV Transaction Layer", "Clocks"}},
			Metrics: []string{"clock-offset.meannanos",
				"clock-offset.stddevnanos"},
		},
		{
			Name:         "Offsets",
			Organization: [][]string{{"Process", "Clocks"}},
			Metrics: []string{"clock-offset.meannanos",
				"clock-offset.stddevnanos"},
		},
		{
			Name:         "Counts",
			Organization: [][]string{{"Replication Layer", "Replicas", "Command Queue"}},
			Metrics: []string{"replicas.commandqueue.combinedqueuesize",
				"replicas.commandqueue.combinedreadcount",
				"replicas.commandqueue.combinedwritecount"},
		},
		{
			Name:         "Commits",
			Organization: [][]string{{"KV Transaction Layer", "Transactions"}},
			Metrics: []string{"txn.commits",
				"txn.commits1PC"},
		},
		{
			Name:         "Time",
			Organization: [][]string{{"Storage Layer", "Storage", "Compactor"}},
			Metrics:      []string{"compactor.compactingnanos"},
		},
		{
			Name:         "Success",
			Organization: [][]string{{"Storage Layer", "Storage", "Compactor"}},
			Metrics: []string{"compactor.compactions.failure",
				"compactor.compactions.success"},
		},
		{
			Name:         "Connections",
			Organization: [][]string{{"Distribution Layer", "Gossip"}},
			Downsampler:  "MAX",
			Aggregator:   "AVG",
			Rate:         "Rate",
			Percentiles:  false,
			Metrics: []string{"gossip.connections.refused",
				"gossip.connections.incoming",
				"gossip.connections.outgoing"},
		},
		{
			Name:         "Connections",
			Organization: [][]string{{"SQL Layer", "SQL"}},
			Metrics:      []string{"sql.conns"},
		},
		{
			Name:         "Count",
			Organization: [][]string{{"Replication Layer", "Consistency Checker Queue"}},
			Metrics: []string{"queue.consistency.process.failure",
				"queue.consistency.pending",
				"queue.consistency.process.success"},
		},
		{
			Name:         "Time Spent",
			Organization: [][]string{{"Replication Layer", "Consistency Checker Queue"}},
			Metrics:      []string{"queue.consistency.processingnanos"},
		},
		{
			Name:         "Time",
			Organization: [][]string{{"Process", "CPU"}},
			Metrics: []string{"sys.cpu.sys.ns",
				"sys.cpu.user.ns"},
		},
		{
			Name:         "Percentage",
			Organization: [][]string{{"Process", "CPU"}},
			Metrics: []string{"sys.cpu.sys.percent",
				"sys.cpu.user.percent"},
		},
		{
			Name:         "Current Memory Usage",
			Organization: [][]string{{"SQL Layer", "DistSQL"}},
			Metrics:      []string{"sql.mem.distsql.current"},
		},
		{
			Name:         "Current",
			Organization: [][]string{{"SQL Layer", "SQL Memory", "Admin"}},
			Metrics:      []string{"sql.mem.admin.current"},
		},
		{
			Name:         "Current",
			Organization: [][]string{{"SQL Layer", "SQL Memory", "Internal"}},
			Metrics:      []string{"sql.mem.internal.current"},
		},
		{
			Name:         "Current",
			Organization: [][]string{{"SQL Layer", "SQL Memory", "Connections"}},
			Metrics:      []string{"sql.mem.conns.current"},
		},
		{
			Name:         "DDL Count",
			Organization: [][]string{{"SQL Layer", "SQL"}},
			Metrics: []string{"sql.ddl.count",
				"sql.query.count"},
		},
		{
			Name:         "DML Mix",
			Organization: [][]string{{"SQL Layer", "SQL"}},
			Metrics: []string{"sql.delete.count",
				"sql.insert.count",
				"sql.misc.count",
				"sql.query.count",
				"sql.select.count",
				"sql.update.count"},
		},
		{
			Name:         "Exec Latency",
			Organization: [][]string{{"SQL Layer", "DistSQL"}},
			Metrics:      []string{"sql.distsql.exec.latency"},
		},
		{
			Name:         "DML Mix",
			Organization: [][]string{{"SQL Layer", "DistSQL"}},
			Metrics:      []string{"sql.distsql.select.count"},
		},
		{
			Name:         "Service Latency",
			Organization: [][]string{{"SQL Layer", "DistSQL"}},
			Metrics:      []string{"sql.distsql.service.latency"},
		},
		{
			Name:         "Durations",
			Organization: [][]string{{"KV Transaction Layer", "Transactions"}},
			Metrics:      []string{"txn.durations"},
		},
		{
			Name:         "Epoch Increment Count",
			Organization: [][]string{{"Replication Layer", "Node Liveness"}},
			Metrics:      []string{"liveness.epochincrements"},
		},
		{
			Name:         "Success",
			Organization: [][]string{{"KV Transaction Layer", "Requests", "Overview"}},
			Downsampler:  "MAX",
			Aggregator:   "AVG",
			Rate:         "Rate",
			Percentiles:  false,
			Metrics: []string{"exec.error",
				"exec.success"},
		},
		{
			Name:         "File Descriptors (FD)",
			Organization: [][]string{{"Process", "Server", "Overview"}},
			Metrics: []string{"sys.fd.open",
				"sys.fd.softlimit"},
		},
		{
			Name:         "Active Flows",
			Organization: [][]string{{"SQL Layer", "DistSQL"}},
			Metrics:      []string{"sql.distsql.flows.active"},
		},
		{
			Name:         "Total Flows",
			Organization: [][]string{{"SQL Layer", "DistSQL"}},
			Metrics:      []string{"sql.distsql.flows.total"},
		},
		{
			Name:         "AbortSpan",
			Organization: [][]string{{"KV Transaction Layer", "Garbage Collection (GC)", "Keys"}},
			Metrics: []string{"queue.gc.info.abortspanconsidered",
				"queue.gc.info.abortspangcnum",
				"queue.gc.info.abortspanscanned"},
		},
		{
			Name:         "Cumultative Age of Non-Live Data",
			Organization: [][]string{{"KV Transaction Layer", "Storage"}},
			Metrics:      []string{"gcbytesage"},
		},
		{
			Name:         "Cumultative Age of Non-Live Data",
			Organization: [][]string{{"Storage Layer", "Storage", "KV"}},
			Metrics:      []string{"gcbytesage"},
		},
		{
			Name:         "Total GC Runs",
			Organization: [][]string{{"KV Transaction Layer", "Garbage Collection (GC)", "Overview"}},
			Metrics:      []string{"sys.gc.count"},
		},
		{
			Name:         "Old Intents",
			Organization: [][]string{{"KV Transaction Layer", "Garbage Collection (GC)", "Keys"}},
			Metrics:      []string{"queue.gc.info.intentsconsidered"},
		},
		{
			Name:         "Distinct Txns",
			Organization: [][]string{{"KV Transaction Layer", "Garbage Collection (GC)", "Keys"}},
			Metrics:      []string{"queue.gc.info.intenttxns"},
		},
		{
			Name:         "Keys with GC'able Data",
			Organization: [][]string{{"KV Transaction Layer", "Garbage Collection (GC)", "Keys"}},
			Metrics:      []string{"queue.gc.info.numkeysaffected"},
		},
		{
			Name:         "Total GC Pause (NS)",
			Organization: [][]string{{"KV Transaction Layer", "Garbage Collection (GC)", "Overview"}},
			Metrics:      []string{"sys.gc.pause.ns"},
		},
		{
			Name:         "Current GC Pause Percent",
			Organization: [][]string{{"KV Transaction Layer", "Garbage Collection (GC)", "Overview"}},
			Metrics:      []string{"sys.gc.pause.percent"},
		},
		{
			Name:         "Pushes",
			Organization: [][]string{{"KV Transaction Layer", "Garbage Collection (GC)", "Keys"}},
			Metrics:      []string{"queue.gc.info.pushtxn"},
		},
		{
			Name: "Queue Success",
			Organization: [][]string{{"Replication Layer", "Garbage Collection"},
				{
					"Storage Layer", "Garbage Collection"}},
			Metrics: []string{"queue.gc.process.failure",
				"queue.gc.pending",
				"queue.gc.process.success"},
		},
		{
			Name: "Queue Time",
			Organization: [][]string{{"Replication Layer", "Garbage Collection"},
				{
					"Storage Layer", "Garbage Collection"}},
			Metrics: []string{"queue.gc.processingnanos"},
		},
		{
			Name:         "Intents",
			Organization: [][]string{{"KV Transaction Layer", "Garbage Collection (GC)", "Keys"}},
			Metrics: []string{"queue.gc.info.resolvesuccess",
				"queue.gc.info.resolvetotal"},
		},
		{
			Name:         "Txn Relationship",
			Organization: [][]string{{"KV Transaction Layer", "Garbage Collection (GC)", "Keys"}},
			Metrics: []string{"queue.gc.info.transactionspangcaborted",
				"queue.gc.info.transactionspangccommitted",
				"queue.gc.info.transactionspangcpending"},
		},
		{
			Name:         "Enteries in Txn Spans",
			Organization: [][]string{{"KV Transaction Layer", "Garbage Collection (GC)", "Keys"}},
			Metrics:      []string{"queue.gc.info.transactionspanscanned"},
		},
		{
			Name:         "Memory",
			Organization: [][]string{{"Process", "Server", "go"}},
			Metrics: []string{"sys.go.allocbytes",
				"sys.go.totalbytes"},
		},
		{
			Name:         "goroutines",
			Organization: [][]string{{"Process", "Server", "go"}},
			Metrics:      []string{"sys.goroutines"},
		},
		{
			Name:         "Heartbeats Success",
			Organization: [][]string{{"Replication Layer", "Node Liveness"}},
			Metrics: []string{"liveness.heartbeatfailures",
				"liveness.heartbeatsuccesses"},
		},
		{
			Name:         "Heartbeat Latency",
			Organization: [][]string{{"Replication Layer", "Node Liveness"}},
			Metrics:      []string{"liveness.heartbeatlatency"},
		},
		{
			Name:         "Infos",
			Organization: [][]string{{"Distribution Layer", "Gossip"}},
			Metrics: []string{"gossip.infos.received",
				"gossip.infos.sent"},
		},
		{
			Name:         "Cumultative Intent Age",
			Organization: [][]string{{"KV Transaction Layer", "Storage"}},
			Metrics:      []string{"intentage"},
		},
		{
			Name:         "Cumultative Intent Age",
			Organization: [][]string{{"Storage Layer", "Storage", "KV"}},
			Metrics:      []string{"intentage"},
		},
		{
			Name:         "Size",
			Organization: [][]string{{"KV Transaction Layer", "Storage"}},
			Metrics: []string{"intentbytes",
				"keybytes",
				"livebytes",
				"sysbytes",
				"totalbytes",
				"valbytes"},
		},
		{
			Name:         "Size",
			Organization: [][]string{{"Storage Layer", "Storage", "KV"}},
			Metrics: []string{"intentbytes",
				"keybytes",
				"livebytes",
				"sysbytes",
				"totalbytes",
				"valbytes"},
		},
		{
			Name:         "Counts",
			Organization: [][]string{{"KV Transaction Layer", "Storage"}},
			AxisLabel:    "MVCC Keys & Values",
			Metrics: []string{"intentcount",
				"keycount",
				"livecount",
				"syscount",
				"valcount"},
		},
		{
			Name:         "Counts",
			Organization: [][]string{{"Storage Layer", "Storage", "KV"}},
			AxisLabel:    "MVCC Keys & Values",
			Metrics: []string{"intentcount",
				"keycount",
				"livecount",
				"syscount",
				"valcount"},
		},
		{
			Name:         "Metric Update Frequency",
			Organization: [][]string{{"KV Transaction Layer", "Storage"}},
			Downsampler:  "AVG",
			Aggregator:   "AVG",
			Rate:         "Rate",
			Percentiles:  false,
			Metrics:      []string{"lastupdatenanos"},
		},
		{
			Name:         "Metric Update Frequency",
			Organization: [][]string{{"Storage Layer", "Storage", "KV"}},
			Metrics:      []string{"lastupdatenanos"},
		},
		{
			Name:         "Latency",
			Organization: [][]string{{"KV Transaction Layer", "Requests", "Overview"}},
			Metrics:      []string{"exec.latency"},
		},
		{
			Name:         "Roundtrip Latency",
			Organization: [][]string{{"KV Transaction Layer", "Clocks"}},
			Metrics:      []string{"round-trip-latency"},
		},
		{
			Name:         "Roundtrip Latency",
			Organization: [][]string{{"Process", "Clocks"}},
			Metrics:      []string{"round-trip-latency"},
		},
		{
			Name:         "Total",
			Organization: [][]string{{"Replication Layer", "Leases"}},
			Metrics: []string{"leases.epoch",
				"leases.expiration",
				"replicas.leaseholders",
				"replicas.leaders_not_leaseholders"},
		},
		{
			Name:         "Leaseholders",
			Organization: [][]string{{"Replication Layer", "Replicas", "Overview"}},
			Metrics:      []string{"replicas.leaseholders"},
		},
		{
			Name:         "Succcess Rate",
			Organization: [][]string{{"Replication Layer", "Leases"}},
			Metrics: []string{"leases.error",
				"leases.success"},
		},
		{
			Name:         "Transfer Success Rate",
			Organization: [][]string{{"Replication Layer", "Leases"}},
			Metrics: []string{"leases.transfers.error",
				"leases.transfers.success"},
		},
		{
			Name:         "Node Count",
			Organization: [][]string{{"Replication Layer", "Node Liveness"}},
			Downsampler:  "MAX",
			Aggregator:   "MAX",
			Rate:         "Normal",
			Percentiles:  false,
			Metrics:      []string{"liveness.livenodes"},
		},
		{
			Name:         "RPCs",
			Organization: [][]string{{"Distribution Layer", "DistSender"}},
			Metrics: []string{"distsender.rpc.sent.local",
				"distsender.rpc.sent"},
		},
		{
			Name:         "Memory Usage per Statement",
			Organization: [][]string{{"SQL Layer", "DistSQL"}},
			Metrics:      []string{"sql.mem.distsql.max"},
		},
		{
			Name:         "All",
			Organization: [][]string{{"SQL Layer", "SQL Memory", "Admin"}},
			Metrics:      []string{"sql.mem.admin.max"},
		},
		{
			Name:         "All",
			Organization: [][]string{{"SQL Layer", "SQL Memory", "Internal"}},
			Metrics:      []string{"sql.mem.internal.max"},
		},
		{
			Name:         "All",
			Organization: [][]string{{"SQL Layer", "SQL Memory", "Connections"}},
			Metrics:      []string{"sql.mem.conns.max"},
		},
		{
			Name:         "Command Maxes",
			Organization: [][]string{{"Replication Layer", "Replicas", "Command Queue"}},
			Metrics: []string{
				"replicas.commandqueue.maxoverlaps",
				"replicas.commandqueue.maxreadcount",
				"replicas.commandqueue.maxsize",
				"replicas.commandqueue.maxwritecount",
			},
		},
		{
			Name:         "Tree Size Max",
			Organization: [][]string{{"Replication Layer", "Replicas", "Command Queue"}},
			Metrics:      []string{"replicas.commandqueue.maxtreesize"},
		},
		{
			Name:         "Errors",
			Organization: [][]string{{"Distribution Layer", "DistSender"}},
			Metrics: []string{
				"distsender.rpc.sent.nextreplicaerror",
				"distsender.errors.notleaseholder",
				"requests.slow.distsender",
			},
		},
		// {
		// 	Name:         "Node Cert Expiration",
		// 	Organization: [][]string{{"Process", "Certificates"}},
		// 	Downsampler:  "MAX",
		// 	Aggregator:   "MAX",
		// 	Rate:         "Normal",
		// 	Percentiles:  false,
		// 	Metrics:      []string{"security.certificate.expiration.node"},
		// },
		{
			Name:         "ID",
			Organization: [][]string{{"Process", "Node"}},
			Downsampler:  "MAX",
			Aggregator:   "MAX",
			Rate:         "Normal",
			Percentiles:  false,
			Metrics:      []string{"node-id"},
		},
		{
			Name:         "Page Rotations",
			Organization: [][]string{{"KV Transaction Layer", "Timestamp Cache"}},
			Metrics: []string{"tscache.skl.read.rotations",
				"tscache.skl.write.rotations"},
		},
		{
			Name:         "Page Counts",
			Organization: [][]string{{"KV Transaction Layer", "Timestamp Cache"}},
			Metrics: []string{"tscache.skl.read.pages",
				"tscache.skl.write.pages"},
		},
		{
			Name:         "Active Queries",
			Organization: [][]string{{"SQL Layer", "DistSQL"}},
			Metrics:      []string{"sql.distsql.queries.active"},
		},
		{
			Name:         "Total Queries",
			Organization: [][]string{{"SQL Layer", "DistSQL"}},
			Metrics:      []string{"sql.distsql.queries.total"},
		},
		{
			Name:         "Count",
			Organization: [][]string{{"Replication Layer", "Replicas", "Overview"}},
			Metrics: []string{"replicas.quiescent",
				"replicas",
				"replicas.reserved"},
		},
		{
			Name:         "Pending",
			Organization: [][]string{{"Replication Layer", "Raft", "Heartbeats"}},
			Metrics:      []string{"raft.heartbeats.pending"},
		},
		{
			Name:         "Command Commit",
			Organization: [][]string{{"Replication Layer", "Raft", "Latency"}},
			Metrics:      []string{"raft.process.commandcommit.latency"},
		},
		{
			Name:         "Commands Count",
			Organization: [][]string{{"Replication Layer", "Raft", "Overview"}},
			Metrics:      []string{"raft.commandsapplied"},
		},
		{
			Name:         "Enqueued",
			Organization: [][]string{{"Replication Layer", "Raft", "Overview"}},
			Metrics:      []string{"raft.enqueued.pending"},
		},
		{
			Name:         "Leaders",
			Organization: [][]string{{"Replication Layer", "Raft", "Overview"}},
			Metrics:      []string{"replicas.leaders"},
		},
		{
			Name:         "Log Commit",
			Organization: [][]string{{"Replication Layer", "Raft", "Latency"}},
			Metrics:      []string{"raft.process.logcommit.latency"},
		},
		{
			Name:         "Followers Behind By...",
			Organization: [][]string{{"Replication Layer", "Raft", "Log"}},
			Metrics:      []string{"raftlog.behind"},
		},
		{
			Name:         "Log Status",
			Organization: [][]string{{"Replication Layer", "Raft", "Queues"}},
			Metrics: []string{"queue.raftlog.process.failure",
				"queue.raftlog.pending",
				"queue.raftlog.process.success"},
		},
		{
			Name:         "Log Processing Time Spent",
			Organization: [][]string{{"Replication Layer", "Raft", "Queues"}},
			Metrics:      []string{"queue.raftlog.processingnanos"},
		},
		{
			Name:         "Entries Truncated",
			Organization: [][]string{{"Replication Layer", "Raft", "Log"}},
			Metrics:      []string{"raftlog.truncated"},
		},
		{
			Name:         "MsgApp Count",
			Organization: [][]string{{"Replication Layer", "Raft", "Received"}},
			Metrics:      []string{"raft.rcvd.app"},
		},
		{
			Name:         "MsgAppResp Count",
			Organization: [][]string{{"Replication Layer", "Raft", "Received"}},
			Metrics:      []string{"raft.rcvd.appresp"},
		},
		{
			Name:         "Dropped",
			Organization: [][]string{{"Replication Layer", "Raft", "Received"}},
			Metrics:      []string{"raft.rcvd.dropped"},
		},
		{
			Name:         "Heartbeat Count",
			Organization: [][]string{{"Replication Layer", "Raft", "Received"}},
			Metrics:      []string{"raft.rcvd.heartbeat"},
		},
		{
			Name:         "MsgHeartbeatResp Count",
			Organization: [][]string{{"Replication Layer", "Raft", "Received"}},
			Metrics:      []string{"raft.rcvd.heartbeatresp"},
		},
		{
			Name:         "MsgHeartbeatResp Count",
			Organization: [][]string{{"Replication Layer", "Raft", "Heartbeats"}},
			Metrics:      []string{"raft.rcvd.heartbeatresp"},
		},
		{
			Name:         "MsgPreVote Count",
			Organization: [][]string{{"Replication Layer", "Raft", "Received"}},
			Metrics:      []string{"raft.rcvd.prevote"},
		},
		{
			Name:         "MsgPreVoteResp Count",
			Organization: [][]string{{"Replication Layer", "Raft", "Received"}},
			Metrics:      []string{"raft.rcvd.prevoteresp"},
		},
		{
			Name:         "MsgProp Count",
			Organization: [][]string{{"Replication Layer", "Raft", "Received"}},
			Metrics:      []string{"raft.rcvd.prop"},
		},
		{
			Name:         "MsgSnap Count",
			Organization: [][]string{{"Replication Layer", "Raft", "Received"}},
			Metrics:      []string{"raft.rcvd.snap"},
		},
		{
			Name:         "MsgTimeoutNow Count",
			Organization: [][]string{{"Replication Layer", "Raft", "Received"}},
			Metrics:      []string{"raft.rcvd.timeoutnow"},
		},
		{
			Name:         "MsgTransferLeader Count",
			Organization: [][]string{{"Replication Layer", "Raft", "Received"}},
			Metrics:      []string{"raft.rcvd.transferleader"},
		},
		{
			Name:         "MsgTransferLeader Count",
			Organization: [][]string{{"Replication Layer", "Raft", "Heartbeats"}},
			Metrics:      []string{"raft.rcvd.transferleader"},
		},
		{
			Name:         "MsgVote Count",
			Organization: [][]string{{"Replication Layer", "Raft", "Received"}},
			Metrics:      []string{"raft.rcvd.vote"},
		},
		{
			Name:         "MsgVoteResp Count",
			Organization: [][]string{{"Replication Layer", "Raft", "Received"}},
			Metrics:      []string{"raft.rcvd.voteresp"},
		},
		{
			Name:         "Snapshot Status",
			Organization: [][]string{{"Replication Layer", "Raft", "Queues"}},
			Metrics: []string{"queue.raftsnapshot.process.failure",
				"queue.raftsnapshot.pending",
				"queue.raftsnapshot.process.success"},
		},
		{
			Name:         "Snapshot Processing Time Spent",
			Organization: [][]string{{"Replication Layer", "Raft", "Queues"}},
			Metrics:      []string{"queue.raftsnapshot.processingnanos"},
		},
		{
			Name:         "Working vs. Ticking TIme",
			Organization: [][]string{{"Replication Layer", "Raft", "Overview"}},
			Metrics: []string{"raft.process.tickingnanos",
				"raft.process.workingnanos"},
		},
		{
			Name:         "Ticks Queued",
			Organization: [][]string{{"Replication Layer", "Raft", "Overview"}},
			Metrics:      []string{"raft.ticks"},
		},
		{
			Name: "Add, Split, Remove",
			Organization: [][]string{{"Distribution Layer", "Ranges"},
				{
					"Replication Layer", "Ranges"}},
			Metrics: []string{"range.adds",
				"range.removes",
				"range.splits"},
		},
		{
			Name:         "Overview",
			Organization: [][]string{{"Distribution Layer", "Ranges"}},
			Metrics:      []string{"ranges"},
		},
		{
			Name:         "Overview",
			Organization: [][]string{{"Replication Layer", "Ranges"}},
			Metrics: []string{"ranges",
				"ranges.unavailable",
				"ranges.underreplicated"},
		},
		{
			Name:         "Leader Transfers",
			Organization: [][]string{{"Replication Layer", "Raft", "Overview"}},
			Metrics:      []string{"range.raftleadertransfers"},
		},
		{
			Name:         "Raft Leader Transfers",
			Organization: [][]string{{"Replication Layer", "Ranges"}},
			Metrics:      []string{"range.raftleadertransfers"},
		},
		{
			Name: "Snapshots",
			Organization: [][]string{{"Distribution Layer", "Ranges"},
				{"Replication Layer", "Ranges"}},
			Metrics: []string{"range.snapshots.generated",
				"range.snapshots.normal-applied",
				"range.snapshots.preemptive-applied"},
		},
		{
			Name:         "Success",
			Organization: [][]string{{"Storage Layer", "RocksDB", "Block Cache"}},
			Metrics: []string{"rocksdb.block.cache.hits",
				"rocksdb.block.cache.misses"},
		},
		{
			Name:         "Size",
			Organization: [][]string{{"Storage Layer", "RocksDB", "Block Cache"}},
			Metrics: []string{"rocksdb.block.cache.pinned-usage",
				"rocksdb.block.cache.usage"},
		},
		{
			Name:         "Bloom Filter",
			Organization: [][]string{{"Storage Layer", "RocksDB", "Overview"}},
			Metrics: []string{"rocksdb.bloom.filter.prefix.checked",
				"rocksdb.bloom.filter.prefix.useful"},
		},
		{
			Name:         "Compactions",
			Organization: [][]string{{"Storage Layer", "RocksDB", "Overview"}},
			Metrics:      []string{"rocksdb.compactions"},
		},
		{
			Name:         "Flushes",
			Organization: [][]string{{"Storage Layer", "RocksDB", "Overview"}},
			Metrics:      []string{"rocksdb.flushes"},
		},
		{
			Name:         "Memtable",
			Organization: [][]string{{"Storage Layer", "RocksDB", "Overview"}},
			Metrics:      []string{"rocksdb.memtable.total-size"},
		},
		{
			Name:         "Count",
			Organization: [][]string{{"Storage Layer", "RocksDB", "SSTables"}},
			Metrics:      []string{"rocksdb.num-sstables"},
		},
		{
			Name:         "Read Amplification",
			Organization: [][]string{{"Storage Layer", "RocksDB", "Overview"}},
			Metrics:      []string{"rocksdb.read-amplification"},
		},
		{
			Name:         "Index & Filter Block Size",
			Organization: [][]string{{"Storage Layer", "RocksDB", "Overview"}},
			Metrics:      []string{"rocksdb.table-readers-mem-estimate"},
		},
		{
			Name:         "Reblance Count",
			Organization: [][]string{{"Replication Layer", "Replicate Queue"}},
			Metrics:      []string{"queue.replicate.rebalancereplica"},
		},
		{
			Name:         "Remove Replica Count",
			Organization: [][]string{{"Replication Layer", "Replicate Queue"}},
			Metrics: []string{"queue.replicate.removedeadreplica",
				"queue.replicate.removereplica"},
		},
		{
			Name:         "Removal Count",
			Organization: [][]string{{"Replication Layer", "Replica GC Queue"}},
			Metrics:      []string{"queue.replicagc.removereplica"},
		},
		{
			Name:         "Count",
			Organization: [][]string{{"Replication Layer", "Replica GC Queue"}},
			Metrics: []string{"queue.replicagc.process.failure",
				"queue.replicagc.pending",
				"queue.replicagc.process.success"},
		},
		{
			Name:         "Time Spent",
			Organization: [][]string{{"Replication Layer", "Replica GC Queue"}},
			Metrics:      []string{"queue.replicagc.processingnanos"},
		},
		{
			Name:         "Count",
			Organization: [][]string{{"Replication Layer", "Replicate Queue"}},
			Metrics: []string{"queue.replicate.process.failure",
				"queue.replicate.pending",
				"queue.replicate.purgatory",
				"queue.replicate.process.success"},
		},
		{
			Name:         "Time Spent",
			Organization: [][]string{{"Replication Layer", "Replicate Queue"}},
			Metrics:      []string{"queue.replicate.processingnanos"},
		},
		{
			Name:         "Restarts",
			Organization: [][]string{{"KV Transaction Layer", "Transactions"}},
			Downsampler:  "MAX",
			Aggregator:   "MAX",
			Rate:         "Normal",
			Percentiles:  true,
			Metrics:      []string{"txn.restarts"},
		},
		{
			Name:         "Restart Cause Mix",
			Organization: [][]string{{"KV Transaction Layer", "Transactions"}},
			Metrics: []string{"txn.restarts.deleterange",
				"txn.restarts.possiblereplay",
				"txn.restarts.serializable",
				"txn.restarts.writetooold"},
		},
		{
			Name:         "RSS",
			Organization: [][]string{{"Process", "Server", "Overview"}},
			Metrics:      []string{"sys.rss"},
		},
		{
			Name:         "Session Current",
			Organization: [][]string{{"SQL Layer", "SQL Memory", "Admin"}},
			Metrics:      []string{"sql.mem.admin.session.current"},
		},
		{
			Name:         "Session Current",
			Organization: [][]string{{"SQL Layer", "SQL Memory", "Internal"}},
			Metrics:      []string{"sql.mem.internal.session.current"},
		},
		{
			Name:         "Session Current",
			Organization: [][]string{{"SQL Layer", "SQL Memory", "Connections"}},
			Metrics:      []string{"sql.mem.conns.session.current"},
		},
		{
			Name:         "Session All",
			Organization: [][]string{{"SQL Layer", "SQL Memory", "Admin"}},
			Metrics:      []string{"sql.mem.admin.session.max"},
		},
		{
			Name:         "Session All",
			Organization: [][]string{{"SQL Layer", "SQL Memory", "Internal"}},
			Metrics:      []string{"sql.mem.internal.session.max"},
		},
		{
			Name:         "Session All",
			Organization: [][]string{{"SQL Layer", "SQL Memory", "Connections"}},
			Metrics:      []string{"sql.mem.conns.session.max"},
		},
		{
			Name: "Stuck in Command Queue",
			Organization: [][]string{{"KV Transaction Layer", "Requests", "Slow"},
				{
					"Replication Layer", "Requests", "Slow"}},
			Downsampler: "MAX",
			Aggregator:  "MAX",
			Rate:        "Normal",
			Percentiles: false,
			Metrics:     []string{"requests.slow.commandqueue"},
		},
		{
			Name: "Stuck Acquiring Lease",
			Organization: [][]string{{"KV Transaction Layer", "Requests", "Slow"},
				{
					"Replication Layer", "Requests", "Slow"}},
			Downsampler: "MAX",
			Aggregator:  "MAX",
			Rate:        "Normal",
			Percentiles: false,
			Metrics:     []string{"requests.slow.commandqueue"},
		},
		{
			Name:         "Stuck Request Count",
			Organization: [][]string{{"Replication Layer", "Replicas", "Command Queue"}},
			Metrics:      []string{"requests.slow.commandqueue"},
		},
		{
			Name:         "Stuck Acquisition Count",
			Organization: [][]string{{"Replication Layer", "Leases"}},
			Metrics:      []string{"requests.slow.lease"},
		},
		{
			Name: "Stuck in Raft",
			Organization: [][]string{{"KV Transaction Layer", "Requests", "Slow"},
				{
					"Replication Layer", "Requests", "Slow"}},
			Downsampler: "MAX",
			Aggregator:  "MAX",
			Rate:        "Normal",
			Percentiles: false,
			Metrics:     []string{"requests.slow.raft"},
		},
		{
			Name:         "Stuck Request Count",
			Organization: [][]string{{"Replication Layer", "Raft", "Overview"}},
			Metrics:      []string{"requests.slow.raft"},
		},
		{
			Name: "Count",
			Organization: [][]string{{"Distribution Layer", "Split Queue"},
				{
					"Replication Layer", "Split Queue"}},
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
			Organization: [][]string{{"Distribution Layer", "Split Queue"},
				{
					"Replication Layer", "Split Queue"}},
			Metrics: []string{"queue.split.processingnanos"},
		},
		{
			Name:         "Exec Latency",
			Organization: [][]string{{"SQL Layer", "SQL"}},
			Metrics:      []string{"sql.exec.latency"},
		},
		{
			Name:         "Service Latency",
			Organization: [][]string{{"SQL Layer", "SQL"}},
			Metrics:      []string{"sql.service.latency"},
		},
		{
			Name:         "Count",
			Organization: [][]string{{"Timeseries", "Maintenance Queue"}},
			Metrics: []string{"queue.tsmaintenance.process.success",
				"queue.tsmaintenance.pending",
				"queue.tsmaintenance.process.failure"},
		},
		{
			Name:         "Time Spent",
			Organization: [][]string{{"Timeseries", "Maintenance Queue"}},
			Metrics:      []string{"queue.tsmaintenance.processingnanos"},
		},
		{
			Name:         "Lease Transfer Count",
			Organization: [][]string{{"Replication Layer", "Replicate Queue"}},
			Metrics:      []string{"queue.replicate.transferlease"},
		},
		{
			Name:         "Transaction Control Mix",
			Organization: [][]string{{"SQL Layer", "SQL"}},
			Metrics: []string{"sql.txn.abort.count",
				"sql.txn.begin.count",
				"sql.txn.commit.count",
				"sql.txn.rollback.count"},
		},
		{
			Name:         "Txn Current",
			Organization: [][]string{{"SQL Layer", "SQL Memory", "Admin"}},
			Metrics:      []string{"sql.mem.admin.txn.current"},
		},
		{
			Name:         "Txn Current",
			Organization: [][]string{{"SQL Layer", "SQL Memory", "Internal"}},
			Metrics:      []string{"sql.mem.internal.txn.current"},
		},
		{
			Name:         "Txn Current",
			Organization: [][]string{{"SQL Layer", "SQL Memory", "Connections"}},
			Metrics:      []string{"sql.mem.conns.txn.current"},
		},
		{
			Name:         "Txn All",
			Organization: [][]string{{"SQL Layer", "SQL Memory", "Admin"}},
			Metrics:      []string{"sql.mem.admin.txn.max"},
		},
		{
			Name:         "Txn All",
			Organization: [][]string{{"SQL Layer", "SQL Memory", "Internal"}},
			Metrics:      []string{"sql.mem.internal.txn.max"},
		},
		{
			Name:         "Txn All",
			Organization: [][]string{{"SQL Layer", "SQL Memory", "Connections"}},
			Metrics:      []string{"sql.mem.conns.txn.max"},
		},
		{
			Name:         "Uptime",
			Organization: [][]string{{"Process", "Server", "Overview"}},
			Metrics:      []string{"sys.uptime"},
		},
		{
			Name:         "Size",
			Organization: [][]string{{"Timeseries", "Overview"}},
			Metrics:      []string{"timeseries.write.bytes"},
		},
		{
			Name:         "Error Count",
			Organization: [][]string{{"Timeseries", "Overview"}},
			Metrics:      []string{"timeseries.write.errors"},
		},
		{
			Name:         "Count",
			Organization: [][]string{{"Timeseries", "Overview"}},
			Metrics:      []string{"timeseries.write.samples"}},
	}

	return charts
}

func GenerateCatalog(metadata map[string]metric.Metadata) []*ChartSection {

	catalog := make([]*ChartSection, 7)

	catalogKey := map[string]int{
		"Process":              0,
		"SQL Layer":            1,
		"KV Transaction Layer": 2,
		"Distribution Layer":   3,
		"Replication Layer":    4,
		"Storage Layer":        5,
		"Timeseries":           6,
	}

	catalogKey["process"] = 0

	catalog[0] = &ChartSection{
		Name:           "Process",
		Longname:       "process",
		Collectionname: "process-all",
		Description: `These charts detail the overall performance of the <code>cockroach</code> 
		process running on this server.`,
		Level: 0,
	}
	catalog[1] = &ChartSection{
		Name:           "SQL Layer",
		Longname:       "SQL Layer",
		Collectionname: "sql-layer-all",
		Description: `In the SQL layer, nodes receive commands and then parse, plan, and 
		execute them. <br/><br/><a class="catalog-link" href="https://www.cockroachlabs.com
		/docs/stable/architecture/sql-layer.html">SQL Layer Architecture Docs >></a>"`,
		Level: 0,
	}
	catalog[2] = &ChartSection{
		Name:           "KV Transaction Layer",
		Longname:       "KV Transaction Layer",
		Collectionname: "kv-transaction-layer-all",
		Description: `The KV Transaction Layer coordinates concurrent requests as key-value 
		operations. To maintain consistency, this is also where the cluster manages time. <br/>
		<br/><a class="catalog-link" href="https://www.cockroachlabs.com/docs/stable/architecture
		/transaction-layer.html">Transaction Layer Architecture Docs >></a>`,
		Level: 0,
	}
	catalog[3] = &ChartSection{
		Name:           "Distribution Layer",
		Longname:       "Distribution Layer",
		Collectionname: "distribution-layer-all",
		Description: `The Distribution Layer provides a unified view of your cluster’s data, 
		which are actually broken up into many key-value ranges. <br/><br/><a class="catalog-link" 
		href="https://www.cockroachlabs.com/docs/stable/architecture/distribution-layer.html"> 
		Distribution Layer Architecture Docs >></a>`,
		Level: 0,
	}
	catalog[4] = &ChartSection{
		Name:           "Replication Layer",
		Longname:       "Replication Layer",
		Collectionname: "replication-layer-all",
		Description: `The Replication Layer maintains consistency between copies of ranges (known 
			as replicas) through our consensus algorithm, Raft. <br/><br/><a class="catalog-link" 
			href="https://www.cockroachlabs.com/docs/stable/architecture/replication-layer.html"> 
			Replication Layer Architecture Docs >></a>`,
		Level: 0,
	}
	catalog[5] = &ChartSection{
		Name:           "Storage Layer",
		Longname:       "Storage Layer",
		Collectionname: "replication-layer-all",
		Description: `The Storage Layer reads and writes data to disk, as well as manages garbage 
		collection. <br/><br/><a class="catalog-link" href="https://www.cockroachlabs.com/docs/stable
		/architecture/storage-layer.html">Storage Layer Architecture Docs >></a>`,
		Level: 0,
	}
	catalog[6] = &ChartSection{
		Name:           "Timeseries",
		Longname:       "Timeseries",
		Collectionname: "timeseries-all",
		Description: `Your cluster collects data about its own performance, which is used to power the 
		very charts you\'re using, among other things.`,
		Level: 0,
	}

	type chartDefaults struct {
		Downsampler string // AVG by default
		Aggregator  string // AVG by default
		Rate        string // Default depends on type
		Percentiles bool   // True only for Latency and Histogram metrics
	}

	defaultsByDisplayMethod := make(map[prometheusgo.MetricType]chartDefaults)

	defaultsByDisplayMethod[prometheusgo.MetricType_COUNTER] = chartDefaults{
		Downsampler: "AVG",
		Aggregator:  "AVG",
		Rate:        "Non-negative rate",
		Percentiles: false,
	}

	defaultsByDisplayMethod[prometheusgo.MetricType_GAUGE] = chartDefaults{
		Downsampler: "AVG",
		Aggregator:  "AVG",
		Rate:        "Normal",
		Percentiles: false,
	}

	defaultsByDisplayMethod[prometheusgo.MetricType_HISTOGRAM] = chartDefaults{
		Downsampler: "AVG",
		Aggregator:  "AVG",
		Rate:        "Normal",
		Percentiles: true,
	}

	var MetadataUnitsToAxisUnits = map[metric.DisplayUnit]AxisUnits{
		metric.DisplayUnit_Bytes:       AxisUnits_Bytes,
		metric.DisplayUnit_Const:       AxisUnits_Count,
		metric.DisplayUnit_Count:       AxisUnits_Count,
		metric.DisplayUnit_Nanoseconds: AxisUnits_Duration,
		metric.DisplayUnit_Percent:     AxisUnits_Count,
		metric.DisplayUnit_Timestamp:   AxisUnits_Duration,
	}

	var charts = GetAllCharts()

	for _, v := range charts {

		for _, orgLevel := range v.Organization {

			numberOfLevels := len(orgLevel)
			taxonomySlugs := make([]string, numberOfLevels)
			var thisCollectionName string
			var thisLongName string
			makeDashes := regexp.MustCompile("( )|/|,")
			for k, n := range orgLevel {
				taxonomySlugs[k] = makeDashes.ReplaceAllString(strings.ToLower(n), "-")
				thisCollectionName = thisCollectionName + taxonomySlugs[k] + "-"
				thisLongName = thisLongName + n + " | "
			}
			thisCollectionName += makeDashes.ReplaceAllString(strings.ToLower(v.Name), "-")
			thisLongName += v.Name

			w, ok := catalogKey[orgLevel[0]]

			if !ok {
				log.Fatal("Trying to put something where it can't exist")
			}

			var chartMetrics []*ChartMetric
			var metricType prometheusgo.MetricType

			for _, x := range v.Metrics {

				md, ok := metadata[x]

				// If metric is missing from metadata, don't add it to this chart
				// because we won't be able to calculate it
				if !ok {
					fmt.Printf("Trying to use metric %v, but it doesn't exist\n", x)
				}

				var thisMetric = &ChartMetric{
					Name:           md.Name,
					Help:           md.Help,
					AxisLabel:      md.Unit,
					PreferredUnits: MetadataUnitsToAxisUnits[md.DisplayUnit],
				}

				metricType = md.MetricType

				chartMetrics = append(chartMetrics, thisMetric)

				if chartMetrics[0].PreferredUnits != thisMetric.PreferredUnits {
					fmt.Printf("Charts should have the same units: %v has %v; %v has %v\n", chartMetrics[0].Name, chartMetrics[0].PreferredUnits, thisMetric.Name, thisMetric.PreferredUnits)
				}
				if chartMetrics[0].AxisLabel != thisMetric.AxisLabel {
					fmt.Printf("Charts must have the same axislabel: %v has %v; %v has %v\n", chartMetrics[0].Name, chartMetrics[0].AxisLabel, thisMetric.Name, thisMetric.AxisLabel)
				}
			}

			// If the chart has no metrics, don't add it
			if len(chartMetrics) == 0 {
				continue
			}

			thisChartDefaults := defaultsByDisplayMethod[metricType]

			if v.Downsampler == "" {
				v.Downsampler = thisChartDefaults.Downsampler
			}
			if v.Aggregator == "" {
				v.Aggregator = thisChartDefaults.Aggregator
			}
			if v.Rate == "" {
				v.Rate = thisChartDefaults.Rate
			}
			if v.Percentiles == false {
				v.Percentiles = thisChartDefaults.Percentiles
			}

			if v.Units == AxisUnits_Unset {
				v.Units = chartMetrics[0].PreferredUnits
			}

			if v.AxisLabel == "" {
				v.AxisLabel = chartMetrics[0].AxisLabel
			}

			thisChart := IndividualChart{
				Title:          v.Name,
				Longname:       thisLongName,
				Collectionname: thisCollectionName,
				Downsampler:    v.Downsampler,
				Aggregator:     v.Aggregator,
				Derivative:     v.Rate,
				Units:          v.Units,
				AxisLabel:      v.AxisLabel,
				Percentiles:    v.Percentiles,
				Data:           chartMetrics,
			}

			var found bool
			var level1 *ChartSection

			for _, x := range catalog[w].Subsections {
				if x.Name == orgLevel[1] {
					found = true
					level1 = x
				}
			}

			if !found {
				level1 = &ChartSection{
					Name:           orgLevel[1],
					Longname:       "All " + orgLevel[1],
					Collectionname: taxonomySlugs[0] + "-" + taxonomySlugs[1],
					Level:          1,
				}

				catalog[w].Subsections = append(catalog[w].Subsections, level1)
			}

			found = false

			if numberOfLevels > 2 {

				var level2 *ChartSection

				for _, x := range level1.Subsections {
					if x.Name == orgLevel[2] {
						found = true
						level2 = x
					}
				}

				if !found {
					level2 = &ChartSection{
						Name:           orgLevel[2],
						Longname:       "All " + orgLevel[1] + " " + orgLevel[2],
						Collectionname: taxonomySlugs[0] + "-" + taxonomySlugs[1] + taxonomySlugs[2],
						Level:          2,
					}

					level1.Subsections = append(level1.Subsections, level2)
				}

				level2.Charts = append(level2.Charts, &thisChart)
			} else {
				level1.Charts = append(level1.Charts, &thisChart)
			}

			// for _, x := range catalog {
			// 	fmt.Print(x)
			// }
		}
	}

	return catalog
}
