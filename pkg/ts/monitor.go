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
	valid  bool
	rollup rollupDatapoint
}

type Level struct {
	res                   RollupRes
	dataRing              *ring.Ring
	bufferLen             int
	compressionPeriod     int // bufferLen + 1; disabled by 0
	currentRingPos        *ring.Ring
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
					levels:   [4]Level{OneMinuteLevel, TenMinuteLevel, OneHourLevel, OneDayLevel},
					metadata: metadata,
				}

				for i, level := range newMetric.levels {
					newMetric.levels[i].dataRing = ring.New(level.bufferLen)
					newMetric.levels[i].currentRingPos = newMetric.levels[i].dataRing
				}

				metrics[metadata.TimeseriesPrefix+name+quantile] = &newMetric
			}
		} else {
			newMetric := MetricDetails{
				levels:   [4]Level{OneMinuteLevel, TenMinuteLevel, OneHourLevel, OneDayLevel},
				metadata: metadata,
			}

			for i, level := range newMetric.levels {
				newMetric.levels[i].dataRing = ring.New(level.bufferLen)
				newMetric.levels[i].currentRingPos = newMetric.levels[i].dataRing

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
	for x := range time.Tick(d) {
		fmt.Println(x)
		f()
	}
}

func (m *Monitor) Query() {

	go func() {

		for metricName, metricDeets := range m.metrics {

			thisLevel := &metricDeets.levels[OneM]

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

			tsri := timeSeriesResolutionInfo{
				metricName,
				Resolution10s,
			}

			targetSpan := roachpb.Span{
				Key: MakeDataKey(metricName, "1" /* source */, Resolution10s, sKey),
				EndKey: MakeDataKey(
					metricName, "1" /* source */, Resolution10s, eKey,
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
					valid:  false,
					rollup: rollupDatapoint{timestampNanos: sKey},
				}
			} else {
				dataToAdd = validatedData{
					valid:  true,
					rollup: res.datapoints[0],
				}
			}

			fmt.Printf("%s \n", metricName)
			fmt.Println("\t", dataToAdd.valid)
			fmt.Println("\t", dataToAdd.rollup)

			metricDeets.addDataToLevel(dataToAdd, OneM)
		}
		fmt.Println("----------------------------------------------------------------------")
	}()
}

func (mdt *MetricDetails) addDataToLevel(newData validatedData, level RollupRes) {
	thisLevel := &mdt.levels[level]
	thisLevel.currentRingPos.Value = newData
	// Oldest data is one to the right
	if thisLevel.currentRingPos.Next().Value != nil {
		oldestRingData := thisLevel.currentRingPos.Next().Value.(validatedData)
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
		mdt.compressData(thisLevel)
	}

	thisLevel.mostRecentTSNanos = newData.rollup.timestampNanos
	fmt.Println("\tthisLevel", thisLevel)
	// thisLevel.currentRingPos.Do(func(p interface{}) {
	// 	if p != nil {
	// 		fmt.Println("ring val ", thisLevel.res, p.(validatedData))
	// 	}
	// })
	thisLevel.currentRingPos = thisLevel.currentRingPos.Next()
}

func (mdt *MetricDetails) compressData(level *Level) {
	var datapointsToCompressRing []rollupDatapoint

	ringCursor := level.currentRingPos
	for i := 0; i < level.compressionPeriod; i++ {
		if ringCursor.Value != nil {
			if ringCursor.Value.(validatedData).valid {
				datapointsToCompressRing = append(datapointsToCompressRing, ringCursor.Value.(validatedData).rollup)
			}
		}

		ringCursor = ringCursor.Prev()
	}

	var oldestTimestamp int64
	// ringCursor ends up one past the "end" of the compression period
	if ringCursor.Next().Value != nil {
		oldestTimestamp = ringCursor.Next().Value.(validatedData).rollup.timestampNanos
	}

	var dataToAdd validatedData
	if len(datapointsToCompressRing) > 0 {
		compressedData := compressRollupDatapointsAtTimestamp(datapointsToCompressRing, oldestTimestamp)
		dataToAdd = validatedData{
			valid:  true,
			rollup: compressedData,
		}
	} else {
		dataToAdd = validatedData{
			valid: false,
			rollup: rollupDatapoint{
				timestampNanos: oldestTimestamp,
			},
		}
	}

	nextRes := RollupRes(int(level.res) + 1)
	mdt.addDataToLevel(dataToAdd, nextRes)
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
