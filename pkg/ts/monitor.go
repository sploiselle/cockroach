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

// TODO:
// Propagate node ID all the way through Tukey analysis

package ts

import (
	"context"
	"fmt"
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

// type rollupRes int32

// const (
// 	oneM             rollupRes = 0
// 	tenM             rollupRes = 1
// 	oneH             rollupRes = 2
// 	twelveH          rollupRes = 3
// 	oneD             rollupRes = 4
// 	oneW             rollupRes = 5
// 	rollupResStopper rollupRes = 6
// )

type levelDetails struct {
	res tspb.RollupRes
	// childrenToRollup   int
	// childrenSkipFactor int
	targetRollupCount int
	rollupForParent   int
	parentSkipFactor  int
}

var levels = map[tspb.RollupRes]levelDetails{
	tspb.RollupRes_oneM: levelDetails{
		res:               tspb.RollupRes_oneM,
		targetRollupCount: 6,
		rollupForParent:   10,
		parentSkipFactor:  1,
	},
	tspb.RollupRes_tenM: levelDetails{
		res: tspb.RollupRes_tenM,
		// childrenToRollup:   10,
		// childrenSkipFactor: 1,
		targetRollupCount: 6 * 10,
		rollupForParent:   6,
		parentSkipFactor:  10,
	},
	tspb.RollupRes_oneH: levelDetails{
		res: tspb.RollupRes_oneH,
		// childrenToRollup:   6,
		// childrenSkipFactor: 10,
		targetRollupCount: 6 * 10 * 6,
		rollupForParent:   12,
		parentSkipFactor:  6 * 10,
	},
	tspb.RollupRes_twelveH: levelDetails{
		res: tspb.RollupRes_twelveH,
		// childrenToRollup:   12,
		// childrenSkipFactor: 10 * 6,
		targetRollupCount: 6 * 10 * 6 * 12,
		rollupForParent:   2,
		parentSkipFactor:  6 * 10 * 12,
	},
	tspb.RollupRes_oneD: levelDetails{
		res: tspb.RollupRes_oneD,
		// childrenToRollup:   2,
		// childrenSkipFactor: 10 * 6 * 12,
		targetRollupCount: 6 * 10 * 6 * 12 * 2,
		rollupForParent:   7,
		parentSkipFactor:  10 * 6 * 12 * 2,
	},
	tspb.RollupRes_oneW: levelDetails{
		res: tspb.RollupRes_oneW,
		// childrenToRollup:   2,
		// childrenSkipFactor: 10 * 6 * 12 * 2,
		targetRollupCount: 6 * 10 * 6 * 12 * 2,
	},
}

// var oneMinuteLevelDetails = levelDetails{
// 	res:               tspb.RollupRes_oneM,
// 	targetRollupCount: 6,
// }

// var tenMinuteLevellDetails = levelDetails{
// 	res:                tspb.RollupRes_tenM,
// 	childrenToRollup:   10,
// 	childrenSkipFactor: 1,
// 	targetRollupCount:  6 * 10,
// }

// var oneHourLevellDetails = levelDetails{
// 	res:                tspb.RollupRes_oneH,
// 	childrenToRollup:   6,
// 	childrenSkipFactor: 10,
// 	targetRollupCount:  6 * 10 * 6,
// }

// var twelveHourLevellDetails = levelDetails{
// 	res:                tspb.RollupRes_twelveH,
// 	childrenToRollup:   12,
// 	childrenSkipFactor: 10 * 6,
// 	targetRollupCount:  6 * 10 * 6 * 12,
// }

// var oneDayLevellDetails = levelDetails{
// 	res:                tspb.RollupRes_oneD,
// 	childrenToRollup:   2,
// 	childrenSkipFactor: 10 * 6 * 12,
// 	targetRollupCount:  6 * 10 * 6 * 12 * 2,
// }

// var oneWeekLevellDetails = levelDetails{
// 	res:                tspb.RollupRes_oneW,
// 	childrenToRollup:   2,
// 	childrenSkipFactor: 10 * 6 * 12 * 2,
// 	targetRollupCount:  6 * 10 * 6 * 12 * 2,
// }

const numOfCols int = 10080

// 10080 rows; one for each minute in the week
type rollupTable [numOfCols]tspb.RollupCol

type FVal struct {
	Observed     float64
	Target       float64
	AtResolution tspb.RollupRes
}

type FValTable [numOfCols]FVal

type MetricDetails struct {
	rollups           rollupTable
	FVals             FValTable
	mostRecentTSNanos int64
	currentTablePos   int
	metadata          metric.Metadata
}

type Monitor struct {
	nodeID  roachpb.NodeID
	db      *DB
	g       *gossip.Gossip
	Metrics map[string]*MetricDetails
	mem     QueryMemoryContext
}

type validatedData struct {
	valid  bool
	rollup tspb.RollupDatapoint
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

	monitor := Monitor{
		nodeID,
		db,
		g,
		make(map[string]*MetricDetails),
		qmc,
	}

	for name, metadata := range md {
		// Histograms are accessible only as 8 separate metrics; one for each quantile.
		// Each must be tracked as a separate key in metricsToMonitor
		if metadata.MetricType == prometheusgo.MetricType_HISTOGRAM {
			for _, quantile := range quantiles {

				matched, _ := regexp.MatchString(metricRegex, metadata.TimeseriesPrefix+name+quantile)
				if matched {
					monitor.addMetric(metadata.TimeseriesPrefix+name+quantile, metadata)
				}
			}
		} else {

			matched, _ := regexp.MatchString(metricRegex, metadata.TimeseriesPrefix+name)
			if matched {
				monitor.addMetric(metadata.TimeseriesPrefix+name, metadata)
			}
		}
	}

	thisNodeID := strconv.Itoa(int(nodeID))

	// Add all of these metrics to Gossip
	for metricName := range monitor.Metrics {
		gossipMetricKey := gossip.MakeKey("anomaly", metricName, thisNodeID)
		fmt.Println("gossipMetricKey", gossipMetricKey)

		err := g.AddInfoProto(gossipMetricKey, &tspb.RollupCol{}, 2*time.Minute)

		if err != nil {
			fmt.Printf("when adding %s to gossip, %s", metricName, err)
		}
	}

	fmt.Printf("Monitoring %d metrics\n", len(monitor.Metrics))

	return &monitor
}

func (m Monitor) addMetric(metricName string, metadata metric.Metadata) {

	newMetric := MetricDetails{
		metadata: metadata,
	}

	m.Metrics[metricName] = &newMetric
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

	for metricName, metricDetails := range m.Metrics {

		dataToAdd := validatedData{valid: true}

		// currentLevelDetails := levels[tspb.RollupRes_oneM]

		var now, sKey, eKey int64
		if metricDetails.mostRecentTSNanos == 0 {
			// time.Sleep(time.Second * 10)
			// dataToAdd.valid = false // this lets us easily skip adding the data for the first period
			now = timeutil.Now().UnixNano()
			// Get nearest 1m interval in the past
			now = now - (now % OneMNanos)

			sKey = now - OneMNanos
			metricDetails.mostRecentTSNanos = sKey
			eKey = now
		} else {
			sKey = metricDetails.mostRecentTSNanos + OneMNanos
			metricDetails.mostRecentTSNanos = sKey
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

		if metricDetails.metadata.MetricType != prometheusgo.MetricType_COUNTER {
			measureDatapoints := make([]tspb.TimeSeriesDatapoint, 0)

			measureQ := tspb.Query{
				Name:             metricName,
				Downsampler:      tspb.TimeSeriesQueryAggregator_AVG.Enum(),
				SourceAggregator: tspb.TimeSeriesQueryAggregator_SUM.Enum(),
				Derivative:       tspb.TimeSeriesQueryDerivative_NONE.Enum(),
			}

			aggregateSpansToDatapoints(sourceSpans, measureQ, qts, Resolution10s.SampleDuration(), &measureDatapoints)
			measureDatapoints = trimTimeseriesDatapointSlice(measureDatapoints, qts)
			measureRollup := computeRollupsFromData(tspb.TimeSeriesData{Name: metricName, Source: strconv.Itoa(int(m.nodeID)), Datapoints: measureDatapoints}, qts.EndNanos-qts.StartNanos)
			fmt.Println("measureRollup", measureRollup)
		}

		derQ := tspb.Query{
			Name:             metricName,
			Downsampler:      tspb.TimeSeriesQueryAggregator_AVG.Enum(),
			SourceAggregator: tspb.TimeSeriesQueryAggregator_SUM.Enum(),
			Derivative:       tspb.TimeSeriesQueryDerivative_NON_NEGATIVE_DERIVATIVE.Enum(),
		}

		derDatapoints := make([]tspb.TimeSeriesDatapoint, 0)

		aggregateSpansToDatapoints(sourceSpans, derQ, qts, Resolution10s.SampleDuration(), &derDatapoints)

		if len(derDatapoints) == 0 {
			dataToAdd.valid = false
		}

		derDatapoints = trimTimeseriesDatapointSlice(derDatapoints, qts)

		derRollup := computeRollupsFromData(tspb.TimeSeriesData{Name: metricName, Source: strconv.Itoa(int(m.nodeID)), Datapoints: derDatapoints}, qts.EndNanos-qts.StartNanos)

		var dpToCompress []tspb.RollupDatapoint

		// Convert from rollup to tspb.RollupDatapoint
		for _, dp := range derRollup.datapoints {
			dpToCompress = append(dpToCompress, convertMetricRollupToTspbRollup(dp))
		}

		// var finalDatapoint tspb.RollupDatapoint

		if len(dpToCompress) > 1 {
			dataToAdd.rollup = compressRollupDatapoints(dpToCompress)
		} else if len(dpToCompress) == 0 {
			dataToAdd.valid = false
		} else {
			dataToAdd.rollup = dpToCompress[0]
		}

		fmt.Printf("%s \n", metricName)
		fmt.Println("\t", dataToAdd.valid)
		fmt.Println("\t", dataToAdd.rollup)

		// m.gossipRollup(metricName, dataToAdd, oneM)
		metricDetails.rollups[metricDetails.currentTablePos].TimestampNanos = metricDetails.mostRecentTSNanos
		m.addDataToLevel(metricDetails, dataToAdd, tspb.RollupRes_oneM)
		m.gossipRollup(metricDetails, metricName)
		m.checkOtherNodes(metricName)
	}
	fmt.Println("----------------------------------------------------------------------")
}

func (m *Monitor) addDataToLevel(mdt *MetricDetails, newData validatedData, level tspb.RollupRes) {
	if level == tspb.RollupRes_stopper {
		return
	}

	cl, ok := levels[level]
	if !ok {
		fmt.Println("addDataToLevel invalid level")
	}

	if newData.valid && newData.rollup.Count >= uint32(float32(cl.targetRollupCount)*.8) {
		mdt.rollups[mdt.currentTablePos].Values[level] = newData.rollup
	} else {
		mdt.rollups[mdt.currentTablePos].Values[level] = tspb.RollupDatapoint{}
	}

	if cl.rollupForParent != 0 {
		var dataToCompress []tspb.RollupDatapoint

		for i := 0; i < cl.rollupForParent; i++ {
			dataToCompress = append(dataToCompress, mdt.rollups[(mdt.currentTablePos-i*cl.parentSkipFactor)%numOfCols].Values[level])
		}
		compressedData := compressRollupDatapoints(dataToCompress)
		m.addDataToLevel(mdt, validatedData{true, compressedData}, level+1)
	}
}

func (m *Monitor) gossipRollup(mdt *MetricDetails, mn string) error {
	// rollups := tspb.MetricRollups{}
	// rollups.MostRecent = make(map[int32]*tspb.RollupDatapoint)
	thisNodeID := strconv.Itoa(int(m.nodeID))
	writeGossipMetricKey := gossip.MakeKey("anomaly", mn, thisNodeID)
	currentGossipRollups, err := m.g.GetInfo(writeGossipMetricKey)

	if err != nil {
		return err
	}

	dataToGossip := mdt.rollups[mdt.currentTablePos]

	if err = protoutil.Unmarshal(currentGossipRollups, &dataToGossip); err != nil {
		return errors.Wrapf(err, "failed to create Gossip value %q", mn)
	}

	// lastRollup, ok := rollups.MostRecent[int32(level)]

	// if ok {
	// 	fmt.Println("Time between this data and gossip is", (newData.rollup.timestampNanos-lastRollup.TimestampNanos)/int64(time.Second), "seconds")
	// }

	// var dataToGossip tspb.RollupDatapoint

	// if newData.valid {
	// 	dataToGossip = tspb.RollupDatapoint{
	// 		TimestampNanos: newData.rollup.timestampNanos,
	// 		First:          newData.rollup.first,
	// 		Last:           newData.rollup.last,
	// 		Min:            newData.rollup.min,
	// 		Max:            newData.rollup.max,
	// 		Sum:            newData.rollup.sum,
	// 		Count:          newData.rollup.count,
	// 		Variance:       newData.rollup.variance,
	// 	}
	// }

	// rollups.MostRecent[int32(level)] = &dataToGossip

	m.g.AddInfoProto(writeGossipMetricKey, &dataToGossip, 2*time.Minute)

	return nil
}

func (m *Monitor) checkOtherNodes(mn string) error {

	targetTimestamp := m.Metrics[mn].mostRecentTSNanos

	readGossipRegex := gossip.MakeKey("anomaly", mn)

	// var allNodesDataFromSameRes []tspb.NodeRollupDatapoint

	// map[nodeID]tspb.RollupCol
	allNodesDataFromSameTimestamp := make(map[string]tspb.RollupCol)

	possibleNodes := 0
	iterations := 0

	// Aggregate all rollup columns you can
	for {
		possibleNodes = 0

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

				var r tspb.RollupCol

				if err := protoutil.Unmarshal(bytes, &r); err != nil {
					return errors.Wrapf(err, "failed to parse value for key %q", key)

				}

				if r.TimestampNanos == targetTimestamp {
					allNodesDataFromSameTimestamp[keyNodeID] = r
				}
			}

			return nil
		}); err != nil {
			return err
		}

		iterations++

		// Iterate over all of the Gossipped info to find what you need until:
		// You've checked 4 times
		// You have all of the possible nodes' worth of data
		if iterations < 5 || len(allNodesDataFromSameTimestamp) < possibleNodes {
			// wait 5 seconds and repeat
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}

	fmt.Println("GOSSIP ROLLUPS????", allNodesDataFromSameTimestamp)

	// TODO! NEED TO CALL COMPARE ROLLUPS
	currentPos := m.Metrics[mn].currentTablePos
	m.Metrics[mn].FVals[currentPos] = compareRollupCols(allNodesDataFromSameTimestamp, mn)

	return nil
}

// This is a good place to return the F-value we want to write
func compareRollupCols(cols map[string]tspb.RollupCol, mn string) FVal {

	fValsAtEachRes := make(map[tspb.RollupRes]FVal)

	// [resoltuion][rollupdatDataPoint from node]
	dpsFromEachRes := make([][]rollupDatapoint, tspb.RollupRes_stopper)

	for i := range dpsFromEachRes {
		dpsFromEachRes[i] = make([]rollupDatapoint, len(cols))
	}

	for _, col := range cols {
		for res, val := range col.Values {
			dpsFromEachRes[res] = append(dpsFromEachRes[res], converttspbRollupToMetricRollup(val))
		}
	}

	var fValToWrite FVal
	// RANGE OVER THE LEFT DIM OF THE TABLE
	for res, rollupsToProcess := range dpsFromEachRes {
		fValToWrite = anova(0.05, rollupsToProcess)
		fValToWrite.AtResolution = tspb.RollupRes(res)
		fValsAtEachRes[fValToWrite.AtResolution] = fValToWrite
	}

	var worstFVal FVal

	for res, fval := range fValsAtEachRes {
		if fval.Observed/fval.Target > worstFVal.Observed/worstFVal.Target {
			worstFVal = fval
			worstFVal.AtResolution = res
		}
	}
	return worstFVal

	// // var rollupsToProcess []rollupDatapoint

	// for _, d := range allNodesDataFromSameTimestamp {
	// 	rollupsToProcess = append(rollupsToProcess, converttspbRollupToMetricRollup(*d.Rollup))
	// }

	// log.Printf("Checking %d nodes' %s stats", len(rollupsToProcess), mn)

	// anovaRollups := sigDifInRollupGroup(rollupsToProcess)

	// if len(anovaRollups) > 1 {
	// 	anovaPass05 := passesAnova(0.05, anovaRollups)

	// 	if !anovaPass05 {
	// 		log.Printf(
	// 			"!!!WARNING!!! %d nodes have different %s readings at %d\n",
	// 			len(allNodesDataFromSameRes),
	// 			mn,
	// 			time.Unix(0, targetTimestamp),
	// 		)
	// 	}
	// }

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

func converttspbRollupToMetricRollup(t tspb.RollupDatapoint) rollupDatapoint {
	return rollupDatapoint{
		first:    t.First,
		last:     t.Last,
		min:      t.Min,
		max:      t.Max,
		sum:      t.Sum,
		count:    t.Count,
		variance: t.Variance,
	}
}

func convertMetricRollupToTspbRollup(m rollupDatapoint) tspb.RollupDatapoint {
	return tspb.RollupDatapoint{
		First:    m.first,
		Last:     m.last,
		Min:      m.min,
		Max:      m.max,
		Sum:      m.sum,
		Count:    m.count,
		Variance: m.variance,
	}
}

// func (m *Monitor) insertRollup(mdt *MetricDetails, newData validatedData, level rollupRes, mn string) {
// 	mdt.currentRollupPos.Value.(rollupPerLevel)[oneM] = newData.rollup
// 	generateRollupAtLevel(tenMinuteLevellDetails)
// 	generateRollupAtLevel(oneHourLevellDetails)
// 	generateRollupAtLevel(twelveHourLevellDetails)
// 	generateRollupAtLevel(oneDayLevellDetails)

// }

// func (mdt *MetricDetails) generateRollupAtLevel(ld levelDetails) {
// 	var rollupsToCompress []rollupDatapoint
// 	cursor := mdt.currentRollupPos
// 	for i := 0; i < ld.childrenToRollup; i++ {
// 		rollupsToCompress = append(rollupsToCompress, cursor.Value.(rollupPerLevel)[ld.res-1])
// 		cursor = cursor.Prev()
// 	}
// 	rdp := compressRollupDatapoints(rollupsToCompress)
// 	mdt.currentRollupPos.Value.(rollupPerLevel)[ld.res] = rdp
// }

// func (m *Monitor) gossipAllRollups(mn string) error {
// 	rollups := tspb.MetricRollups{}
// 	rollups.MostRecent = make(map[int32]*tspb.RollupDatapoint)
// 	thisNodeID := strconv.Itoa(int(m.nodeID))
// 	writeGossipMetricKey := gossip.MakeKey("anomalyAll", mn, thisNodeID)
// 	currentGossipRollups, err := m.g.GetInfo(writeGossipMetricKey)

// 	if err != nil {
// 		return err
// 	}

// 	if err = protoutil.Unmarshal(currentGossipRollups, &rollups); err != nil {
// 		return errors.Wrapf(err, "failed to gossip new rollup for %q", mn)
// 	}

// 	var dataToGossip tspb.RollupDatapoint

// 	if newData.valid {
// 		dataToGossip = tspb.RollupDatapoint{
// 			TimestampNanos: newData.rollup.timestampNanos,
// 			First:          newData.rollup.first,
// 			Last:           newData.rollup.last,
// 			Min:            newData.rollup.min,
// 			Max:            newData.rollup.max,
// 			Sum:            newData.rollup.sum,
// 			Count:          newData.rollup.count,
// 			Variance:       newData.rollup.variance,
// 		}
// 	}

// 	rollups.MostRecent[int32(level)] = &dataToGossip

// 	m.g.AddInfoProto(writeGossipMetricKey, &rollups, 4*time.Minute)

// 	return nil
// }

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

// func passesAnova(fp float32, stats []rollupDatapoint) bool {

func anova(fp float32, stats []rollupDatapoint) FVal {

	var retVal FVal
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
	retVal.Observed = msb / msw
	// fResult := msb / msw

	// fmt.Println("fResult", fResult)

	if fp == 0.05 {
		an05row, ok := F05[int(ssw.df)]
		if !ok {
			an05row = F05[200]
		}

		retVal.Target = an05row[int(ssb.df)-1]
		// an05val := an05row[int(ssb.df)-1]

		// fmt.Println("an05val", an05val)

		// if fResult > an05val {
		// 	if len(stats) < 3 {
		// 		return false
		// 	}
		// }
	} else {
		an01row, ok := F01[int(ssw.df)]
		if !ok {
			an01row = F01[200]
		}

		retVal.Target = an01row[int(ssb.df)-1]
		// an01val := an01row[int(ssb.df)-1]

		// if fResult > an01val {
		// 	return false
		// 	// return tukey(0.01, msw, int(ssw.df), int(ssb.df), ns)
		// }
	}

	return retVal
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

// 	m.Metrics[mdt.]

// 	currentLevel := &mdt.levels[level]
// 	currentLevel.currentBufferPos.Value = newData

// 	// Oldest data is one to the right
// 	if currentLevel.currentBufferPos.Next().Value != nil {
// 		oldestRingData := currentLevel.currentBufferPos.Next().Value.(validatedData)
// 		if oldestRingData.valid && newData.valid {
// 			dataToTest := []rollupDatapoint{newData.rollup, oldestRingData.rollup}
// 			insigDif := passSigDif(0.05, dataToTest)
// 			if !insigDif {
// 				anovaPass05 := passesAnova(0.05, dataToTest)
// 				if !anovaPass05 {
// 					m.checkOtherNodes(mn, level, 0, 0)
// 				}
// 			}
// 		}
// 	}

// 	currentLevel.currentCompressionPos = (currentLevel.currentCompressionPos + 1) % currentLevel.compressionPeriod
// 	if currentLevel.currentCompressionPos == 0 && currentLevel.compressionPeriod != 0 {
// 		dataToAdd := mdt.compressData(currentLevel)

// 		// If not 80% of this period's data, then invalidate
// 		if dataToAdd.rollup.count < uint32(0.8*float32(currentLevel.compressionPeriod*currentLevel.targetRollupCount)) {
// 			dataToAdd.valid = false
// 			dataToAdd.rollup = rollupDatapoint{}
// 		}
// 		nextRes := rollupRes(int(level) + 1)
// 		m.gossipRollup(mn, dataToAdd, nextRes)
// 		m.addDataToLevel(mdt, dataToAdd, nextRes, mn)
// 		m.checkOtherNodes(mn, nextRes, 0, 0)
// 	}

// 	currentLevel.mostRecentTSNanos = newData.rollup.timestampNanos
// 	fmt.Println("\tcurrentLevel", currentLevel)
// 	// currentLevel.currentBufferPos.Do(func(p interface{}) {
// 	// 	if p != nil {
// 	// 		fmt.Println("ring val ", currentLevel.res, p.(validatedData))
// 	// 	}
// 	// })

// 	currentLevel.currentBufferPos = currentLevel.currentBufferPos.Next()
// }

// func (mdt *MetricDetails) compressData(level *Level) validatedData {
// 	var datapointsToCompress []rollupDatapoint

// 	ringCursor := level.currentBufferPos
// 	for i := 0; i < level.compressionPeriod; i++ {
// 		if ringCursor.Value != nil {
// 			if ringCursor.Value.(validatedData).valid {
// 				datapointsToCompress = append(datapointsToCompress, ringCursor.Value.(validatedData).rollup)
// 			}
// 		}

// 		ringCursor = ringCursor.Prev()
// 	}

// 	var dataToAdd validatedData
// 	if len(datapointsToCompress) > 0 {
// 		compressedData := compressRollupDatapoints(datapointsToCompress)
// 		dataToAdd = validatedData{
// 			valid:  true,
// 			rollup: compressedData,
// 		}
// 	} else {

// 		var oldestTimestamp int64
// 		// ringCursor ends up one past the "end" of the compression period
// 		if ringCursor.Next().Value != nil {
// 			oldestTimestamp = ringCursor.Next().Value.(validatedData).rollup.timestampNanos
// 		}

// 		dataToAdd = validatedData{
// 			valid: false,
// 			rollup: rollupDatapoint{
// 				timestampNanos: oldestTimestamp,
// 			},
// 		}
// 	}

// 	return dataToAdd
// }
