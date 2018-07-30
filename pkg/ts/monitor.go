// Copyright 2018 The Cockroach Authors.
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

// QUESTIONS
// How to identify sources?
// How to make QueryMemoryContext?
// How to bring in context?

package ts

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/cockroachdb/cockroach/pkg/util/mon"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/util/metric"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
)

type Monitor struct {
	db       *DB
	metadata map[string]metric.Metadata
	mem      QueryMemoryContext
}

func NewMonitor(db *DB, md map[string]metric.Metadata, st *cluster.Settings) *Monitor {

	bytesMonitor := mon.MakeMonitor("timeseries monitor", mon.MemoryResource, nil, nil, 0, 100*1024*1024, st)

	memOpts := QueryMemoryOptions{
		// Large budget, but not maximum to avoid overflows.
		BudgetBytes:             math.MaxInt64,
		EstimatedSources:        1, // Not needed for rollups
		InterpolationLimitNanos: 0,
		Columnar:                db.WriteColumnar(),
	}

	qmc := MakeQueryMemoryContext(&bytesMonitor, &bytesMonitor, memOpts)

	monitor := Monitor{
		db,
		md,
		qmc,
	}

	return &monitor
}

func (m *Monitor) Query() {

	go func() {
		time.Sleep(61 * time.Second)
		eKey := timeutil.Now().UnixNano()
		fmt.Println(eKey)

		var randMetric string

		for k := range m.metadata {
			randMetric = k
		}

		tsri := timeSeriesResolutionInfo{
			randMetric,
			Resolution10s,
		}

		thresholds := m.db.computeThresholds(eKey)
		threshold := thresholds[Resolution10s]

		targetSpan := roachpb.Span{
			Key: MakeDataKey(randMetric, "" /* source */, Resolution10s, 0),
			EndKey: MakeDataKey(
				randMetric, "" /* source */, Resolution10s, threshold,
			),
		}

		rollupDataMap := make(map[string]rollupData)

		fmt.Println("randMetric", randMetric)

		span, err := m.db.queryAndComputeRollupsForSpan(
			context.TODO(),
			tsri,
			targetSpan,
			Resolution10s,
			rollupDataMap,
			m.mem,
		)

		fmt.Println("span", span)
		fmt.Println("err", err)

		fmt.Println("rollupDataMap", rollupDataMap)
	}()
}
