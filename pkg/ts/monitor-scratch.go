// // Copyright 2018 The Cockroach Authors.
// //
// // Licensed under the Apache License, Version 2.0 (the "License");
// // you may not use this file except in compliance with the License.
// // You may obtain a copy of the License at
// //
// //     http://www.apache.org/licenses/LICENSE-2.0
// //
// // Unless required by applicable law or agreed to in writing, software
// // distributed under the License is distributed on an "AS IS" BASIS,
// // WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// // implied. See the License for the specific language governing
// // permissions and limitations under the License.

// // QUESTIONS
// // How to identify sources?
// // How to make QueryMemoryContext?
// // How to bring in context?

package ts

// import (
// 	"context"
// 	"fmt"
// 	"math"

// 	prometheusgo "github.com/prometheus/client_model/go"

// 	"github.com/cockroachdb/cockroach/pkg/roachpb"
// 	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
// 	"github.com/cockroachdb/cockroach/pkg/util/metric"
// 	"github.com/cockroachdb/cockroach/pkg/util/mon"
// 	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
// )

// type Monitor struct {
// 	db       *DB
// 	metadata map[string]metric.Metadata
// 	mem      QueryMemoryContext
// }

// func NewMonitor(db *DB, md map[string]metric.Metadata, st *cluster.Settings) *Monitor {

// 	bytesMonitor := mon.MakeMonitor("timeseries monitor", mon.MemoryResource, nil, nil, 0, 100*1024*1024, st)
// 	bytesMonitor.Start(context.TODO(), nil, mon.MakeStandaloneBudget(100*1024*1024))
// 	bytesMonitor.MakeBoundAccount()

// 	memOpts := QueryMemoryOptions{
// 		// Large budget, but not maximum to avoid overflows.
// 		BudgetBytes:             math.MaxInt64,
// 		EstimatedSources:        1, // Not needed for rollups
// 		InterpolationLimitNanos: 0,
// 		Columnar:                db.WriteColumnar(),
// 	}

// 	qmc := MakeQueryMemoryContext(&bytesMonitor, &bytesMonitor, memOpts)

// 	monitor := Monitor{
// 		db,
// 		md,
// 		qmc,
// 	}

// 	return &monitor
// }

// func (m *Monitor) Query() {

// 	go func() {
// 		// time.Sleep(time.Second * 10)
// 		now := timeutil.Now().UnixNano()
// 		// Get nearest 10s interval in the past
// 		now = now - (now % 6e+10)

// 		sKey := now - 6e+10
// 		eKey := now

// 		fmt.Println("sKey", sKey)

// 		var randMetric string
// 		isCounter := false
// 		// randMetric = "cr.node.sys.uptime"
// 		for _, v := range m.metadata {
// 			randMetric = v.TimeseriesPrefix + v.Name
// 			if v.MetricType == prometheusgo.MetricType_COUNTER {
// 				isCounter = true
// 			}
// 		}

// 		tsri := timeSeriesResolutionInfo{
// 			randMetric,
// 			Resolution10s,
// 		}

// 		targetSpan := roachpb.Span{
// 			Key: MakeDataKey(randMetric, "1" /* source */, Resolution10s, sKey),
// 			EndKey: MakeDataKey(
// 				randMetric, "1" /* source */, Resolution10s, eKey,
// 			),
// 		}

// 		qts := QueryTimespan{sKey, eKey, now, Resolution10s.SampleDuration()}

// 		res1, err := m.db.rollupQuery(
// 			context.TODO(),
// 			tsri,
// 			targetSpan,
// 			Resolution10s,
// 			m.mem,
// 			qts,
// 			isCounter,
// 		)

// 		if err != nil {
// 			fmt.Println(err)
// 		}

// 		fmt.Println("res1", res1)
// 	}()
// }
