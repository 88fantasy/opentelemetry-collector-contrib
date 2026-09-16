// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	_ "embed"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

//go:embed sql/metrics_summary_ddl.sql
var metricsSummaryDDL string

// dMetricSummary Summary metric model to Doris
type dMetricSummary struct {
	*dMetric       `json:",inline"`
	Timestamp      string            `json:"timestamp"`
	Attributes     map[string]any    `json:"attributes"`
	StartTime      string            `json:"start_time"`
	Count          int64             `json:"count"`
	Sum            any               `json:"sum"`
	QuantileValues []*dQuantileValue `json:"quantile_values"`
}

// dQuantileValue Quantile Value to Doris
type dQuantileValue struct {
	Quantile any `json:"quantile"`
	Value    any `json:"value"`
}

type metricModelSummary struct {
	metricModelCommon[dMetricSummary]
}

func (*metricModelSummary) metricType() pmetric.MetricType {
	return pmetric.MetricTypeSummary
}

func (*metricModelSummary) tableSuffix() string {
	return "_summary"
}

func (m *metricModelSummary) add(pm pmetric.Metric, dm *dMetric, e *metricsExporter, stats *metricDropStats) error {
	if pm.Type() != pmetric.MetricTypeSummary {
		return fmt.Errorf("metric type is not summary: %v", pm.Type().String())
	}

	dataPoints := pm.Summary().DataPoints()
	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)

		encodedAttributes, ok := encodeDataPointAttributes(dp.Attributes(), stats)
		if !ok {
			continue
		}
		sum, ok := encodeFloat64(dp.Sum(), e.allowNonFinite)
		if !ok {
			stats.datapoints++
			continue
		}

		quantileValues := dp.QuantileValues()
		newQuantileValues := make([]*dQuantileValue, 0, quantileValues.Len())
		for j := 0; j < quantileValues.Len(); j++ {
			quantileValue := quantileValues.At(j)
			quantile, ok := encodeFloat64(quantileValue.Quantile(), e.allowNonFinite)
			if !ok {
				stats.datapoints++
				newQuantileValues = nil
				break
			}
			value, ok := encodeFloat64(quantileValue.Value(), e.allowNonFinite)
			if !ok {
				stats.datapoints++
				newQuantileValues = nil
				break
			}

			newQuantileValue := &dQuantileValue{
				Quantile: quantile,
				Value:    value,
			}

			newQuantileValues = append(newQuantileValues, newQuantileValue)
		}
		if newQuantileValues == nil {
			continue
		}

		metric := &dMetricSummary{
			dMetric:        dm,
			Timestamp:      e.formatTime(dp.Timestamp().AsTime()),
			Attributes:     encodedAttributes,
			StartTime:      e.formatTime(dp.StartTimestamp().AsTime()),
			Count:          int64(dp.Count()),
			Sum:            sum,
			QuantileValues: newQuantileValues,
		}
		m.data = append(m.data, metric)
	}

	return nil
}
