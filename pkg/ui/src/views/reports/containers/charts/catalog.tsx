// //taken from pkg.ts.tsb.timeseries.pb
// export const TimeSeriesQueryAggregator_value: { [aggregatorString: string]: number; } = {
//   "AVG": 1,
//   "SUM": 2,
//   "MAX": 3,
//   "MIN": 4,
// }

// export const TimeSeriesQueryAggregator_name: { [aggregatorNumber: number]: string; } = {
//   1: "AVG",
//   2: "SUM",
//   3: "MAX",
//   4: "MIN",
// }

// export const TimeSeriesQueryRate_value: { [rateString: string]: number; } = {
//   "Normal": 0,
//   "Rate": 1,
//   "Non-negative rate": 2
// }

// export const TimeSeriesQueryRate_name: { [rateNumber: number]: string; } = {
//   0: "Normal",
//   1: "Rate",
//   2: "Non-negative rate"
// }

// export const TimeSeriesQueryUnits_name: { [unitNumber: number]: string; } = {
//   0: "Count",
//   1: "Bytes",
//   2: "Duration"
// }

// export const TimeSeriesQueryUnits_value: { [unitsString: string]: number; } = {
//   "Count": 0,
//   "Bytes": 1,
//   "Duration": 2
// }

// export interface ChartMeta {
//   name: string;
//   longName: string;
//   collectionName: string;
//   level: number;
// }

// export interface ChartSection extends ChartMeta {
//   description?: string;
//   subsections?: ChartSection[];
//   charts: ChartTimeSeries[];
// }

// export interface ChartTimeSeries extends ChartMeta{
//   timeseries: IndividualChart;
// }

// export interface IndividualChart {
//   title?: string;
//   data: TimeseriesData[];
//   downsampler: string;
//   aggregator: string;
//   rate: string;
//   units: string;
//   axisLabel: string;
// }

// export interface TimeseriesData {
//   name: string;
//   description: string;
//   location: string;
// }

// import catalog from './catalog.json';

// export const Charts: ChartSection[] = catalog;