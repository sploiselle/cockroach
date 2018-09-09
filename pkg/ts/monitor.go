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
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/cockroachdb/cockroach/pkg/gossip"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/ts/tspb"
	"github.com/cockroachdb/cockroach/pkg/util/metric"
	"github.com/cockroachdb/cockroach/pkg/util/mon"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	prometheusgo "github.com/prometheus/client_model/go"
)

type rollupRes int32

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
	nodeID  roachpb.NodeID
	db      *DB
	g       *gossip.Gossip
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

func NewMonitor(
	nodeID roachpb.NodeID,
	db *DB,
	g *gossip.Gossip,
	md map[string]metric.Metadata,
	st *cluster.Settings,
	metricRegex string,
) *Monitor {

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

	metricsToMonitor := make(map[string]*MetricDetails)

	for name, metadata := range md {
		// Histograms are accessible only as 8 separate metrics; one for each quantile.
		// Each must be tracked as a separate key in metricsToMonitor
		if metadata.MetricType == prometheusgo.MetricType_HISTOGRAM {
			for _, quantile := range quantiles {

				matched, _ := regexp.MatchString(metricRegex, metadata.TimeseriesPrefix+name+quantile)
				if matched {
					addMetric(metadata.TimeseriesPrefix+name+quantile, metadata, metricsToMonitor)
				}
			}
		} else {

			matched, _ := regexp.MatchString(metricRegex, metadata.TimeseriesPrefix+name)
			if matched {
				addMetric(metadata.TimeseriesPrefix+name, metadata, metricsToMonitor)
			}
		}
	}

	thisNodeID := strconv.Itoa(int(nodeID))

	// Add all of these metrics to Gossip
	for metricName := range metricsToMonitor {
		gossipMetricKey := gossip.MakeKey("anomaly", metricName, thisNodeID)
		fmt.Println("gossipMetricKey", gossipMetricKey)
		mr := &tspb.MetricRollups{MostRecent: make(map[int32]*tspb.RollupDatapoint)}

		mr.MostRecent[int32(oneM)] = &tspb.RollupDatapoint{}

		err := g.AddInfoProto(gossipMetricKey, mr, 10*time.Minute)

		if err != nil {
			fmt.Printf("when adding %s to gossip, %s", metricName, err)
		}
	}

	monitor := Monitor{
		nodeID,
		db,
		g,
		metricsToMonitor,
		qmc,
	}

	fmt.Printf("Monitoring %d metrics\n", len(monitor.metrics))

	return &monitor
}

func addMetric(metricName string, metadata metric.Metadata, metrics map[string]*MetricDetails) {

	newMetric := MetricDetails{
		levels:   [4]Level{oneMinuteLevel, tenMinuteLevel, oneHourLevel, oneDayLevel},
		metadata: metadata,
	}

	for i, level := range newMetric.levels {
		newMetric.levels[i].dataRing = ring.New(level.bufferLen)
		newMetric.levels[i].currentBufferPos = newMetric.levels[i].dataRing

	}
	metrics[metricName] = &newMetric
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

	for metricName, metricDetails := range m.metrics {

		dataToAdd := validatedData{valid: true}

		thisLevel := &metricDetails.levels[oneM]

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

		kvs, err := m.db.readFromDatabase(context.TODO(), metricName, Resolution10s, qts, []string{strconv.Itoa(int(m.nodeID))})
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

		if metricDetails.metadata.MetricType != prometheusgo.MetricType_COUNTER {
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
			measureRollup := computeRollupsFromData(tspb.TimeSeriesData{Name: metricName, Source: strconv.Itoa(int(m.nodeID)), Datapoints: measureDatapoints}, qts.EndNanos-qts.StartNanos)
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

		derRollup := computeRollupsFromData(tspb.TimeSeriesData{Name: metricName, Source: strconv.Itoa(int(m.nodeID)), Datapoints: derDatapoints}, qts.EndNanos-qts.StartNanos)

		if len(derRollup.datapoints) > 1 {
			compressedRollup := compressRollupDatapoints(derRollup.datapoints)
			derRollup.datapoints = []rollupDatapoint{compressedRollup}
		}

		// Invalidate data if it doesn't have at least 80% of its values
		if len(derRollup.datapoints) < 1 {
			dataToAdd = validatedData{
				valid:  false,
				rollup: rollupDatapoint{timestampNanos: sKey},
			}
		} else {
			// I think this is could be moved up to the preceding if...
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

		m.gossipRollup(metricName, dataToAdd, oneM)
		m.addDataToLevel(metricDetails, dataToAdd, oneM, metricName)
	}
	fmt.Println("----------------------------------------------------------------------")
}

func (m *Monitor) gossipRollup(mn string, newData validatedData, level rollupRes) error {
	rollups := tspb.MetricRollups{}
	rollups.MostRecent = make(map[int32]*tspb.RollupDatapoint)
	thisNodeID := strconv.Itoa(int(m.nodeID))
	writeGossipMetricKey := gossip.MakeKey("anomaly", mn, thisNodeID)
	currentGossipRollups, err := m.g.GetInfo(writeGossipMetricKey)

	if err != nil {
		return err
	}

	if err = protoutil.Unmarshal(currentGossipRollups, &rollups); err != nil {
		return errors.Wrapf(err, "failed to gossip new rollup for %q", mn)
	}

	// lastRollup, ok := rollups.MostRecent[int32(level)]

	// if ok {
	// 	fmt.Println("Time between this data and gossip is", (newData.rollup.timestampNanos-lastRollup.TimestampNanos)/int64(time.Second), "seconds")
	// }

	var dataToGossip tspb.RollupDatapoint

	if newData.valid {
		dataToGossip = tspb.RollupDatapoint{
			TimestampNanos: newData.rollup.timestampNanos,
			First:          newData.rollup.first,
			Last:           newData.rollup.last,
			Min:            newData.rollup.min,
			Max:            newData.rollup.max,
			Sum:            newData.rollup.sum,
			Count:          newData.rollup.count,
			Variance:       newData.rollup.variance,
		}
	}

	rollups.MostRecent[int32(level)] = &dataToGossip

	m.g.AddInfoProto(writeGossipMetricKey, &rollups, 4*time.Minute)

	return nil
}

func (m *Monitor) checkOtherNodes(mn string, level rollupRes, counter int, mostNodesSeen int) error {
	if counter > 5 {
		return nil
	}
	targetTimestamp := m.metrics[mn].levels[int(level)].mostRecentTSNanos

	readGossipRegex := gossip.MakeKey("anomaly", mn)

	var allNodesDataFromSameRes []tspb.NodeRollupDatapoint

	possibleNodes := 0

	if err := m.g.IterateInfos(readGossipRegex, func(key string, info gossip.Info) error {

		fmt.Println("Key?", key)
		possibleNodes++

		nodeIDRegex := regexp.MustCompile(":(\\d+)$")
		keyNodeIDMatches := nodeIDRegex.FindStringSubmatch(key)
		if len(keyNodeIDMatches) > 1 {
			keyNodeID := keyNodeIDMatches[1]

			bytes, err := info.Value.GetBytes()
			if err != nil {
				return errors.Wrapf(err, "failed to extract bytes for key %q", key)
			}

			var r tspb.MetricRollups

			if err := protoutil.Unmarshal(bytes, &r); err != nil {
				return errors.Wrapf(err, "failed to parse value for key %q", key)

			}

			rdp, ok := r.MostRecent[int32(level)]

			if ok {
				if rdp.TimestampNanos == targetTimestamp && rdp.Count > 0 {
					allNodesDataFromSameRes = append(allNodesDataFromSameRes, tspb.NodeRollupDatapoint{NodeID: keyNodeID, Rollup: rdp})
				}
			}
		}

		return nil
	}); err != nil {
		return err
	}

	fmt.Println("GOSSIP ROLLUPS????", allNodesDataFromSameRes)

	// Wait for more Gossips if you don't have 2
	// or if you don't have them all and it looks like more are still coming in
	if len(allNodesDataFromSameRes) < 2 || (len(allNodesDataFromSameRes) < possibleNodes && len(allNodesDataFromSameRes) > mostNodesSeen) {
		go func() {
			fmt.Println("Waiting for better data to come into gossip")
			time.Sleep(5 * time.Second)
			counter++
			m.checkOtherNodes(mn, level, counter, len(allNodesDataFromSameRes))
		}()
	} else {

		var rollupsToProcess []rollupDatapoint

		for _, d := range allNodesDataFromSameRes {
			rollupsToProcess = append(rollupsToProcess, converttspbRollup(*d.Rollup))
		}

		log.Printf("Checking %d nodes' %s stats", len(rollupsToProcess), mn)

		anovaRollups := sigDifInRollupGroup(rollupsToProcess)

		if len(anovaRollups) > 1 {
			anovaPass05 := passesAnova(0.05, anovaRollups)

			if !anovaPass05 {
				log.Printf(
					"!!!WARNING!!! %d nodes have different %s readings at %d\n",
					len(allNodesDataFromSameRes),
					mn,
					time.Unix(0, targetTimestamp),
				)
			}
		}
	}

	return nil
}

func sigDifInRollupGroup(rollups []rollupDatapoint) []rollupDatapoint {
	var sigDifRollups []rollupDatapoint
	var sigDifRollupIndexes []int
	m := map[int]bool{}

	for i := 0; i < len(rollups); i++ {
		for j := i + 1; j < len(rollups); j++ {
			insigDif := passSigDif(0.05, []rollupDatapoint{rollups[i], rollups[j]})
			if !insigDif {
				sigDifRollupIndexes = append(sigDifRollupIndexes, i)
				sigDifRollupIndexes = append(sigDifRollupIndexes, j)
			}
		}
	}

	for _, v := range sigDifRollupIndexes {
		if _, seen := m[v]; !seen {
			sigDifRollups = append(sigDifRollups, rollups[v])
			m[v] = true
		}
	}
	return sigDifRollups
}

func converttspbRollup(t tspb.RollupDatapoint) rollupDatapoint {
	return rollupDatapoint{
		timestampNanos: t.TimestampNanos,
		first:          t.First,
		last:           t.Last,
		min:            t.Min,
		max:            t.Max,
		sum:            t.Sum,
		count:          t.Count,
		variance:       t.Variance,
	}
}

func (m *Monitor) addDataToLevel(mdt *MetricDetails, newData validatedData, level rollupRes, mn string) {

	thisLevel := &mdt.levels[level]
	thisLevel.currentBufferPos.Value = newData

	// Oldest data is one to the right
	if thisLevel.currentBufferPos.Next().Value != nil {
		oldestRingData := thisLevel.currentBufferPos.Next().Value.(validatedData)
		if oldestRingData.valid && newData.valid {
			dataToTest := []rollupDatapoint{newData.rollup, oldestRingData.rollup}
			insigDif := passSigDif(0.05, dataToTest)
			if !insigDif {
				anovaPass05 := passesAnova(0.05, dataToTest)
				if !anovaPass05 {
					m.checkOtherNodes(mn, level, 0, 0)
				}
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
		m.gossipRollup(mn, dataToAdd, nextRes)
		m.addDataToLevel(mdt, dataToAdd, nextRes, mn)
		m.checkOtherNodes(mn, nextRes, 0, 0)
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

func passSigDif(fp float64, stats []rollupDatapoint) bool {
	var zTarget float64
	if fp == 0.05 {
		zTarget = 1.645
	} else {
		zTarget = 2.33
	}
	firstMeasure := stats[0]
	secondMeasure := stats[1]

	stdDev := math.Sqrt(firstMeasure.variance)

	firstMean := firstMeasure.sum / float64(firstMeasure.count)
	secondMean := secondMeasure.sum / float64(secondMeasure.count)

	z := math.Abs((firstMean - secondMean) / stdDev)
	if math.IsNaN(z) {
		stdDev = math.Sqrt(secondMeasure.variance)
		z = math.Abs((firstMean - secondMean) / stdDev)
	}

	if math.IsNaN(z) {
		return false
	}

	return z < zTarget
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

func passesAnova(fp float32, stats []rollupDatapoint) bool {

	fmt.Printf("anova between %v", time.Unix(0, stats[0].timestampNanos))

	subsetsStats := make([]anovaElement, len(stats))

	for i, stat := range stats {
		if i > 0 {
			fmt.Printf(" and %v\n", time.Unix(0, stat.timestampNanos))
		}
		subsetsStats[i] = anovaElement{
			g:    1,
			k:    stat.count,
			mean: stat.sum / float64(stat.count),
			vari: stat.variance,
			ssw:  stat.variance * float64(stat.count-1),
			ssb:  0,
		}
	}

	ssStats := calculateSuperSet(subsetsStats)
	// fmt.Println(ssStats)

	// Total Sum of Squares
	sst := anovaRow{
		//
		ssStats.vari * float64(ssStats.k-1),
		float64(ssStats.k - 1),
	}

	ssw := anovaRow{
		ssStats.ssw,
		float64(ssStats.k - ssStats.g),
	}

	ssb := anovaRow{
		sst.val - ssw.val,
		float64(ssStats.g - 1),
	}

	msb := ssb.val / ssb.df
	msw := ssw.val / ssw.df

	// fmt.Println("msb", msb)
	// fmt.Println("msw", msw)

	if msw == 0 {
		msw = 1
	}
	fResult := msb / msw

	// fmt.Println("fResult", fResult)

	if fp == 0.05 {
		an05row, ok := F05[int(ssw.df)]
		if !ok {
			an05row = F05[200]
		}

		an05val := an05row[int(ssb.df)-1]

		// fmt.Println("an05val", an05val)

		if fResult > an05val {
			if len(stats) < 3 {
				return false
			}
		}
	} else {
		an01row, ok := F01[int(ssw.df)]
		if !ok {
			an01row = F01[200]
		}

		an01val := an01row[int(ssb.df)-1]

		if fResult > an01val {
			return false
			// return tukey(0.01, msw, int(ssw.df), int(ssb.df), ns)
		}
	}

	return true
}

func calculateSuperSet(ae []anovaElement) anovaElement {
	g := len(ae)

	var grandSum float64
	var grandCount int

	for i := 0; i < g; i++ {
		grandSum += ae[i].mean * float64(ae[i].k)
		grandCount += int(ae[i].k)
	}

	grandMean := grandSum / float64(grandCount)

	var sumVar, ssSSW, ssSSB float64

	for _, v := range ae {
		v.ssb = float64(v.k) * math.Pow(v.mean-grandMean, 2)
		sumVar += v.vari
		ssSSW += v.ssw
		ssSSB += v.ssb
	}

	var anotherSumVar, sumSquares float64

	for i := 0; i < g; i++ {
		anotherSumVar += float64(ae[i].k-1) * ae[i].vari
		sumSquares += float64(ae[i].k) * math.Pow(ae[i].mean-grandMean, 2)
	}

	ssVar := (anotherSumVar + sumSquares) / float64(grandCount-1)

	// varMean := stat.Variance(allMeans, nil)

	// ssVar := ((kFloat - 1) / (gFloat*kFloat - 1)) * (sumVar + varMean*(kFloat*(gFloat-1)/(kFloat-1)))

	ssStats := anovaElement{
		g:    uint32(g),
		k:    uint32(grandCount),
		mean: grandMean,
		vari: ssVar,
		ssw:  ssSSW,
		ssb:  ssSSB,
	}

	return ssStats
}
