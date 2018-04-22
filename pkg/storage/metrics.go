// Copyright 2016 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package storage

import (
	"time"

	"github.com/cockroachdb/cockroach/pkg/storage/engine"
	"github.com/cockroachdb/cockroach/pkg/storage/engine/enginepb"
	"github.com/cockroachdb/cockroach/pkg/util/metric"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
	"github.com/coreos/etcd/raft/raftpb"
)

var (
	// Replica metrics.
	metaReplicaCount = metric.Metadata{
		Name:      "replicas",
		Help:      "Number of replicas",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaReservedReplicaCount = metric.Metadata{
		Name:      "replicas.reserved",
		Help:      "Number of replicas reserved for snapshots",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaRaftLeaderCount = metric.Metadata{
		Name:      "replicas.leaders",
		Help:      "Number of raft leaders",
		Units:     "Count",
		AxisLabel: "Raft Leaders"}
	metaRaftLeaderNotLeaseHolderCount = metric.Metadata{
		Name: "replicas.leaders_not_leaseholders", Units: "Count", AxisLabel: "Replicas",
		Help: "Number of replicas that are Raft leaders whose range lease is held by another store",
	}
	metaLeaseHolderCount = metric.Metadata{
		Name:      "replicas.leaseholders",
		Help:      "Number of lease holders",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaQuiescentCount = metric.Metadata{
		Name:      "replicas.quiescent",
		Help:      "Number of quiesced replicas",
		Units:     "Count",
		AxisLabel: "Replicas"}

	// Replica CommandQueue metrics. Max size metrics track the maximum value
	// seen for all replicas during a single replica scan.
	metaMaxCommandQueueSize = metric.Metadata{
		Name:      "replicas.commandqueue.maxsize",
		Help:      "Largest number of commands in any CommandQueue",
		Units:     "Count",
		AxisLabel: "Commands"}
	metaMaxCommandQueueWriteCount = metric.Metadata{
		Name:      "replicas.commandqueue.maxwritecount",
		Help:      "Largest number of read-write commands in any CommandQueue",
		Units:     "Count",
		AxisLabel: "Commands"}
	metaMaxCommandQueueReadCount = metric.Metadata{
		Name:      "replicas.commandqueue.maxreadcount",
		Help:      "Largest number of read-only commands in any CommandQueue",
		Units:     "Count",
		AxisLabel: "Commands"}
	metaMaxCommandQueueTreeSize = metric.Metadata{
		Name:      "replicas.commandqueue.maxtreesize",
		Help:      "Largest number of intervals in any CommandQueue's interval tree",
		Units:     "Count",
		AxisLabel: "Commands"}
	metaMaxCommandQueueOverlaps = metric.Metadata{
		Name:      "replicas.commandqueue.maxoverlaps",
		Help:      "Largest number of overlapping commands seen when adding to any CommandQueue",
		Units:     "Count",
		AxisLabel: "Commands"}
	metaCombinedCommandQueueSize = metric.Metadata{
		Name:      "replicas.commandqueue.combinedqueuesize",
		Help:      "Number of commands in all CommandQueues combined",
		Units:     "Count",
		AxisLabel: "Commands"}
	metaCombinedCommandWriteCount = metric.Metadata{
		Name:      "replicas.commandqueue.combinedwritecount",
		Help:      "Number of read-write commands in all CommandQueues combined",
		Units:     "Count",
		AxisLabel: "Commands"}
	metaCombinedCommandReadCount = metric.Metadata{
		Name:      "replicas.commandqueue.combinedreadcount",
		Help:      "Number of read-only commands in all CommandQueues combined",
		Units:     "Count",
		AxisLabel: "Commands"}

	// Range metrics.
	metaRangeCount = metric.Metadata{
		Name:      "ranges",
		Help:      "Number of ranges",
		Units:     "Count",
		AxisLabel: "Ranges"}
	metaUnavailableRangeCount = metric.Metadata{
		Name:      "ranges.unavailable",
		Help:      "Number of ranges with fewer live replicas than needed for quorum",
		Units:     "Count",
		AxisLabel: "Ranges"}
	metaUnderReplicatedRangeCount = metric.Metadata{
		Name:      "ranges.underreplicated",
		Help:      "Number of ranges with fewer live replicas than the replication target",
		Units:     "Count",
		AxisLabel: "Ranges"}

	// Lease request metrics.
	metaLeaseRequestSuccessCount = metric.Metadata{
		Name:      "leases.success",
		Help:      "Number of successful lease requests",
		Units:     "Count",
		AxisLabel: "Lease Requests"}
	metaLeaseRequestErrorCount = metric.Metadata{
		Name:      "leases.error",
		Help:      "Number of failed lease requests",
		Units:     "Count",
		AxisLabel: "Lease Requests"}
	metaLeaseTransferSuccessCount = metric.Metadata{
		Name:      "leases.transfers.success",
		Help:      "Number of successful lease transfers",
		Units:     "Count",
		AxisLabel: "Lease Transfers"}
	metaLeaseTransferErrorCount = metric.Metadata{
		Name:      "leases.transfers.error",
		Help:      "Number of failed lease transfers",
		Units:     "Count",
		AxisLabel: "Lease Transfers"}
	metaLeaseExpirationCount = metric.Metadata{
		Name:      "leases.expiration",
		Help:      "Number of replica leaseholders using expiration-based leases",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaLeaseEpochCount = metric.Metadata{
		Name:      "leases.epoch",
		Help:      "Number of replica leaseholders using epoch-based leases",
		Units:     "Count",
		AxisLabel: "Replicas"}

	// Storage metrics.
	metaLiveBytes = metric.Metadata{
		Name:      "livebytes",
		Help:      "Number of bytes of live data (keys plus values)",
		Units:     "Bytes",
		AxisLabel: "Size"}
	metaKeyBytes = metric.Metadata{
		Name:      "keybytes",
		Help:      "Number of bytes taken up by keys",
		Units:     "Bytes",
		AxisLabel: "Size"}
	metaValBytes = metric.Metadata{
		Name:      "valbytes",
		Help:      "Number of bytes taken up by values",
		Units:     "Bytes",
		AxisLabel: "Size"}
	metaTotalBytes = metric.Metadata{
		Name:      "totalbytes",
		Help:      "Total number of bytes taken up by keys and values including non-live data",
		Units:     "Bytes",
		AxisLabel: "Size"}
	metaIntentBytes = metric.Metadata{
		Name:      "intentbytes",
		Help:      "Number of bytes in intent KV pairs",
		Units:     "Bytes",
		AxisLabel: "Size"}
	metaLiveCount = metric.Metadata{
		Name:      "livecount",
		Help:      "Count of live keys",
		Units:     "Count",
		AxisLabel: "Keys"}
	metaKeyCount = metric.Metadata{
		Name:      "keycount",
		Help:      "Count of all keys",
		Units:     "Count",
		AxisLabel: "Keys"}
	metaValCount = metric.Metadata{
		Name:      "valcount",
		Help:      "Count of all values",
		Units:     "Count",
		AxisLabel: "Keys"}
	metaIntentCount = metric.Metadata{
		Name:      "intentcount",
		Help:      "Count of intent keys",
		Units:     "Count",
		AxisLabel: "Keys"}
	metaIntentAge = metric.Metadata{
		Name:      "intentage",
		Help:      "Cumulative age of intents in seconds",
		Units:     "Duration",
		AxisLabel: "Time"}
	metaGcBytesAge = metric.Metadata{
		Name:      "gcbytesage",
		Help:      "Cumulative age of non-live data in seconds",
		Units:     "Duration",
		AxisLabel: "Time"}
	metaLastUpdateNanos = metric.Metadata{
		Name:      "lastupdatenanos",
		Help:      "Time in nanoseconds since Unix epoch at which bytes/keys/intents metrics were last updated",
		Units:     "Duration",
		AxisLabel: "Time"}

	// Disk usage diagram (CR=Cockroach):
	//                            ---------------------------------
	// Entire hard drive:         | non-CR data | CR data | empty |
	//                            ---------------------------------
	// Metrics:
	//                "capacity": |===============================|
	//                    "used":               |=========|
	//               "available":                         |=======|
	// "usable" (computed in UI):               |=================|
	metaCapacity = metric.Metadata{
		Name:      "capacity",
		Help:      "Total storage capacity",
		Units:     "Bytes",
		AxisLabel: "Size"}
	metaAvailable = metric.Metadata{
		Name:      "capacity.available",
		Help:      "Available storage capacity",
		Units:     "Bytes",
		AxisLabel: "Size"}
	metaUsed = metric.Metadata{
		Name:      "capacity.used",
		Help:      "Used storage capacity",
		Units:     "Bytes",
		AxisLabel: "Size"}

	metaReserved = metric.Metadata{
		Name:      "capacity.reserved",
		Help:      "Capacity reserved for snapshots",
		Units:     "Bytes",
		AxisLabel: "Size"}
	metaSysBytes = metric.Metadata{
		Name:      "sysbytes",
		Help:      "Number of bytes in system KV pairs",
		Units:     "Bytes",
		AxisLabel: "Size"}
	metaSysCount = metric.Metadata{
		Name:      "syscount",
		Help:      "Count of system KV pairs",
		Units:     "Count",
		AxisLabel: "Keys"}

	// Metrics used by the rebalancing logic that aren't already captured elsewhere.
	metaAverageWritesPerSecond = metric.Metadata{
		Name:      "rebalancing.writespersecond",
		Help:      "Number of keys written (i.e. applied by raft) per second to the store, averaged over a large time period as used in rebalancing decisions",
		Units:     "Count",
		AxisLabel: "Keys/Sec"}

	// RocksDB metrics.
	metaRdbBlockCacheHits = metric.Metadata{
		Name:      "rocksdb.block.cache.hits",
		Help:      "Count of block cache hits",
		Units:     "Count",
		AxisLabel: "Cache Ops"}
	metaRdbBlockCacheMisses = metric.Metadata{
		Name:      "rocksdb.block.cache.misses",
		Help:      "Count of block cache misses",
		Units:     "Count",
		AxisLabel: "Cache Ops"}
	metaRdbBlockCacheUsage = metric.Metadata{
		Name:      "rocksdb.block.cache.usage",
		Help:      "Bytes used by the block cache",
		Units:     "Bytes",
		AxisLabel: "Size"}
	metaRdbBlockCachePinnedUsage = metric.Metadata{
		Name:      "rocksdb.block.cache.pinned-usage",
		Help:      "Bytes pinned by the block cache",
		Units:     "Bytes",
		AxisLabel: "Size"}
	metaRdbBloomFilterPrefixChecked = metric.Metadata{
		Name:      "rocksdb.bloom.filter.prefix.checked",
		Help:      "Number of times the bloom filter was checked",
		Units:     "Count",
		AxisLabel: "Bloom Filter Ops"}
	metaRdbBloomFilterPrefixUseful = metric.Metadata{
		Name:      "rocksdb.bloom.filter.prefix.useful",
		Help:      "Number of times the bloom filter helped avoid iterator creation",
		Units:     "Count",
		AxisLabel: "Bloom Filter Ops"}
	metaRdbMemtableTotalSize = metric.Metadata{
		Name:      "rocksdb.memtable.total-size",
		Help:      "Current size of memtable in bytes",
		Units:     "Bytes",
		AxisLabel: "Size"}
	metaRdbFlushes = metric.Metadata{
		Name:      "rocksdb.flushes",
		Help:      "Number of table flushes",
		Units:     "Count",
		AxisLabel: "Flushes"}
	metaRdbCompactions = metric.Metadata{
		Name:      "rocksdb.compactions",
		Help:      "Number of table compactions",
		Units:     "Count",
		AxisLabel: "Compactions"}
	metaRdbTableReadersMemEstimate = metric.Metadata{
		Name:      "rocksdb.table-readers-mem-estimate",
		Help:      "Memory used by index and filter blocks",
		Units:     "Bytes",
		AxisLabel: "Size"}
	metaRdbReadAmplification = metric.Metadata{
		Name:      "rocksdb.read-amplification",
		Help:      "Number of disk reads per query",
		Units:     "Count",
		AxisLabel: "Disk Reads per Query"}
	metaRdbNumSSTables = metric.Metadata{
		Name:      "rocksdb.num-sstables",
		Help:      "Number of rocksdb SSTables",
		Units:     "Count",
		AxisLabel: "SSTables"}

	// Range event metrics.
	metaRangeSplits = metric.Metadata{
		Name:      "range.splits",
		Help:      "Number of range splits",
		Units:     "Count",
		AxisLabel: "Range Ops"}
	metaRangeAdds = metric.Metadata{
		Name:      "range.adds",
		Help:      "Number of range additions",
		Units:     "Count",
		AxisLabel: "Range Ops"}
	metaRangeRemoves = metric.Metadata{
		Name:      "range.removes",
		Help:      "Number of range removals",
		Units:     "Count",
		AxisLabel: "Range Ops"}
	metaRangeSnapshotsGenerated = metric.Metadata{
		Name:      "range.snapshots.generated",
		Help:      "Number of generated snapshots",
		Units:     "Count",
		AxisLabel: "Snapshots"}
	metaRangeSnapshotsNormalApplied = metric.Metadata{
		Name:      "range.snapshots.normal-applied",
		Help:      "Number of applied snapshots",
		Units:     "Count",
		AxisLabel: "Snapshots"}
	metaRangeSnapshotsPreemptiveApplied = metric.Metadata{
		Name:      "range.snapshots.preemptive-applied",
		Help:      "Number of applied pre-emptive snapshots",
		Units:     "Count",
		AxisLabel: "Snapshots"}
	metaRangeRaftLeaderTransfers = metric.Metadata{
		Name:      "range.raftleadertransfers",
		Help:      "Number of raft leader transfers",
		Units:     "Count",
		AxisLabel: "Leader Transfers"}

	// Raft processing metrics.
	metaRaftTicks = metric.Metadata{
		Name:      "raft.ticks",
		Help:      "Number of Raft ticks queued",
		Units:     "Count",
		AxisLabel: "Ticks"}
	metaRaftWorkingDurationNanos = metric.Metadata{
		Name:      "raft.process.workingnanos",
		Help:      "Nanoseconds spent in store.processRaft() working",
		Units:     "Duration",
		AxisLabel: "Time"}
	metaRaftTickingDurationNanos = metric.Metadata{
		Name:      "raft.process.tickingnanos",
		Help:      "Nanoseconds spent in store.processRaft() processing replica.Tick()",
		Units:     "Duration",
		AxisLabel: "Time"}
	metaRaftCommandsApplied = metric.Metadata{
		Name:      "raft.commandsapplied",
		Help:      "Count of Raft commands applied",
		Units:     "Count",
		AxisLabel: "Commands"}
	metaRaftLogCommitLatency = metric.Metadata{
		Name:      "raft.process.logcommit.latency",
		Help:      "Latency histogram in nanoseconds for committing Raft log entries",
		Units:     "Count",
		AxisLabel: "Time"}
	metaRaftCommandCommitLatency = metric.Metadata{
		Name:      "raft.process.commandcommit.latency",
		Help:      "Latency histogram in nanoseconds for committing Raft commands",
		Units:     "Count",
		AxisLabel: "Time"}

	// Raft message metrics.
	metaRaftRcvdProp = metric.Metadata{
		Name:      "raft.rcvd.prop",
		Help:      "Number of MsgProp messages received by this store",
		Units:     "Count",
		AxisLabel: "Messages"}
	metaRaftRcvdApp = metric.Metadata{
		Name:      "raft.rcvd.app",
		Help:      "Number of MsgApp messages received by this store",
		Units:     "Count",
		AxisLabel: "Messages"}
	metaRaftRcvdAppResp = metric.Metadata{
		Name:      "raft.rcvd.appresp",
		Help:      "Number of MsgAppResp messages received by this store",
		Units:     "Count",
		AxisLabel: "Messages"}
	metaRaftRcvdVote = metric.Metadata{
		Name:      "raft.rcvd.vote",
		Help:      "Number of MsgVote messages received by this store",
		Units:     "Count",
		AxisLabel: "Messages"}
	metaRaftRcvdVoteResp = metric.Metadata{
		Name:      "raft.rcvd.voteresp",
		Help:      "Number of MsgVoteResp messages received by this store",
		Units:     "Count",
		AxisLabel: "Messages"}
	metaRaftRcvdPreVote = metric.Metadata{
		Name:      "raft.rcvd.prevote",
		Help:      "Number of MsgPreVote messages received by this store",
		Units:     "Count",
		AxisLabel: "Messages"}
	metaRaftRcvdPreVoteResp = metric.Metadata{
		Name:      "raft.rcvd.prevoteresp",
		Help:      "Number of MsgPreVoteResp messages received by this store",
		Units:     "Count",
		AxisLabel: "Messages"}
	metaRaftRcvdSnap = metric.Metadata{
		Name:      "raft.rcvd.snap",
		Help:      "Number of MsgSnap messages received by this store",
		Units:     "Count",
		AxisLabel: "Messages"}
	metaRaftRcvdHeartbeat = metric.Metadata{
		Name:      "raft.rcvd.heartbeat",
		Help:      "Number of (coalesced, if enabled) MsgHeartbeat messages received by this store",
		Units:     "Count",
		AxisLabel: "Messages"}
	metaRaftRcvdHeartbeatResp = metric.Metadata{
		Name:      "raft.rcvd.heartbeatresp",
		Help:      "Number of (coalesced, if enabled) MsgHeartbeatResp messages received by this store",
		Units:     "Count",
		AxisLabel: "Messages"}
	metaRaftRcvdTransferLeader = metric.Metadata{
		Name:      "raft.rcvd.transferleader",
		Help:      "Number of MsgTransferLeader messages received by this store",
		Units:     "Count",
		AxisLabel: "Messages"}
	metaRaftRcvdTimeoutNow = metric.Metadata{
		Name:      "raft.rcvd.timeoutnow",
		Help:      "Number of MsgTimeoutNow messages received by this store",
		Units:     "Count",
		AxisLabel: "Messages"}
	metaRaftRcvdDropped = metric.Metadata{
		Name:      "raft.rcvd.dropped",
		Help:      "Number of dropped incoming Raft messages",
		Units:     "Count",
		AxisLabel: "Messages"}
	metaRaftEnqueuedPending = metric.Metadata{
		Name:      "raft.enqueued.pending",
		Help:      "Number of pending outgoing messages in the Raft Transport queue",
		Units:     "Count",
		AxisLabel: "Messages"}
	metaRaftCoalescedHeartbeatsPending = metric.Metadata{
		Name:      "raft.heartbeats.pending",
		Help:      "Number of pending heartbeats and responses waiting to be coalesced",
		Units:     "Count",
		AxisLabel: "Messages"}

	// Raft log metrics.
	metaRaftLogFollowerBehindCount = metric.Metadata{
		Name:      "raftlog.behind",
		Help:      "Number of Raft log entries followers on other stores are behind",
		Units:     "Count",
		AxisLabel: "Log Entries"}
	metaRaftLogTruncated = metric.Metadata{
		Name:      "raftlog.truncated",
		Help:      "Number of Raft log entries truncated",
		Units:     "Count",
		AxisLabel: "Log Entries"}

	// Replica queue metrics.
	metaGCQueueSuccesses = metric.Metadata{
		Name:      "queue.gc.process.success",
		Help:      "Number of replicas successfully processed by the GC queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaGCQueueFailures = metric.Metadata{
		Name:      "queue.gc.process.failure",
		Help:      "Number of replicas which failed processing in the GC queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaGCQueuePending = metric.Metadata{
		Name:      "queue.gc.pending",
		Help:      "Number of pending replicas in the GC queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaGCQueueProcessingNanos = metric.Metadata{
		Name:      "queue.gc.processingnanos",
		Help:      "Nanoseconds spent processing replicas in the GC queue",
		Units:     "Duration",
		AxisLabel: "Time"}
	metaRaftLogQueueSuccesses = metric.Metadata{
		Name:      "queue.raftlog.process.success",
		Help:      "Number of replicas successfully processed by the Raft log queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaRaftLogQueueFailures = metric.Metadata{
		Name:      "queue.raftlog.process.failure",
		Help:      "Number of replicas which failed processing in the Raft log queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaRaftLogQueuePending = metric.Metadata{
		Name:      "queue.raftlog.pending",
		Help:      "Number of pending replicas in the Raft log queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaRaftLogQueueProcessingNanos = metric.Metadata{
		Name:      "queue.raftlog.processingnanos",
		Help:      "Nanoseconds spent processing replicas in the Raft log queue",
		Units:     "Duration",
		AxisLabel: "Time"}
	metaRaftSnapshotQueueSuccesses = metric.Metadata{
		Name:      "queue.raftsnapshot.process.success",
		Help:      "Number of replicas successfully processed by the Raft repair queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaRaftSnapshotQueueFailures = metric.Metadata{
		Name:      "queue.raftsnapshot.process.failure",
		Help:      "Number of replicas which failed processing in the Raft repair queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaRaftSnapshotQueuePending = metric.Metadata{
		Name:      "queue.raftsnapshot.pending",
		Help:      "Number of pending replicas in the Raft repair queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaRaftSnapshotQueueProcessingNanos = metric.Metadata{
		Name:      "queue.raftsnapshot.processingnanos",
		Help:      "Nanoseconds spent processing replicas in the Raft repair queue",
		Units:     "Duration",
		AxisLabel: "Time"}
	metaConsistencyQueueSuccesses = metric.Metadata{
		Name:      "queue.consistency.process.success",
		Help:      "Number of replicas successfully processed by the consistency checker queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaConsistencyQueueFailures = metric.Metadata{
		Name:      "queue.consistency.process.failure",
		Help:      "Number of replicas which failed processing in the consistency checker queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaConsistencyQueuePending = metric.Metadata{
		Name:      "queue.consistency.pending",
		Help:      "Number of pending replicas in the consistency checker queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaConsistencyQueueProcessingNanos = metric.Metadata{
		Name:      "queue.consistency.processingnanos",
		Help:      "Nanoseconds spent processing replicas in the consistency checker queue",
		Units:     "Duration",
		AxisLabel: "Time"}
	metaReplicaGCQueueSuccesses = metric.Metadata{
		Name:      "queue.replicagc.process.success",
		Help:      "Number of replicas successfully processed by the replica GC queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaReplicaGCQueueFailures = metric.Metadata{
		Name:      "queue.replicagc.process.failure",
		Help:      "Number of replicas which failed processing in the replica GC queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaReplicaGCQueuePending = metric.Metadata{
		Name:      "queue.replicagc.pending",
		Help:      "Number of pending replicas in the replica GC queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaReplicaGCQueueProcessingNanos = metric.Metadata{
		Name:      "queue.replicagc.processingnanos",
		Help:      "Nanoseconds spent processing replicas in the replica GC queue",
		Units:     "Duration",
		AxisLabel: "Time"}
	metaReplicateQueueSuccesses = metric.Metadata{
		Name:      "queue.replicate.process.success",
		Help:      "Number of replicas successfully processed by the replicate queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaReplicateQueueFailures = metric.Metadata{
		Name:      "queue.replicate.process.failure",
		Help:      "Number of replicas which failed processing in the replicate queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaReplicateQueuePending = metric.Metadata{
		Name:      "queue.replicate.pending",
		Help:      "Number of pending replicas in the replicate queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaReplicateQueueProcessingNanos = metric.Metadata{
		Name:      "queue.replicate.processingnanos",
		Help:      "Nanoseconds spent processing replicas in the replicate queue",
		Units:     "Duration",
		AxisLabel: "Time"}
	metaReplicateQueuePurgatory = metric.Metadata{
		Name:      "queue.replicate.purgatory",
		Help:      "Number of replicas in the replicate queue's purgatory, awaiting allocation options",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaSplitQueueSuccesses = metric.Metadata{
		Name:      "queue.split.process.success",
		Help:      "Number of replicas successfully processed by the split queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaSplitQueueFailures = metric.Metadata{
		Name:      "queue.split.process.failure",
		Help:      "Number of replicas which failed processing in the split queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaSplitQueuePending = metric.Metadata{
		Name:      "queue.split.pending",
		Help:      "Number of pending replicas in the split queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaSplitQueueProcessingNanos = metric.Metadata{
		Name:      "queue.split.processingnanos",
		Help:      "Nanoseconds spent processing replicas in the split queue",
		Units:     "Duration",
		AxisLabel: "Time"}
	metaTimeSeriesMaintenanceQueueSuccesses = metric.Metadata{
		Name:      "queue.tsmaintenance.process.success",
		Help:      "Number of replicas successfully processed by the time series maintenance queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaTimeSeriesMaintenanceQueueFailures = metric.Metadata{
		Name:      "queue.tsmaintenance.process.failure",
		Help:      "Number of replicas which failed processing in the time series maintenance queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaTimeSeriesMaintenanceQueuePending = metric.Metadata{
		Name:      "queue.tsmaintenance.pending",
		Help:      "Number of pending replicas in the time series maintenance queue",
		Units:     "Count",
		AxisLabel: "Replicas"}
	metaTimeSeriesMaintenanceQueueProcessingNanos = metric.Metadata{
		Name:      "queue.tsmaintenance.processingnanos",
		Help:      "Nanoseconds spent processing replicas in the time series maintenance queue",
		Units:     "Duration",
		AxisLabel: "Time"}

	// GCInfo cumulative totals.
	metaGCNumKeysAffected = metric.Metadata{
		Name:      "queue.gc.info.numkeysaffected",
		Help:      "Number of keys with GC'able data",
		Units:     "Count",
		AxisLabel: "Keys"}
	metaGCIntentsConsidered = metric.Metadata{
		Name:      "queue.gc.info.intentsconsidered",
		Help:      "Number of 'old' intents",
		Units:     "Count",
		AxisLabel: "Intents"}
	metaGCIntentTxns = metric.Metadata{
		Name:      "queue.gc.info.intenttxns",
		Help:      "Number of associated distinct transactions",
		Units:     "Count",
		AxisLabel: "Txns"}
	metaGCTransactionSpanScanned = metric.Metadata{
		Name:      "queue.gc.info.transactionspanscanned",
		Help:      "Number of entries in transaction spans scanned from the engine",
		Units:     "Count",
		AxisLabel: "Entries"}
	metaGCTransactionSpanGCAborted = metric.Metadata{
		Name:      "queue.gc.info.transactionspangcaborted",
		Help:      "Number of GC'able entries corresponding to aborted txns",
		Units:     "Count",
		AxisLabel: "Entries"}
	metaGCTransactionSpanGCCommitted = metric.Metadata{
		Name:      "queue.gc.info.transactionspangccommitted",
		Help:      "Number of GC'able entries corresponding to committed txns",
		Units:     "Count",
		AxisLabel: "Entries"}
	metaGCTransactionSpanGCPending = metric.Metadata{
		Name:      "queue.gc.info.transactionspangcpending",
		Help:      "Number of GC'able entries corresponding to pending txns",
		Units:     "Count",
		AxisLabel: "Entries"}
	metaGCAbortSpanScanned = metric.Metadata{
		Name:      "queue.gc.info.abortspanscanned",
		Help:      "Number of transactions present in the AbortSpan scanned from the engine",
		Units:     "Count",
		AxisLabel: "Entries"}
	metaGCAbortSpanConsidered = metric.Metadata{
		Name:      "queue.gc.info.abortspanconsidered",
		Help:      "Number of AbortSpan entries old enough to be considered for removal",
		Units:     "Count",
		AxisLabel: "Entries"}
	metaGCAbortSpanGCNum = metric.Metadata{
		Name:      "queue.gc.info.abortspangcnum",
		Help:      "Number of AbortSpan entries fit for removal",
		Units:     "Count",
		AxisLabel: "Entries"}
	metaGCPushTxn = metric.Metadata{
		Name:      "queue.gc.info.pushtxn",
		Help:      "Number of attempted pushes",
		Units:     "Count",
		AxisLabel: "Pushes"}
	metaGCResolveTotal = metric.Metadata{
		Name:      "queue.gc.info.resolvetotal",
		Help:      "Number of attempted intent resolutions",
		Units:     "Count",
		AxisLabel: "Intent Resolutions"}
	metaGCResolveSuccess = metric.Metadata{
		Name:      "queue.gc.info.resolvesuccess",
		Help:      "Number of successful intent resolutions",
		Units:     "Count",
		AxisLabel: "Intent Resolutions"}

	// Slow request metrics.
	metaSlowCommandQueueRequests = metric.Metadata{
		Name:      "requests.slow.commandqueue",
		Help:      "Number of requests that have been stuck for a long time in the command queue",
		Units:     "Count",
		AxisLabel: "Requests"}
	metaSlowLeaseRequests = metric.Metadata{
		Name:      "requests.slow.lease",
		Help:      "Number of requests that have been stuck for a long time acquiring a lease",
		Units:     "Count",
		AxisLabel: "Requests"}
	metaSlowRaftRequests = metric.Metadata{
		Name:      "requests.slow.raft",
		Help:      "Number of requests that have been stuck for a long time in raft",
		Units:     "Count",
		AxisLabel: "Requests"}

	// Backpressure metrics.
	metaBackpressuredOnSplitRequests = metric.Metadata{
		Name:      "requests.backpressure.split",
		Help:      "Number of backpressured writes waiting on a Range split",
		Units:     "Count",
		AxisLabel: "Writes"}

	// AddSSTable metrics.
	metaAddSSTableProposals = metric.Metadata{
		Name:      "addsstable.proposals",
		Help:      "Number of SSTable ingestions proposed (i.e. sent to Raft by lease holders)",
		Units:     "Count",
		AxisLabel: "Ingestions"}
	metaAddSSTableApplications = metric.Metadata{
		Name:      "addsstable.applications",
		Help:      "Number of SSTable ingestions applied (i.e. applied by Replicas)",
		Units:     "Count",
		AxisLabel: "Ingestions"}
	metaAddSSTableApplicationCopies = metric.Metadata{
		Name: "addsstable.copies", Units: "Count", AxisLabel: "Ingestions",
		Help: "number of SSTable ingestions that required copying files during application",
	}
)

// StoreMetrics is the set of metrics for a given store.
type StoreMetrics struct {
	registry *metric.Registry

	// Replica metrics.
	ReplicaCount                  *metric.Gauge // Does not include reserved replicas.
	ReservedReplicaCount          *metric.Gauge
	RaftLeaderCount               *metric.Gauge
	RaftLeaderNotLeaseHolderCount *metric.Gauge
	LeaseHolderCount              *metric.Gauge
	QuiescentCount                *metric.Gauge

	// Replica CommandQueue metrics.
	MaxCommandQueueSize       *metric.Gauge
	MaxCommandQueueWriteCount *metric.Gauge
	MaxCommandQueueReadCount  *metric.Gauge
	MaxCommandQueueTreeSize   *metric.Gauge
	MaxCommandQueueOverlaps   *metric.Gauge
	CombinedCommandQueueSize  *metric.Gauge
	CombinedCommandWriteCount *metric.Gauge
	CombinedCommandReadCount  *metric.Gauge

	// Range metrics.
	RangeCount                *metric.Gauge
	UnavailableRangeCount     *metric.Gauge
	UnderReplicatedRangeCount *metric.Gauge

	// Lease request metrics for successful and failed lease requests. These
	// count proposals (i.e. it does not matter how many replicas apply the
	// lease).
	LeaseRequestSuccessCount  *metric.Counter
	LeaseRequestErrorCount    *metric.Counter
	LeaseTransferSuccessCount *metric.Counter
	LeaseTransferErrorCount   *metric.Counter
	LeaseExpirationCount      *metric.Gauge
	LeaseEpochCount           *metric.Gauge

	// Storage metrics.
	LiveBytes       *metric.Gauge
	KeyBytes        *metric.Gauge
	ValBytes        *metric.Gauge
	TotalBytes      *metric.Gauge
	IntentBytes     *metric.Gauge
	LiveCount       *metric.Gauge
	KeyCount        *metric.Gauge
	ValCount        *metric.Gauge
	IntentCount     *metric.Gauge
	IntentAge       *metric.Gauge
	GcBytesAge      *metric.Gauge
	LastUpdateNanos *metric.Gauge
	Capacity        *metric.Gauge
	Available       *metric.Gauge
	Used            *metric.Gauge
	Reserved        *metric.Gauge
	SysBytes        *metric.Gauge
	SysCount        *metric.Gauge

	// Rebalancing metrics.
	AverageWritesPerSecond *metric.GaugeFloat64

	// RocksDB metrics.
	RdbBlockCacheHits           *metric.Gauge
	RdbBlockCacheMisses         *metric.Gauge
	RdbBlockCacheUsage          *metric.Gauge
	RdbBlockCachePinnedUsage    *metric.Gauge
	RdbBloomFilterPrefixChecked *metric.Gauge
	RdbBloomFilterPrefixUseful  *metric.Gauge
	RdbMemtableTotalSize        *metric.Gauge
	RdbFlushes                  *metric.Gauge
	RdbCompactions              *metric.Gauge
	RdbTableReadersMemEstimate  *metric.Gauge
	RdbReadAmplification        *metric.Gauge
	RdbNumSSTables              *metric.Gauge

	// TODO(mrtracy): This should be removed as part of #4465. This is only
	// maintained to keep the current structure of StatusSummaries; it would be
	// better to convert the Gauges above into counters which are adjusted
	// accordingly.

	// Range event metrics.
	RangeSplits                     *metric.Counter
	RangeAdds                       *metric.Counter
	RangeRemoves                    *metric.Counter
	RangeSnapshotsGenerated         *metric.Counter
	RangeSnapshotsNormalApplied     *metric.Counter
	RangeSnapshotsPreemptiveApplied *metric.Counter
	RangeRaftLeaderTransfers        *metric.Counter

	// Raft processing metrics.
	RaftTicks                *metric.Counter
	RaftWorkingDurationNanos *metric.Counter
	RaftTickingDurationNanos *metric.Counter
	RaftCommandsApplied      *metric.Counter
	RaftLogCommitLatency     *metric.Histogram
	RaftCommandCommitLatency *metric.Histogram

	// Raft message metrics.
	RaftRcvdMsgProp           *metric.Counter
	RaftRcvdMsgApp            *metric.Counter
	RaftRcvdMsgAppResp        *metric.Counter
	RaftRcvdMsgVote           *metric.Counter
	RaftRcvdMsgVoteResp       *metric.Counter
	RaftRcvdMsgPreVote        *metric.Counter
	RaftRcvdMsgPreVoteResp    *metric.Counter
	RaftRcvdMsgSnap           *metric.Counter
	RaftRcvdMsgHeartbeat      *metric.Counter
	RaftRcvdMsgHeartbeatResp  *metric.Counter
	RaftRcvdMsgTransferLeader *metric.Counter
	RaftRcvdMsgTimeoutNow     *metric.Counter
	RaftRcvdMsgDropped        *metric.Counter

	// Raft log metrics.
	RaftLogFollowerBehindCount *metric.Gauge
	RaftLogTruncated           *metric.Counter

	// A map for conveniently finding the appropriate metric. The individual
	// metric references must exist as AddMetricStruct adds them by reflection
	// on this struct and does not process map types.
	// TODO(arjun): eliminate this duplication.
	raftRcvdMessages map[raftpb.MessageType]*metric.Counter

	RaftEnqueuedPending            *metric.Gauge
	RaftCoalescedHeartbeatsPending *metric.Gauge

	// Replica queue metrics.
	GCQueueSuccesses                          *metric.Counter
	GCQueueFailures                           *metric.Counter
	GCQueuePending                            *metric.Gauge
	GCQueueProcessingNanos                    *metric.Counter
	RaftLogQueueSuccesses                     *metric.Counter
	RaftLogQueueFailures                      *metric.Counter
	RaftLogQueuePending                       *metric.Gauge
	RaftLogQueueProcessingNanos               *metric.Counter
	RaftSnapshotQueueSuccesses                *metric.Counter
	RaftSnapshotQueueFailures                 *metric.Counter
	RaftSnapshotQueuePending                  *metric.Gauge
	RaftSnapshotQueueProcessingNanos          *metric.Counter
	ConsistencyQueueSuccesses                 *metric.Counter
	ConsistencyQueueFailures                  *metric.Counter
	ConsistencyQueuePending                   *metric.Gauge
	ConsistencyQueueProcessingNanos           *metric.Counter
	ReplicaGCQueueSuccesses                   *metric.Counter
	ReplicaGCQueueFailures                    *metric.Counter
	ReplicaGCQueuePending                     *metric.Gauge
	ReplicaGCQueueProcessingNanos             *metric.Counter
	ReplicateQueueSuccesses                   *metric.Counter
	ReplicateQueueFailures                    *metric.Counter
	ReplicateQueuePending                     *metric.Gauge
	ReplicateQueueProcessingNanos             *metric.Counter
	ReplicateQueuePurgatory                   *metric.Gauge
	SplitQueueSuccesses                       *metric.Counter
	SplitQueueFailures                        *metric.Counter
	SplitQueuePending                         *metric.Gauge
	SplitQueueProcessingNanos                 *metric.Counter
	TimeSeriesMaintenanceQueueSuccesses       *metric.Counter
	TimeSeriesMaintenanceQueueFailures        *metric.Counter
	TimeSeriesMaintenanceQueuePending         *metric.Gauge
	TimeSeriesMaintenanceQueueProcessingNanos *metric.Counter

	// GCInfo cumulative totals.
	GCNumKeysAffected            *metric.Counter
	GCIntentsConsidered          *metric.Counter
	GCIntentTxns                 *metric.Counter
	GCTransactionSpanScanned     *metric.Counter
	GCTransactionSpanGCAborted   *metric.Counter
	GCTransactionSpanGCCommitted *metric.Counter
	GCTransactionSpanGCPending   *metric.Counter
	GCAbortSpanScanned           *metric.Counter
	GCAbortSpanConsidered        *metric.Counter
	GCAbortSpanGCNum             *metric.Counter
	GCPushTxn                    *metric.Counter
	GCResolveTotal               *metric.Counter
	GCResolveSuccess             *metric.Counter

	// Slow request counts.
	SlowCommandQueueRequests *metric.Gauge
	SlowLeaseRequests        *metric.Gauge
	SlowRaftRequests         *metric.Gauge

	// Backpressure counts.
	BackpressuredOnSplitRequests *metric.Gauge

	// AddSSTable stats: how many AddSSTable commands were proposed and how many
	// were applied? How many applications required writing a copy?
	AddSSTableProposals         *metric.Counter
	AddSSTableApplications      *metric.Counter
	AddSSTableApplicationCopies *metric.Counter

	// Stats for efficient merges.
	mu struct {
		syncutil.Mutex
		stats enginepb.MVCCStats
	}
}

func newStoreMetrics(histogramWindow time.Duration) *StoreMetrics {
	storeRegistry := metric.NewRegistry()
	sm := &StoreMetrics{
		registry: storeRegistry,

		// Replica metrics.
		ReplicaCount:                  metric.NewGauge(metaReplicaCount),
		ReservedReplicaCount:          metric.NewGauge(metaReservedReplicaCount),
		RaftLeaderCount:               metric.NewGauge(metaRaftLeaderCount),
		RaftLeaderNotLeaseHolderCount: metric.NewGauge(metaRaftLeaderNotLeaseHolderCount),
		LeaseHolderCount:              metric.NewGauge(metaLeaseHolderCount),
		QuiescentCount:                metric.NewGauge(metaQuiescentCount),

		// Replica CommandQueue metrics.
		MaxCommandQueueSize:       metric.NewGauge(metaMaxCommandQueueSize),
		MaxCommandQueueWriteCount: metric.NewGauge(metaMaxCommandQueueWriteCount),
		MaxCommandQueueReadCount:  metric.NewGauge(metaMaxCommandQueueReadCount),
		MaxCommandQueueTreeSize:   metric.NewGauge(metaMaxCommandQueueTreeSize),
		MaxCommandQueueOverlaps:   metric.NewGauge(metaMaxCommandQueueOverlaps),
		CombinedCommandQueueSize:  metric.NewGauge(metaCombinedCommandQueueSize),
		CombinedCommandWriteCount: metric.NewGauge(metaCombinedCommandWriteCount),
		CombinedCommandReadCount:  metric.NewGauge(metaCombinedCommandReadCount),

		// Range metrics.
		RangeCount:                metric.NewGauge(metaRangeCount),
		UnavailableRangeCount:     metric.NewGauge(metaUnavailableRangeCount),
		UnderReplicatedRangeCount: metric.NewGauge(metaUnderReplicatedRangeCount),

		// Lease request metrics.
		LeaseRequestSuccessCount:  metric.NewCounter(metaLeaseRequestSuccessCount),
		LeaseRequestErrorCount:    metric.NewCounter(metaLeaseRequestErrorCount),
		LeaseTransferSuccessCount: metric.NewCounter(metaLeaseTransferSuccessCount),
		LeaseTransferErrorCount:   metric.NewCounter(metaLeaseTransferErrorCount),
		LeaseExpirationCount:      metric.NewGauge(metaLeaseExpirationCount),
		LeaseEpochCount:           metric.NewGauge(metaLeaseEpochCount),

		// Storage metrics.
		LiveBytes:       metric.NewGauge(metaLiveBytes),
		KeyBytes:        metric.NewGauge(metaKeyBytes),
		ValBytes:        metric.NewGauge(metaValBytes),
		TotalBytes:      metric.NewGauge(metaTotalBytes),
		IntentBytes:     metric.NewGauge(metaIntentBytes),
		LiveCount:       metric.NewGauge(metaLiveCount),
		KeyCount:        metric.NewGauge(metaKeyCount),
		ValCount:        metric.NewGauge(metaValCount),
		IntentCount:     metric.NewGauge(metaIntentCount),
		IntentAge:       metric.NewGauge(metaIntentAge),
		GcBytesAge:      metric.NewGauge(metaGcBytesAge),
		LastUpdateNanos: metric.NewGauge(metaLastUpdateNanos),
		Capacity:        metric.NewGauge(metaCapacity),
		Available:       metric.NewGauge(metaAvailable),
		Used:            metric.NewGauge(metaUsed),
		Reserved:        metric.NewGauge(metaReserved),
		SysBytes:        metric.NewGauge(metaSysBytes),
		SysCount:        metric.NewGauge(metaSysCount),

		// Rebalancing metrics.
		AverageWritesPerSecond: metric.NewGaugeFloat64(metaAverageWritesPerSecond),

		// RocksDB metrics.
		RdbBlockCacheHits:           metric.NewGauge(metaRdbBlockCacheHits),
		RdbBlockCacheMisses:         metric.NewGauge(metaRdbBlockCacheMisses),
		RdbBlockCacheUsage:          metric.NewGauge(metaRdbBlockCacheUsage),
		RdbBlockCachePinnedUsage:    metric.NewGauge(metaRdbBlockCachePinnedUsage),
		RdbBloomFilterPrefixChecked: metric.NewGauge(metaRdbBloomFilterPrefixChecked),
		RdbBloomFilterPrefixUseful:  metric.NewGauge(metaRdbBloomFilterPrefixUseful),
		RdbMemtableTotalSize:        metric.NewGauge(metaRdbMemtableTotalSize),
		RdbFlushes:                  metric.NewGauge(metaRdbFlushes),
		RdbCompactions:              metric.NewGauge(metaRdbCompactions),
		RdbTableReadersMemEstimate:  metric.NewGauge(metaRdbTableReadersMemEstimate),
		RdbReadAmplification:        metric.NewGauge(metaRdbReadAmplification),
		RdbNumSSTables:              metric.NewGauge(metaRdbNumSSTables),

		// Range event metrics.
		RangeSplits:                     metric.NewCounter(metaRangeSplits),
		RangeAdds:                       metric.NewCounter(metaRangeAdds),
		RangeRemoves:                    metric.NewCounter(metaRangeRemoves),
		RangeSnapshotsGenerated:         metric.NewCounter(metaRangeSnapshotsGenerated),
		RangeSnapshotsNormalApplied:     metric.NewCounter(metaRangeSnapshotsNormalApplied),
		RangeSnapshotsPreemptiveApplied: metric.NewCounter(metaRangeSnapshotsPreemptiveApplied),
		RangeRaftLeaderTransfers:        metric.NewCounter(metaRangeRaftLeaderTransfers),

		// Raft processing metrics.
		RaftTicks:                metric.NewCounter(metaRaftTicks),
		RaftWorkingDurationNanos: metric.NewCounter(metaRaftWorkingDurationNanos),
		RaftTickingDurationNanos: metric.NewCounter(metaRaftTickingDurationNanos),
		RaftCommandsApplied:      metric.NewCounter(metaRaftCommandsApplied),
		RaftLogCommitLatency:     metric.NewLatency(metaRaftLogCommitLatency, histogramWindow),
		RaftCommandCommitLatency: metric.NewLatency(metaRaftCommandCommitLatency, histogramWindow),

		// Raft message metrics.
		RaftRcvdMsgProp:           metric.NewCounter(metaRaftRcvdProp),
		RaftRcvdMsgApp:            metric.NewCounter(metaRaftRcvdApp),
		RaftRcvdMsgAppResp:        metric.NewCounter(metaRaftRcvdAppResp),
		RaftRcvdMsgVote:           metric.NewCounter(metaRaftRcvdVote),
		RaftRcvdMsgVoteResp:       metric.NewCounter(metaRaftRcvdVoteResp),
		RaftRcvdMsgPreVote:        metric.NewCounter(metaRaftRcvdPreVote),
		RaftRcvdMsgPreVoteResp:    metric.NewCounter(metaRaftRcvdPreVoteResp),
		RaftRcvdMsgSnap:           metric.NewCounter(metaRaftRcvdSnap),
		RaftRcvdMsgHeartbeat:      metric.NewCounter(metaRaftRcvdHeartbeat),
		RaftRcvdMsgHeartbeatResp:  metric.NewCounter(metaRaftRcvdHeartbeatResp),
		RaftRcvdMsgTransferLeader: metric.NewCounter(metaRaftRcvdTransferLeader),
		RaftRcvdMsgTimeoutNow:     metric.NewCounter(metaRaftRcvdTimeoutNow),
		RaftRcvdMsgDropped:        metric.NewCounter(metaRaftRcvdDropped),
		raftRcvdMessages:          make(map[raftpb.MessageType]*metric.Counter, len(raftpb.MessageType_name)),

		RaftEnqueuedPending: metric.NewGauge(metaRaftEnqueuedPending),

		// This Gauge measures the number of heartbeats queued up just before
		// the queue is cleared, to avoid flapping wildly.
		RaftCoalescedHeartbeatsPending: metric.NewGauge(metaRaftCoalescedHeartbeatsPending),

		// Raft log metrics.
		RaftLogFollowerBehindCount: metric.NewGauge(metaRaftLogFollowerBehindCount),
		RaftLogTruncated:           metric.NewCounter(metaRaftLogTruncated),

		// Replica queue metrics.
		GCQueueSuccesses:                          metric.NewCounter(metaGCQueueSuccesses),
		GCQueueFailures:                           metric.NewCounter(metaGCQueueFailures),
		GCQueuePending:                            metric.NewGauge(metaGCQueuePending),
		GCQueueProcessingNanos:                    metric.NewCounter(metaGCQueueProcessingNanos),
		RaftLogQueueSuccesses:                     metric.NewCounter(metaRaftLogQueueSuccesses),
		RaftLogQueueFailures:                      metric.NewCounter(metaRaftLogQueueFailures),
		RaftLogQueuePending:                       metric.NewGauge(metaRaftLogQueuePending),
		RaftLogQueueProcessingNanos:               metric.NewCounter(metaRaftLogQueueProcessingNanos),
		RaftSnapshotQueueSuccesses:                metric.NewCounter(metaRaftSnapshotQueueSuccesses),
		RaftSnapshotQueueFailures:                 metric.NewCounter(metaRaftSnapshotQueueFailures),
		RaftSnapshotQueuePending:                  metric.NewGauge(metaRaftSnapshotQueuePending),
		RaftSnapshotQueueProcessingNanos:          metric.NewCounter(metaRaftSnapshotQueueProcessingNanos),
		ConsistencyQueueSuccesses:                 metric.NewCounter(metaConsistencyQueueSuccesses),
		ConsistencyQueueFailures:                  metric.NewCounter(metaConsistencyQueueFailures),
		ConsistencyQueuePending:                   metric.NewGauge(metaConsistencyQueuePending),
		ConsistencyQueueProcessingNanos:           metric.NewCounter(metaConsistencyQueueProcessingNanos),
		ReplicaGCQueueSuccesses:                   metric.NewCounter(metaReplicaGCQueueSuccesses),
		ReplicaGCQueueFailures:                    metric.NewCounter(metaReplicaGCQueueFailures),
		ReplicaGCQueuePending:                     metric.NewGauge(metaReplicaGCQueuePending),
		ReplicaGCQueueProcessingNanos:             metric.NewCounter(metaReplicaGCQueueProcessingNanos),
		ReplicateQueueSuccesses:                   metric.NewCounter(metaReplicateQueueSuccesses),
		ReplicateQueueFailures:                    metric.NewCounter(metaReplicateQueueFailures),
		ReplicateQueuePending:                     metric.NewGauge(metaReplicateQueuePending),
		ReplicateQueueProcessingNanos:             metric.NewCounter(metaReplicateQueueProcessingNanos),
		ReplicateQueuePurgatory:                   metric.NewGauge(metaReplicateQueuePurgatory),
		SplitQueueSuccesses:                       metric.NewCounter(metaSplitQueueSuccesses),
		SplitQueueFailures:                        metric.NewCounter(metaSplitQueueFailures),
		SplitQueuePending:                         metric.NewGauge(metaSplitQueuePending),
		SplitQueueProcessingNanos:                 metric.NewCounter(metaSplitQueueProcessingNanos),
		TimeSeriesMaintenanceQueueSuccesses:       metric.NewCounter(metaTimeSeriesMaintenanceQueueFailures),
		TimeSeriesMaintenanceQueueFailures:        metric.NewCounter(metaTimeSeriesMaintenanceQueueSuccesses),
		TimeSeriesMaintenanceQueuePending:         metric.NewGauge(metaTimeSeriesMaintenanceQueuePending),
		TimeSeriesMaintenanceQueueProcessingNanos: metric.NewCounter(metaTimeSeriesMaintenanceQueueProcessingNanos),

		// GCInfo cumulative totals.
		GCNumKeysAffected:            metric.NewCounter(metaGCNumKeysAffected),
		GCIntentsConsidered:          metric.NewCounter(metaGCIntentsConsidered),
		GCIntentTxns:                 metric.NewCounter(metaGCIntentTxns),
		GCTransactionSpanScanned:     metric.NewCounter(metaGCTransactionSpanScanned),
		GCTransactionSpanGCAborted:   metric.NewCounter(metaGCTransactionSpanGCAborted),
		GCTransactionSpanGCCommitted: metric.NewCounter(metaGCTransactionSpanGCCommitted),
		GCTransactionSpanGCPending:   metric.NewCounter(metaGCTransactionSpanGCPending),
		GCAbortSpanScanned:           metric.NewCounter(metaGCAbortSpanScanned),
		GCAbortSpanConsidered:        metric.NewCounter(metaGCAbortSpanConsidered),
		GCAbortSpanGCNum:             metric.NewCounter(metaGCAbortSpanGCNum),
		GCPushTxn:                    metric.NewCounter(metaGCPushTxn),
		GCResolveTotal:               metric.NewCounter(metaGCResolveTotal),
		GCResolveSuccess:             metric.NewCounter(metaGCResolveSuccess),

		// Wedge request counters.
		SlowCommandQueueRequests: metric.NewGauge(metaSlowCommandQueueRequests),
		SlowLeaseRequests:        metric.NewGauge(metaSlowLeaseRequests),
		SlowRaftRequests:         metric.NewGauge(metaSlowRaftRequests),

		// Backpressure counters.
		BackpressuredOnSplitRequests: metric.NewGauge(metaBackpressuredOnSplitRequests),

		// AddSSTable proposal + applications counters.
		AddSSTableProposals:         metric.NewCounter(metaAddSSTableProposals),
		AddSSTableApplications:      metric.NewCounter(metaAddSSTableApplications),
		AddSSTableApplicationCopies: metric.NewCounter(metaAddSSTableApplicationCopies),
	}

	sm.raftRcvdMessages[raftpb.MsgProp] = sm.RaftRcvdMsgProp
	sm.raftRcvdMessages[raftpb.MsgApp] = sm.RaftRcvdMsgApp
	sm.raftRcvdMessages[raftpb.MsgAppResp] = sm.RaftRcvdMsgAppResp
	sm.raftRcvdMessages[raftpb.MsgVote] = sm.RaftRcvdMsgVote
	sm.raftRcvdMessages[raftpb.MsgVoteResp] = sm.RaftRcvdMsgVoteResp
	sm.raftRcvdMessages[raftpb.MsgPreVote] = sm.RaftRcvdMsgPreVote
	sm.raftRcvdMessages[raftpb.MsgPreVoteResp] = sm.RaftRcvdMsgPreVoteResp
	sm.raftRcvdMessages[raftpb.MsgSnap] = sm.RaftRcvdMsgSnap
	sm.raftRcvdMessages[raftpb.MsgHeartbeat] = sm.RaftRcvdMsgHeartbeat
	sm.raftRcvdMessages[raftpb.MsgHeartbeatResp] = sm.RaftRcvdMsgHeartbeatResp
	sm.raftRcvdMessages[raftpb.MsgTransferLeader] = sm.RaftRcvdMsgTransferLeader
	sm.raftRcvdMessages[raftpb.MsgTimeoutNow] = sm.RaftRcvdMsgTimeoutNow

	storeRegistry.AddMetricStruct(sm)

	return sm
}

// updateGaugesLocked breaks out individual metrics from the MVCCStats object.
// This process should be locked with each stat application to ensure that all
// gauges increase/decrease in step with the application of updates. However,
// this locking is not exposed to the registry level, and therefore a single
// snapshot of these gauges in the registry might mix the values of two
// subsequent updates.
func (sm *StoreMetrics) updateMVCCGaugesLocked() {
	sm.LiveBytes.Update(sm.mu.stats.LiveBytes)
	sm.KeyBytes.Update(sm.mu.stats.KeyBytes)
	sm.ValBytes.Update(sm.mu.stats.ValBytes)
	sm.TotalBytes.Update(sm.mu.stats.Total())
	sm.IntentBytes.Update(sm.mu.stats.IntentBytes)
	sm.LiveCount.Update(sm.mu.stats.LiveCount)
	sm.KeyCount.Update(sm.mu.stats.KeyCount)
	sm.ValCount.Update(sm.mu.stats.ValCount)
	sm.IntentCount.Update(sm.mu.stats.IntentCount)
	sm.IntentAge.Update(sm.mu.stats.IntentAge)
	sm.GcBytesAge.Update(sm.mu.stats.GCBytesAge)
	sm.LastUpdateNanos.Update(sm.mu.stats.LastUpdateNanos)
	sm.SysBytes.Update(sm.mu.stats.SysBytes)
	sm.SysCount.Update(sm.mu.stats.SysCount)
}

func (sm *StoreMetrics) addMVCCStats(stats enginepb.MVCCStats) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.mu.stats.Add(stats)
	sm.updateMVCCGaugesLocked()
}

func (sm *StoreMetrics) subtractMVCCStats(stats enginepb.MVCCStats) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.mu.stats.Subtract(stats)
	sm.updateMVCCGaugesLocked()
}

func (sm *StoreMetrics) updateRocksDBStats(stats engine.Stats) {
	// We do not grab a lock here, because it's not possible to get a point-in-
	// time snapshot of RocksDB stats. Retrieving RocksDB stats doesn't grab any
	// locks, and there's no way to retrieve multiple stats in a single operation.
	sm.RdbBlockCacheHits.Update(stats.BlockCacheHits)
	sm.RdbBlockCacheMisses.Update(stats.BlockCacheMisses)
	sm.RdbBlockCacheUsage.Update(stats.BlockCacheUsage)
	sm.RdbBlockCachePinnedUsage.Update(stats.BlockCachePinnedUsage)
	sm.RdbBloomFilterPrefixUseful.Update(stats.BloomFilterPrefixUseful)
	sm.RdbBloomFilterPrefixChecked.Update(stats.BloomFilterPrefixChecked)
	sm.RdbMemtableTotalSize.Update(stats.MemtableTotalSize)
	sm.RdbFlushes.Update(stats.Flushes)
	sm.RdbCompactions.Update(stats.Compactions)
	sm.RdbTableReadersMemEstimate.Update(stats.TableReadersMemEstimate)
}

func (sm *StoreMetrics) leaseRequestComplete(success bool) {
	if success {
		sm.LeaseRequestSuccessCount.Inc(1)
	} else {
		sm.LeaseRequestErrorCount.Inc(1)
	}
}

func (sm *StoreMetrics) leaseTransferComplete(success bool) {
	if success {
		sm.LeaseTransferSuccessCount.Inc(1)
	} else {
		sm.LeaseTransferErrorCount.Inc(1)
	}
}
