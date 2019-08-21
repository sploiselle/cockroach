// Copyright 2019 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

import _ from "lodash";
import * as React from "react";
import { connect } from "react-redux";
import { withRouter, WithRouterProps } from "react-router";

import { refreshNodes, healthReducerObj } from "src/redux/apiReducers";
import { nodesSummarySelector, NodesSummary } from "src/redux/nodes";
import { AdminUIState } from "src/redux/state";

import { MetricsDataProvider } from "src/views/shared/containers/metricDataProvider";
import { LineGraph } from "src/views/cluster/components/linegraph";
import TimeScaleDropdown from "src/views/cluster/containers/timescale";
import { Metric, Axis, AxisUnits } from "src/views/shared/components/metricQuery";
import { PageConfig, PageConfigItem } from "src/views/shared/components/pageconfig";

import { SortedTable } from "src/views/shared/components/sortedtable";

import { cockroach, io } from "src/js/protos";

import "./chartCollection.styl";

// axisUnitsMap converts between the enums in cockroach.ts.catalog and those in
// in metricQuery/index.tsx.
let axisUnitsMap = {
  [cockroach.ts.catalog.AxisUnits.BYTES]: AxisUnits.Bytes,
  [cockroach.ts.catalog.AxisUnits.COUNT]: AxisUnits.Count,
  [cockroach.ts.catalog.AxisUnits.DURATION]: AxisUnits.Duration,
  [cockroach.ts.catalog.AxisUnits.UNSET_UNITS]: AxisUnits.Count
}

// aggregatorEnumStringMap provides the string representation of TimeSeriesQueryAggregator
// enums.
let aggregatorEnumStringMap = {
  [cockroach.ts.tspb.TimeSeriesQueryAggregator.AVG]: "AVG",
  [cockroach.ts.tspb.TimeSeriesQueryAggregator.MAX]: "MAX",
  [cockroach.ts.tspb.TimeSeriesQueryAggregator.MIN]: "MIN",
  [cockroach.ts.tspb.TimeSeriesQueryAggregator.SUM]: "SUM",
  [cockroach.ts.tspb.TimeSeriesQueryAggregator.FIRST]: "FIRST",
  [cockroach.ts.tspb.TimeSeriesQueryAggregator.LAST]: "LAST",
  [cockroach.ts.tspb.TimeSeriesQueryAggregator.VARIANCE]: "VARIANCE"
}

// aggregatorEnumStringMap provides the string representation of TimeSeriesQueryDerivative
// enums.
let derivativeEnumStringMap = {
  [cockroach.ts.tspb.TimeSeriesQueryDerivative.DERIVATIVE]: "Derivative",
  [cockroach.ts.tspb.TimeSeriesQueryDerivative.NON_NEGATIVE_DERIVATIVE]: "Non-negative derivative",
  [cockroach.ts.tspb.TimeSeriesQueryDerivative.NONE]: "None",
}

export interface ChartCollectionProps {
  nodesSummary: NodesSummary;
  refreshNodes: typeof refreshNodes;
}

interface ChartCollectionInterface {
  title: string;
  chartsArr: cockroach.ts.catalog.IndividualChart[];
}

interface ChartCollectionState {
  error: Error;
  isLoaded: boolean;
  catalog: cockroach.ts.catalog.ChartSection[];
  collection?: ChartCollectionInterface
}

class ChartCollection extends React.Component<ChartCollectionProps & WithRouterProps, ChartCollectionState>{

  refresh(props = this.props) {
    props.refreshNodes();
  }

  constructor(props: ChartCollectionProps & WithRouterProps) {
    super(props);

    this.state = {
      error: null,
      isLoaded: false,
      catalog: null
    };
  }

  componentWillMount() {
    this.refresh();
  }

  componentWillReceiveProps(props: ChartCollectionProps & WithRouterProps) {
    this.refresh(props);
  }

  componentDidMount() {
    // Get the chart catalog from the endpoint.
    fetch("/_admin/v1/chartcatalog")
      .then(res => res.json())
      .then(
        result => {
          let collection: ChartCollectionInterface = getCharts(result.catalog, this.props.params.collectionName)
          this.setState({
            isLoaded: true,
            catalog: result.catalog,
            collection: collection
          });
        },
        error => {
          this.setState({
            isLoaded: true,
            error
          });
        }
      );
    
    
    window.scrollTo(0, 0);
  }

  render(){


    const { error, isLoaded } = this.state;
    if (error) {
      return <div>Error: {error.message}</div>;
    } else if (!isLoaded) {
      return <div>Loading...</div>;
    } else {

      const { nodesSummary } = this.props;
    
      // storeMetrics is used to determine the prefix for each metric when rendering
      // the charts; each is either "cr.store" or "cr.node". Those that appear in
      // store metrics are "cr.store" and all others are "cr.node".
      let storeMetrics: string[] = []; 
      nodesSummary.nodeStatuses 
        ? storeMetrics = _.keys(nodesSummary.nodeStatuses[0].store_statuses[0].metrics) 
        : this.refresh();


        // Create URL
        let allCharts = []
        this.state.collection.chartsArr.forEach(chart => {
          allCharts = allCharts.concat(getCustomChartParams(chart, storeMetrics))
        })

        let urlForCustomChart:string = '/#/debug/chart?charts=' + encodeURIComponent(JSON.stringify(allCharts));

        console.log(urlForCustomChart)
        // this.props.history.push(urlForCustomChart)

      return (
        <section className="section">
          
          <p className="navLink"><a href="/#/debug/chart-catalog">&lt; Back to Catalog</a></p>
          <h1>Chart Collection</h1>
          
          <PageConfig>
            <PageConfigItem>
              <TimeScaleDropdown />
            </PageConfigItem>
          </PageConfig>

          <h2 className="chart-catalog-header">{this.state.collection.title}</h2>

          <div>
          {
            _.map(this.state.collection.chartsArr, (chart, key) => 
              <RenderChart chart={chart} key={key} id={key} storeMetrics={storeMetrics}/>
            )
          }
          </div>
        </section>
      )
    }
  }
}

function RenderChart(props: {chart: cockroach.ts.catalog.IndividualChart, id: number, key: number, storeMetrics: string[]}){

  return(
    <div className="chartFromCollection">
    <MetricsDataProvider id={`chartscollection.${props.id}`}>
      <LineGraph title={props.chart.title}>
          <Axis label={props.chart.axisLabel} units={axisUnitsMap[props.chart.units]}>
              { RenderMetrics({chart:props.chart, storeMetrics: props.storeMetrics}) }
          </Axis>
      </LineGraph>
    </MetricsDataProvider>
    <ChartDetails chart={props.chart} storeMetrics={props.storeMetrics}/>
    </div>
  )
}

function RenderMetrics(props: {chart: cockroach.ts.catalog.IndividualChart, storeMetrics: string[]}){

  return _.map(props.chart.metrics, (metric) => (
    <Metric 
    key={metric.name} 
    name={_.includes(props.storeMetrics, metric.name) ? 'cr.store.' + metric.name : 'cr.node.' + metric.name} 
    downsampler={props.chart.downsampler} 
    aggregator={props.chart.aggregator}
    derivative={props.chart.derivative}
    />
  ))
}

interface ChartDetailsState {
  isHidden: boolean;
}

interface ChartDetailsProps {
  chart: cockroach.ts.catalog.IndividualChart;
  storeMetrics: string[];
}

class ChartDetails extends React.Component<ChartDetailsProps, ChartDetailsState> {
  constructor(props: {chart: cockroach.ts.catalog.IndividualChart, storeMetrics: string[]}) {
    super(props);

    this.state = {
      isHidden: true
    }
  }

  toggleHidden () {
    this.setState({
      isHidden: !this.state.isHidden
    })
  }

  render () {
    return (
      <div>
        <button onClick={this.toggleHidden.bind(this)} >
          Chart Details {this.state.isHidden ? '+' : '–'}
        </button>
        <RenderLinkToCustomChart chart={this.props.chart} storeMetrics={this.props.storeMetrics}/>
        <div className="chart-details">
          {!this.state.isHidden && <RenderMetricsDescriptionTable data={this.props.chart.data} />}
          {!this.state.isHidden && <RenderChartDescriptionTable chart={this.props.chart} />}
          {!this.state.isHidden && <hr />}
        </div>
      </div>
    )
  }
}

const MetricsDescriptionTable = SortedTable as new () => SortedTable<TableInfo>;

function RenderMetricsDescriptionTable(props: {data: TimeseriesData[]}){
  
  return(
    <div className="table-wrapper">
    <MetricsDescriptionTable
    data={props.data}
    columns={[
      {
        title: "Metric",
        cell: (data) => <strong>{data.name}</strong>,
        sort: (data) => data.name,
      },
      {
        title: "Description",
        cell: (data) => data.description,
        sort: (data) => data.description,
      }
    ]} />
    </div>
  )
}

const ChartDescriptionTable = SortedTable as new () => SortedTable<TableInfo>;

function RenderChartDescriptionTable(props: {chart: cockroach.ts.catalog.IndividualChart}){

  const chartOperations:{operation: string, value: string}[] = [
    {
      "operation" : "Downsampler",
      "value": aggregatorEnumStringMap[props.chart.downsampler]
    },
    {
      "operation" : "Aggregator",
      "value": aggregatorEnumStringMap[props.chart.aggregator]
    },
    {
      "operation" : "Rate",
      "value": derivativeEnumStringMap[props.chart.derivative]
    }
  ]

  return(
    <div className="table-wrapper">
    <ChartDescriptionTable
      data={chartOperations}
      columns={[
        {
          title: "Graph Operation",
          cell: (data) => <strong>{data.operation}</strong>,
          sort: (data) => data.operation,
        },
        {
          title: "Type",
          cell: (data) => data.value,
          sort: (data) => data.value,
        }
      ]} />
    </div>
  )
}

// getMetricVal creates a metricVal object using a chart and a metric name, which already 
// has the appropriate suffix i.e. cr.node or cr.store. This is broken out into its own
// function for simplicities sake in generating the correct metric name for histogram
// metricsm, i.e. requires appending a range of sufffixes.
function getMetricVal(chart: cockroach.ts.catalog.IndividualChart, metricName: string) {
  let metricVal = {}
  metricVal["downsampler"] = chart.downsampler;
  metricVal["aggregator"] = chart.aggregator;
  metricVal["derivative"] = chart.derivative;
  metricVal["perNode"] = false;
  metricVal["source"] = ""
  metricVal["metric"] = metricName
  return metricVal
}

function getCustomChartParams(chart: cockroach.ts.catalog.IndividualChart, storeMetrics: string[]) {
  let customChartURLParams = {
    ["metrics"]: [],
    ["axisUnits"]: axisUnitsMap[chart.units]
  };

  const quantileSuffixes = ["-max", "-p99.999", "-p99.99","-p99.9", "-p99","-p90","-p75","-p50"]

  chart.metrics.forEach(metric => {

    let metricName:string = _.includes(storeMetrics, metric.name) ? 'cr.store.' + metric.name : 'cr.node.' + metric.name;

    if(metric.metricType == io.prometheus.client.MetricType.HISTOGRAM) {
      // Append each quantile suffix to the metric name
      quantileSuffixes.forEach(suffix => {
        customChartURLParams["metrics"] = customChartURLParams["metrics"].concat(getMetricVal(chart, metricName + suffix))
      })
    } else {
      customChartURLParams["metrics"] = customChartURLParams["metrics"].concat(getMetricVal(chart, metricName))
    }

  });

  return customChartURLParams;
}

function RenderLinkToCustomChart(props: {chart: cockroach.ts.catalog.IndividualChart, storeMetrics: string[]}){
  
  // The custom charts page accepts as its input the following JSON structure,
  // which we'll fill in with this chart's detail. When this code was originally
  // written, the custom chart page only allowed users to render a single chart
  // at a time.
  // TODO(sean): Render multiple charts on the same page.
  let customChartURLParams = [];
  customChartURLParams[0] = {
    ["metrics"]: [],
    ["axisUnits"]: axisUnitsMap[props.chart.units]
  }

  props.chart.metrics.forEach(metric => {

    let metricName:string = _.includes(props.storeMetrics, metric.name) ? 'cr.store.' + metric.name : 'cr.node.' + metric.name;

    let metricVal = {}
    metricVal["downsampler"] = props.chart.downsampler;
    metricVal["aggregator"] = props.chart.aggregator;
    metricVal["derivative"] = props.chart.derivative;
    metricVal["perNode"] = false;
    metricVal["source"] = ""
    metricVal["metric"] = metricName

    customChartURLParams[0]["metrics"] = customChartURLParams[0]["metrics"].concat(metricVal)

  });

  let urlForCustomChart:string = '/#/debug/chart?charts=' + encodeURIComponent(JSON.stringify(customChartURLParams));

  return(
    <a href={urlForCustomChart}>
    <button>
      Load as Custom Chart >>
    </button>
    </a>
  )
}

// getCharts searches the catalog for the "collection" with the given name. Collections
// can either be a single chart or multiple charts, represented as either IndividualCharts
// or ChartSections.
function getCharts(catalog: cockroach.ts.catalog.ChartSection[], name: string){

  let collectionToDisplay: cockroach.ts.catalog.ChartSection|cockroach.ts.catalog.IndividualChart;
  let i: number = 0;

  // Iterate over all of the top levels of the catalog looking for the collection
  // with the given name.
  while(!collectionToDisplay && i < catalog.length){
    let node: cockroach.ts.catalog.ChartSection = catalog[i];
    collectionToDisplay = findCollection(node, name)
    console.log("collectionToDisplay",collectionToDisplay)
    i++
  }
  
  if(collectionToDisplay){
    let chartsArr: cockroach.ts.catalog.IndividualChart[] = []

    chartsArr = aggregateCharts(collectionToDisplay) as cockroach.ts.catalog.IndividualChart[]

    return {
      title: collectionToDisplay.longTitle,
      chartsArr: chartsArr
    };
  }
}

// findCollection searches the catalog for the "collection" with the given name. Collections
// can either be a single chart or multiple charts, represented as either IndividualCharts
// or ChartSections.
function findCollection(node: cockroach.ts.catalog.ChartSection|cockroach.ts.catalog.IndividualChart, collectionName: string): cockroach.ts.catalog.ChartSection|cockroach.ts.catalog.IndividualChart {

  if( node.collectionTitle === collectionName ){
    return node;
  }

  let results: cockroach.ts.catalog.ChartSection|cockroach.ts.catalog.IndividualChart;

  if(node.hasOwnProperty("subsections")){
    node = node as cockroach.ts.catalog.ChartSection

    node.charts.some(chart => {
      results = findCollection(chart as cockroach.ts.catalog.IndividualChart, collectionName)
      return results
    });

    if (!results){
      node.subsections.some(subsection => {
        results = findCollection(subsection as cockroach.ts.catalog.ChartSection, collectionName)
        return results
      });
    }
  }

  return results;
}

// aggregateChartsFromSection aggregates all of a section's IndividualCharts, as well
// as all of its subsection's IndividualCharts.
function aggregateChartsFromSection(section: cockroach.ts.catalog.ChartSection){
  // Aggregate all of the section's IndividualCharts.
  let chartsArr: cockroach.ts.catalog.IndividualChart[] = section.charts as cockroach.ts.catalog.IndividualChart[];

  // Aggregate all of the section's subsections' IndividualCharts.
  section.subsections.forEach((subsection: cockroach.ts.catalog.ChartSection) => {
    chartsArr = chartsArr.concat(aggregateChartsFromSection(subsection as cockroach.ts.catalog.ChartSection))
  });

  return chartsArr
}

// aggregateCharts collects all of the charts that can be reached from the node.
function aggregateCharts(node: cockroach.ts.catalog.ChartSection|cockroach.ts.catalog.IndividualChart){

  let chartsArr: cockroach.ts.catalog.IndividualChart[] = [];

  // Only IndividualCharts have "metrics"; otherwise the node is a ChartCatalog and should
  // aggregate all of its charts and all of its subsection's charts.
  if(node.hasOwnProperty("metrics")){

    chartsArr = chartsArr.concat(node as cockroach.ts.catalog.IndividualChart)
  
  } else {

    chartsArr = chartsArr.concat(aggregateChartsFromSection(node as cockroach.ts.catalog.ChartSection))

  }

  return chartsArr;
}

function mapStateToProps(state: AdminUIState) {
  return {
    nodesSummary: nodesSummarySelector(state),
  };
}

const mapDispatchToProps = {
  refreshNodes,
};

export default connect(mapStateToProps, mapDispatchToProps)(withRouter(ChartCollection));