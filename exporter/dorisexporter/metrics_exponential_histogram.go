// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	_ "embed"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

//go:embed sql/metrics_exponential_histogram_ddl.sql
var metricsExponentialHistogramDDL string

// dMetricExponentialHistogram Exponential Histogram Metric to Doris
type dMetricExponentialHistogram struct {
	*dMetric               `json:",inline"`
	Timestamp              string         `json:"timestamp"`
	Attributes             map[string]any `json:"attributes"`
	StartTime              string         `json:"start_time"`
	Count                  int64          `json:"count"`
	Sum                    any            `json:"sum"`
	Scale                  int32          `json:"scale"`
	ZeroCount              int64          `json:"zero_count"`
	PositiveOffset         int32          `json:"positive_offset"`
	PositiveBucketCounts   []int64        `json:"positive_bucket_counts"`
	NegativeOffset         int32          `json:"negative_offset"`
	NegativeBucketCounts   []int64        `json:"negative_bucket_counts"`
	Exemplars              []*dExemplar   `json:"exemplars"`
	Min                    any            `json:"min"`
	Max                    any            `json:"max"`
	ZeroThreshold          any            `json:"zero_threshold"`
	AggregationTemporality string         `json:"aggregation_temporality"`
}

type metricModelExponentialHistogram struct {
	metricModelCommon[dMetricExponentialHistogram]
}

func (*metricModelExponentialHistogram) metricType() pmetric.MetricType {
	return pmetric.MetricTypeExponentialHistogram
}

func (*metricModelExponentialHistogram) tableSuffix() string {
	return "_exponential_histogram"
}

func (m *metricModelExponentialHistogram) add(pm pmetric.Metric, dm *dMetric, e *metricsExporter, stats *metricDropStats) error {
	if pm.Type() != pmetric.MetricTypeExponentialHistogram {
		return fmt.Errorf("metric type is not exponential histogram: %v", pm.Type().String())
	}

	dataPoints := pm.ExponentialHistogram().DataPoints()
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
		minValue, ok := encodeFloat64(dp.Min(), e.allowNonFinite)
		if !ok {
			stats.datapoints++
			continue
		}
		maxValue, ok := encodeFloat64(dp.Max(), e.allowNonFinite)
		if !ok {
			stats.datapoints++
			continue
		}
		zeroThreshold, ok := encodeFloat64(dp.ZeroThreshold(), e.allowNonFinite)
		if !ok {
			stats.datapoints++
			continue
		}

		positiveBucketCounts := dp.Positive().BucketCounts()
		newPositiveBucketCounts := make([]int64, 0, positiveBucketCounts.Len())
		for j := 0; j < positiveBucketCounts.Len(); j++ {
			newPositiveBucketCounts = append(newPositiveBucketCounts, int64(positiveBucketCounts.At(j)))
		}

		negativeBucketCounts := dp.Negative().BucketCounts()
		newNegativeBucketCounts := make([]int64, 0, negativeBucketCounts.Len())
		for j := 0; j < negativeBucketCounts.Len(); j++ {
			newNegativeBucketCounts = append(newNegativeBucketCounts, int64(negativeBucketCounts.At(j)))
		}

		metric := &dMetricExponentialHistogram{
			dMetric:                dm,
			Timestamp:              e.formatTime(dp.Timestamp().AsTime()),
			Attributes:             encodedAttributes,
			StartTime:              e.formatTime(dp.StartTimestamp().AsTime()),
			Count:                  int64(dp.Count()),
			Sum:                    sum,
			Scale:                  dp.Scale(),
			ZeroCount:              int64(dp.ZeroCount()),
			PositiveOffset:         dp.Positive().Offset(),
			PositiveBucketCounts:   newPositiveBucketCounts,
			NegativeOffset:         dp.Negative().Offset(),
			NegativeBucketCounts:   newNegativeBucketCounts,
			Exemplars:              e.encodeExemplars(dp.Exemplars(), stats),
			Min:                    minValue,
			Max:                    maxValue,
			ZeroThreshold:          zeroThreshold,
			AggregationTemporality: pm.ExponentialHistogram().AggregationTemporality().String(),
		}
		m.data = append(m.data, metric)
	}

	return nil
}
