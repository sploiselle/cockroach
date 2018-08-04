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
	"log"
	"math"
	"time"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/util/metric"
	"github.com/cockroachdb/cockroach/pkg/util/mon"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/montanaflynn/stats"
	prometheusgo "github.com/prometheus/client_model/go"
)

type RollupRes int

const (
	OneM RollupRes = 0
	TenM RollupRes = 1
	OneH RollupRes = 2
	OneD RollupRes = 3
)

type validatedData struct {
	valid bool
	data  rollupDatapoint
}

type Level struct {
	res                   RollupRes
	data                  []validatedData
	bufferLen             int
	currentBufferPos      int
	compressionPeriod     int // bufferLen + 1; disabled by 0
	currentCompressionPos int
	mostRecentTSNanos     int64
}

var OneMinuteLevel = Level{
	res:               OneM,
	bufferLen:         11,
	compressionPeriod: 10,
}

var TenMinuteLevel = Level{
	res:               TenM,
	bufferLen:         7,
	compressionPeriod: 6,
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
	db         *DB
	metrics    map[string]*MetricDetails
	mem        QueryMemoryContext
	randMetric string
}

const (
	OneMNanos int64 = 6e+10
	TenMNanos int64 = 6e+11
	OneHNanos int64 = 3.6e+12
	OneDNanos int64 = 8.64e+13
)

var quantiles = [8]string{
	"-max",
	"-p99.999",
	"-p99.99",
	"-p99.9",
	"-p99",
	"-p90",
	"-p75",
	"-p50",
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

	for name, metadata := range md {
		// Histograms are accessible only as 8 separate metrics; one for each quantile.
		if metadata.MetricType == prometheusgo.MetricType_HISTOGRAM {
			for _, quantile := range quantiles {

				newMetric := MetricDetails{
					levels:   [4]Level{OneMinuteLevel, TenMinuteLevel, OneHourLevel, OneDayLevel},
					metadata: metadata,
				}

				for i, level := range newMetric.levels {
					newMetric.levels[i].data = make([]validatedData, level.bufferLen)
				}

				metrics[metadata.TimeseriesPrefix+name+quantile] = &newMetric
			}
		} else {
			newMetric := MetricDetails{
				levels:   [4]Level{OneMinuteLevel, TenMinuteLevel, OneHourLevel, OneDayLevel},
				metadata: metadata,
			}

			for i, level := range newMetric.levels {
				newMetric.levels[i].data = make([]validatedData, level.bufferLen)

			}
			metrics[metadata.TimeseriesPrefix+name] = &newMetric
		}
	}

	var randMetric string
	// for k := range metrics {
	// 	randMetric = k
	// }
	randMetric = "cr.store.rocksdb.memtable.total-size"

	monitor := Monitor{
		db,
		metrics,
		qmc,
		randMetric,
	}

	return &monitor
}

func (m *Monitor) Start() {
	fmt.Println("m.randMetric", m.randMetric)
	go doEvery(time.Minute, m.Query)
}

func doEvery(d time.Duration, f func()) {
	for x := range time.Tick(d) {
		fmt.Println(x)
		f()
	}
}

func (m *Monitor) Query() {

	go func() {
		metric := m.metrics[m.randMetric]
		thisLevel := &metric.levels[OneM]

		var now, sKey, eKey int64
		if thisLevel.mostRecentTSNanos == 0 {
			// time.Sleep(time.Second * 10)
			now = timeutil.Now().UnixNano()
			// Get nearest 1m interval in the past
			now = now - (now % OneMNanos)

			sKey = now - OneMNanos
			eKey = now
		} else {
			sKey = thisLevel.mostRecentTSNanos + OneMNanos
			now = sKey + OneMNanos
			eKey = now
		}

		isCounter := true
		// isCounter := m.metrics[m.randMetric].metadata.MetricType == prometheusgo.MetricType_COUNTER
		// randMetric = "cr.node.sys.uptime"
		// for k, v := range m.metrics {
		// 	randMetric = k

		// 	if v.metadata.MetricType {
		// 		isCounter = true
		// 	}
		// }

		fmt.Println("m.randMetric", m.randMetric)

		tsri := timeSeriesResolutionInfo{
			m.randMetric,
			Resolution10s,
		}

		targetSpan := roachpb.Span{
			Key: MakeDataKey(m.randMetric, "1" /* source */, Resolution10s, sKey),
			EndKey: MakeDataKey(
				m.randMetric, "1" /* source */, Resolution10s, eKey,
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
		var dataToAdd validatedData

		if err != nil || len(res.datapoints) < 1 {
			fmt.Println(err)
			dataToAdd = validatedData{
				valid: false,
				data:  rollupDatapoint{timestampNanos: sKey},
			}
		} else {
			dataToAdd = validatedData{
				valid: true,
				data:  res.datapoints[0],
			}
		}

		metric.addDataToLevel(dataToAdd, OneM)
	}()
}

func (mdt *MetricDetails) addDataToLevel(data validatedData, level RollupRes) {
	thisLevel := &mdt.levels[level]
	thisLevel.data[thisLevel.currentBufferPos] = data
	// Oldest data is one to the right
	oldestData := thisLevel.data[(thisLevel.currentBufferPos+1)%thisLevel.bufferLen]

	if oldestData.valid && data.valid {
		anovaPass05 := anova(0.05, []rollupDatapoint{data.data, oldestData.data})
		if !anovaPass05 {
			fmt.Printf(
				"!!!WARNING!!! %s failed between %v and %v\nold: &v\nnew: %v\n",
				mdt.metadata.Name,
				time.Unix(0, oldestData.data.timestampNanos),
				time.Unix(0, data.data.timestampNanos),
				oldestData.data,
				data.data,
			)
		}
	} else {
		fmt.Println("No anova")
	}
	thisLevel.currentCompressionPos = (thisLevel.currentCompressionPos + 1) % thisLevel.compressionPeriod
	if thisLevel.currentCompressionPos == 0 && thisLevel.compressionPeriod != 0 {
		mdt.compressData(thisLevel)
	}
	thisLevel.currentBufferPos = (thisLevel.currentBufferPos + 1) % thisLevel.bufferLen
	thisLevel.mostRecentTSNanos = data.data.timestampNanos
	fmt.Println("thisLevel", thisLevel)
}

func (mdt *MetricDetails) compressData(level *Level) {
	period := level.compressionPeriod
	var datapointsToCompress []rollupDatapoint

	oldestIndex := (level.currentBufferPos + level.bufferLen - period + 1) % level.bufferLen
	oldfestTimestamp := level.data[oldestIndex].data.timestampNanos

	for i := 0; i < period; i++ {
		indexPos := (oldestIndex + i) % level.bufferLen
		if level.data[indexPos].valid {
			datapointsToCompress = append(datapointsToCompress, level.data[indexPos].data)
		}
	}
	var dataToAdd validatedData
	if len(datapointsToCompress) > 0 {
		compressedData := compressRollupDatapointsAtTimestamp(datapointsToCompress, oldfestTimestamp)
		dataToAdd = validatedData{
			valid: true,
			data:  compressedData,
		}
	} else {
		dataToAdd = validatedData{
			valid: false,
			data: rollupDatapoint{
				timestampNanos: oldfestTimestamp,
			},
		}
	}
	newLevel := RollupRes(int(level.res) + 1)
	mdt.addDataToLevel(dataToAdd, newLevel)
}

type anovaElement struct {
	g    uint32
	k    uint32
	mean float64
	vari float64
	ssw  float64
	ssb  float64
}

type anovaRow struct {
	val float64
	df  float64
}

type anovaVals struct {
	sst anovaRow
	ssw anovaRow
	ssb anovaRow
}

func anova(fVal float32, stats []rollupDatapoint) bool {
	fmt.Printf("anova between %v", stats[0].timestampNanos)
	anovaElements := make([]anovaElement, len(stats))

	for i, stat := range stats {
		if i > 0 {
			fmt.Printf(" and %v\n", stat.timestampNanos)
		}
		anovaElements[i] = anovaElement{
			g:    1,
			k:    stat.count,
			mean: stat.sum / float64(stat.count),
			vari: stat.variance,
			ssw:  stat.variance * float64(stat.count-1),
			ssb:  0,
		}
	}

	ssStats := calculateSuperSet(anovaElements)

	sst := anovaRow{
		ssStats.vari * float64(ssStats.g*ssStats.k-1),
		float64(ssStats.g*ssStats.k - 1),
	}

	ssw := anovaRow{
		ssStats.ssw,
		float64(ssStats.g * (ssStats.k - 1)),
	}

	ssb := anovaRow{
		sst.val - ssw.val,
		float64(ssStats.g - 1),
	}

	msw := (ssw.val / ssw.df)
	fResult := (ssb.val / ssb.df) / msw

	if fVal == 0.05 {
		an05row, ok := F05[int(ssw.df)]
		if !ok {
			log.Fatal("too many measurements, my guy")
		}

		an05val := an05row[int(ssb.df)-1]

		if fResult > an05val {
			return false
			// return tukey(pt, 0.05, msw, int(ssw.df), int(ssb.df), ns)
		}
	} else {
		an01row, ok := F01[int(ssw.df)]
		if !ok {
			log.Fatal("too many measurements, my guy")
		}

		an01val := an01row[int(ssb.df)-1]

		if fResult > an01val {
			return false
			// return tukey(pt, 0.01, msw, int(ssw.df), int(ssb.df), ns)
		}
	}

	return true
}

func calculateSuperSet(ae []anovaElement) anovaElement {
	k := ae[0].k
	kFloat := float64(k)
	g := len(ae)
	gFloat := float64(g)

	allMeans := make([]float64, g)

	var totalSum float64
	var totalCount uint32

	for i := 0; i < g; i++ {
		totalSum += (ae[i].mean * float64(ae[i].k))
		totalCount += ae[i].k
		allMeans[i] = ae[i].mean
	}
	ssMean := totalSum / float64(totalCount)

	var sumVar, ssSSW, ssSSB float64

	for _, v := range ae {
		v.ssb = kFloat * math.Pow(v.mean-ssMean, 2)
		sumVar += v.vari
		ssSSW += v.ssw
		ssSSB += v.ssb
	}

	varMean, _ := stats.PopulationVariance(allMeans)

	ssVar := ((kFloat - 1) / (gFloat*kFloat - 1)) * (sumVar + varMean*(kFloat*(gFloat-1)/(kFloat-1)))

	ssStats := anovaElement{
		g:    uint32(g),
		k:    k,
		mean: ssMean,
		vari: ssVar,
		ssw:  ssSSW,
		ssb:  ssSSB,
	}

	return ssStats
}
