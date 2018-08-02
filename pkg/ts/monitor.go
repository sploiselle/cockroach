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

	"github.com/cockroachdb/cockroach/pkg/util/mon"
	prometheusgo "github.com/prometheus/client_model/go"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/util/metric"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
)

type RollupRes int

const (
	OneM RollupRes = 0
	TenM RollupRes = 1
	OneH RollupRes = 2
	OneD RollupRes = 3
)

type Level struct {
	res                   RollupRes
	data                  []rollupDatapoint
	bufferLen             int
	currentBufferPos      int
	compressionPeriod     int // bufferLen + 1; disabled by 0
	currentCompressionPos int
}

var OneMinuteLevel = Level{
	res:               OneM,
	bufferLen:         7,
	compressionPeriod: 6,
}

var TenMinuteLevel = Level{
	res:               TenM,
	bufferLen:         11,
	compressionPeriod: 10,
}

var OneHourLevel = Level{
	res:               OneH,
	bufferLen:         25,
	compressionPeriod: 24,
}

var OneDayLevel = Level{
	res:               OneD,
	bufferLen:         8,
	compressionPeriod: 0,
}

type MetricDetails struct {
	levels   [4]Level
	metadata metric.Metadata
}

type Monitor struct {
	db      *DB
	metrics map[string]*MetricDetails
	mem     QueryMemoryContext
}

func NewMonitor(db *DB, md map[string]metric.Metadata, st *cluster.Settings) *Monitor {

	bytesMonitor := mon.MakeMonitor("timeseries monitor", mon.MemoryResource, nil, nil, 0, 100*1024*1024, st)
	bytesMonitor.Start(context.TODO(), nil, mon.MakeStandaloneBudget(100*1024*1024))
	bytesMonitor.MakeBoundAccount()

	memOpts := QueryMemoryOptions{
		// Large budget, but not maximum to avoid overflows.
		BudgetBytes:             math.MaxInt64,
		EstimatedSources:        1, // Not needed for rollups
		InterpolationLimitNanos: 0,
		Columnar:                db.WriteColumnar(),
	}

	qmc := MakeQueryMemoryContext(&bytesMonitor, &bytesMonitor, memOpts)

	metrics := make(map[string]*MetricDetails)

	for k, v := range md {
		newMetric := MetricDetails{
			levels:   [4]Level{OneDayLevel, TenMinuteLevel, OneHourLevel, OneDayLevel},
			metadata: v,
		}

		for i, level := range newMetric.levels {
			newMetric.levels[i].data = make([]rollupDatapoint, level.bufferLen)
			// fmt.Println("level.bufferLen", level.bufferLen)
			// fmt.Println("newMetric.levels[i].data", newMetric.levels[i].data)

		}
		fmt.Println("v.TimeseriesPrefix + k", v.TimeseriesPrefix+k)
		metrics[v.TimeseriesPrefix+k] = &newMetric
	}

	monitor := Monitor{
		db,
		metrics,
		qmc,
	}

	return &monitor
}

func (m *Monitor) Query() {
	fmt.Println(m.metrics)
	go func() {
		// time.Sleep(time.Second * 10)
		now := timeutil.Now().UnixNano()
		// Get nearest 1m interval in the past
		now = now - (now % 6e+10)

		sKey := now - 6e+10
		eKey := now

		var randMetric string
		isCounter := false
		// randMetric = "cr.node.sys.uptime"
		for k, v := range m.metrics {
			randMetric = k

			if v.metadata.MetricType == prometheusgo.MetricType_COUNTER {
				isCounter = true
			} else if v.metadata.MetricType == prometheusgo.MetricType_HISTOGRAM {
				randMetric += "-p99"
			}
		}

		fmt.Println(randMetric)

		tsri := timeSeriesResolutionInfo{
			randMetric,
			Resolution10s,
		}

		targetSpan := roachpb.Span{
			Key: MakeDataKey(randMetric, "1" /* source */, Resolution10s, sKey),
			EndKey: MakeDataKey(
				randMetric, "1" /* source */, Resolution10s, eKey,
			),
		}

		qts := QueryTimespan{sKey, eKey, now, Resolution10s.SampleDuration()}

		res, err := m.db.rollupQuery(
			context.TODO(),
			tsri,
			targetSpan,
			Resolution10s,
			m.mem,
			qts,
			isCounter,
		)

		if err != nil {
			fmt.Println(err)
		}

		fmt.Println("res", res)

		if len(res.datapoints) > 0 {
			metric := m.metrics[randMetric]
			thisLevel := &metric.levels[OneM]
			thisLevel.data[thisLevel.currentBufferPos] = res.datapoints[0]
			thisLevel.currentBufferPos++
			fmt.Println(thisLevel)
		}
	}()
}
