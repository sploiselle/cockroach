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
	"container/ring"
	"context"
	"fmt"
	"log"
	"math"
	"regexp"
	"time"

	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/ts/tspb"
	"github.com/cockroachdb/cockroach/pkg/util/metric"
	"github.com/cockroachdb/cockroach/pkg/util/mon"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/montanaflynn/stats"
	prometheusgo "github.com/prometheus/client_model/go"
)

type rollupRes int

const (
	oneM rollupRes = 0
	tenM rollupRes = 1
	oneH rollupRes = 2
	oneD rollupRes = 3
)

type validatedData struct {
	valid  bool
	rollup rollupDatapoint
}

type Level struct {
	res                   rollupRes
	dataRing              *ring.Ring
	bufferLen             int
	compressionPeriod     int // bufferLen + 1; disabled by 0
	currentBufferPos      *ring.Ring
	currentCompressionPos int
	targetRollupCount     int // the target count of each rollup in the ring
	mostRecentTSNanos     int64
}

var oneMinuteLevel = Level{
	res:               oneM,
	bufferLen:         11,
	compressionPeriod: 10,
	targetRollupCount: 6,
}

var tenMinuteLevel = Level{
	res:               tenM,
	bufferLen:         7,
	compressionPeriod: 6,
	targetRollupCount: 6 * 10,
}

var oneHourLevel = Level{
	res:               oneH,
	bufferLen:         25,
	compressionPeriod: 24,
	targetRollupCount: 6 * 10 * 6,
}

var oneDayLevel = Level{
	res:               oneD,
	bufferLen:         8,
	compressionPeriod: 0,
	targetRollupCount: 6 * 10 * 6 * 24,
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

func NewMonitor(db *DB, md map[string]metric.Metadata, st *cluster.Settings, metricRegex string) *Monitor {

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
					levels:   [4]Level{oneMinuteLevel, tenMinuteLevel, oneHourLevel, oneDayLevel},
					metadata: metadata,
				}

				for i, level := range newMetric.levels {
					newMetric.levels[i].dataRing = ring.New(level.bufferLen)
					newMetric.levels[i].currentBufferPos = newMetric.levels[i].dataRing
				}

				metrics[metadata.TimeseriesPrefix+name+quantile] = &newMetric
			}
		} else {
			newMetric := MetricDetails{
				levels:   [4]Level{oneMinuteLevel, tenMinuteLevel, oneHourLevel, oneDayLevel},
				metadata: metadata,
			}

			for i, level := range newMetric.levels {
				newMetric.levels[i].dataRing = ring.New(level.bufferLen)
				newMetric.levels[i].currentBufferPos = newMetric.levels[i].dataRing

			}
			metrics[metadata.TimeseriesPrefix+name] = &newMetric
		}
	}

	for name := range metrics {
		matched, _ := regexp.MatchString(metricRegex, name)
		if !matched {
			delete(metrics, name)
		}
	}

	monitor := Monitor{
		db,
		metrics,
		qmc,
	}

	fmt.Printf("Monitoring %d metrics\n", len(monitor.metrics))

	return &monitor
}

func (m *Monitor) Start() {
	go doEvery(time.Minute, m.Query)
}

func doEvery(d time.Duration, f func()) {
	for _ = range time.Tick(d) {
		f()
	}
}

func (m *Monitor) Query() {

	for metricName, metricDeets := range m.metrics {

		dataToAdd := validatedData{valid: true}

		thisLevel := &metricDeets.levels[oneM]

		var now, sKey, eKey int64
		if thisLevel.mostRecentTSNanos == 0 {
			// time.Sleep(time.Second * 10)
			// dataToAdd.valid = false // this lets us easily skip adding the data for the first period
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

		qts := QueryTimespan{sKey, eKey, now, Resolution10s.SampleDuration()}

		kvs, err := m.db.readAllSourcesFromDatabase(context.TODO(), metricName, Resolution10s, qts)
		if err != nil {
			dataToAdd.valid = false
			fmt.Println(err)
		}

		// Convert result data into a map of source strings to ordered spans of
		// time series data.
		diskAccount := m.mem.workerMonitor.MakeBoundAccount()
		defer diskAccount.Close(context.TODO())
		sourceSpans, err := convertKeysToSpans(context.TODO(), kvs, &diskAccount)

		if err != nil {
			dataToAdd.valid = false
			fmt.Println(err)
		}

		for _, span := range sourceSpans {
			if span[0].StartTimestampNanos == 0 {
				dataToAdd.valid = false
			}
		}

		agg := tspb.TimeSeriesQueryAggregator_AVG
		der := tspb.TimeSeriesQueryDerivative_DERIVATIVE
		noDer := tspb.TimeSeriesQueryDerivative_NONE

		if metricDeets.metadata.MetricType != prometheusgo.MetricType_COUNTER {
			measureDatapoints := make([]tspb.TimeSeriesDatapoint, 0)

			measureQ := tspb.Query{
				metricName,
				&agg,
				&agg,
				&noDer,
				[]string{""},
			}

			aggregateSpansToDatapoints(sourceSpans, measureQ, qts, Resolution10s.SampleDuration(), &measureDatapoints)
			measureDatapoints = trimTimeseriesDatapointSlice(measureDatapoints, qts)
			measureRollup := computeRollupsFromData(tspb.TimeSeriesData{Name: metricName, Source: "", Datapoints: measureDatapoints}, qts.EndNanos-qts.StartNanos)
			fmt.Println("measureRollup", measureRollup)
		}

		derQ := tspb.Query{
			metricName,
			&agg,
			&agg,
			&der,
			[]string{""},
		}

		derDatapoints := make([]tspb.TimeSeriesDatapoint, 0)

		aggregateSpansToDatapoints(sourceSpans, derQ, qts, Resolution10s.SampleDuration(), &derDatapoints)

		if len(derDatapoints) == 0 {
			dataToAdd.valid = false
		}

		derDatapoints = trimTimeseriesDatapointSlice(derDatapoints, qts)

		derRollup := computeRollupsFromData(tspb.TimeSeriesData{Name: metricName, Source: "", Datapoints: derDatapoints}, qts.EndNanos-qts.StartNanos)

		if len(derRollup.datapoints) > 1 {
			compressedRollup := compressRollupDatapoints(derRollup.datapoints)
			derRollup.datapoints = []rollupDatapoint{compressedRollup}
		}

		fmt.Println("ROLLUP", derRollup.datapoints[0])

		// res, err := m.db.rollupQuery(
		// 	context.TODO(),
		// 	tsri,
		// 	targetSpan,
		// 	Resolution10s,
		// 	m.mem,
		// 	qts,
		// 	isCounter,
		// )

		// Invalidate data if it doesn't have at least 80% of its values
		if len(derRollup.datapoints) < 1 {
			dataToAdd = validatedData{
				valid:  false,
				rollup: rollupDatapoint{timestampNanos: sKey},
			}
		} else {
			if derRollup.datapoints[0].count < 5 || !dataToAdd.valid {
				dataToAdd = validatedData{
					valid:  false,
					rollup: rollupDatapoint{timestampNanos: sKey},
				}
			} else {
				dataToAdd = validatedData{
					valid:  true,
					rollup: derRollup.datapoints[0],
				}
			}
		}

		fmt.Printf("%s \n", metricName)
		fmt.Println("\t", dataToAdd.valid)
		fmt.Println("\t", dataToAdd.rollup)

		metricDeets.addDataToLevel(dataToAdd, oneM)
	}
	fmt.Println("----------------------------------------------------------------------")
}

func (mdt *MetricDetails) addDataToLevel(newData validatedData, level rollupRes) {
	thisLevel := &mdt.levels[level]
	thisLevel.currentBufferPos.Value = newData
	// Oldest data is one to the right
	if thisLevel.currentBufferPos.Next().Value != nil {
		oldestRingData := thisLevel.currentBufferPos.Next().Value.(validatedData)
		if oldestRingData.valid && newData.valid {
			anovaPass05 := anova(0.05, []rollupDatapoint{newData.rollup, oldestRingData.rollup})
			if !anovaPass05 {
				fmt.Printf(
					"!!!WARNING!!! %s failed between %v and %v\nold: %v\nnew: %v\n",
					mdt.metadata.Name,
					time.Unix(0, oldestRingData.rollup.timestampNanos),
					time.Unix(0, newData.rollup.timestampNanos),
					oldestRingData.rollup,
					newData.rollup,
				)
			}
		}
	}

	thisLevel.currentCompressionPos = (thisLevel.currentCompressionPos + 1) % thisLevel.compressionPeriod
	if thisLevel.currentCompressionPos == 0 && thisLevel.compressionPeriod != 0 {
		dataToAdd := mdt.compressData(thisLevel)

		// If not 80% of this period's data, then invalidate
		if dataToAdd.rollup.count < uint32(0.8*float32(thisLevel.compressionPeriod*thisLevel.targetRollupCount)) {
			dataToAdd.valid = false
			dataToAdd.rollup = rollupDatapoint{}
		}
		nextRes := rollupRes(int(level) + 1)
		mdt.addDataToLevel(dataToAdd, nextRes)
	}

	thisLevel.mostRecentTSNanos = newData.rollup.timestampNanos
	fmt.Println("\tthisLevel", thisLevel)
	// thisLevel.currentBufferPos.Do(func(p interface{}) {
	// 	if p != nil {
	// 		fmt.Println("ring val ", thisLevel.res, p.(validatedData))
	// 	}
	// })
	thisLevel.currentBufferPos = thisLevel.currentBufferPos.Next()
}

func (mdt *MetricDetails) compressData(level *Level) validatedData {
	var datapointsToCompress []rollupDatapoint

	ringCursor := level.currentBufferPos
	for i := 0; i < level.compressionPeriod; i++ {
		if ringCursor.Value != nil {
			if ringCursor.Value.(validatedData).valid {
				datapointsToCompress = append(datapointsToCompress, ringCursor.Value.(validatedData).rollup)
			}
		}

		ringCursor = ringCursor.Prev()
	}

	var dataToAdd validatedData
	if len(datapointsToCompress) > 0 {
		compressedData := compressRollupDatapoints(datapointsToCompress)
		dataToAdd = validatedData{
			valid:  true,
			rollup: compressedData,
		}
	} else {

		var oldestTimestamp int64
		// ringCursor ends up one past the "end" of the compression period
		if ringCursor.Next().Value != nil {
			oldestTimestamp = ringCursor.Next().Value.(validatedData).rollup.timestampNanos
		}

		dataToAdd = validatedData{
			valid: false,
			rollup: rollupDatapoint{
				timestampNanos: oldestTimestamp,
			},
		}
	}

	return dataToAdd
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
	fmt.Printf("anova between %v", time.Unix(0, stats[0].timestampNanos))

	anovaElements := make([]anovaElement, len(stats))

	for i, stat := range stats {
		if i > 0 {
			fmt.Printf(" and %v\n", time.Unix(0, stat.timestampNanos))
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
