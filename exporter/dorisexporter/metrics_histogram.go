// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	_ "embed"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

//go:embed sql/metrics_histogram_ddl.sql
var metricsHistogramDDL string

// dMetricHistogram Histogram Metric to Doris
type dMetricHistogram struct {
	*dMetric               `json:",inline"`
	Timestamp              string         `json:"timestamp"`
	Attributes             map[string]any `json:"attributes"`
	StartTime              string         `json:"start_time"`
	Count                  int64          `json:"count"`
	Sum                    any            `json:"sum"`
	BucketCounts           []int64        `json:"bucket_counts"`
	ExplicitBounds         []any          `json:"explicit_bounds"`
	Exemplars              []*dExemplar   `json:"exemplars"`
	Min                    any            `json:"min"`
	Max                    any            `json:"max"`
	AggregationTemporality string         `json:"aggregation_temporality"`
}

type metricModelHistogram struct {
	metricModelCommon[dMetricHistogram]
}

func (*metricModelHistogram) metricType() pmetric.MetricType {
	return pmetric.MetricTypeHistogram
}

func (*metricModelHistogram) tableSuffix() string {
	return "_histogram"
}

func (m *metricModelHistogram) add(pm pmetric.Metric, dm *dMetric, e *metricsExporter, stats *metricDropStats) error {
	if pm.Type() != pmetric.MetricTypeHistogram {
		return fmt.Errorf("metric type is not histogram: %v", pm.Type().String())
	}

	dataPoints := pm.Histogram().DataPoints()
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

		bucketCounts := dp.BucketCounts()
		newBucketCounts := make([]int64, 0, bucketCounts.Len())
		for j := 0; j < bucketCounts.Len(); j++ {
			newBucketCounts = append(newBucketCounts, int64(bucketCounts.At(j)))
		}

		explicitBounds, ok := encodeJSONValue(dp.ExplicitBounds().AsRaw(), e.allowNonFinite)
		if !ok {
			stats.datapoints++
			continue
		}
		encodedBounds, _ := explicitBounds.([]any)

		metric := &dMetricHistogram{
			dMetric:                dm,
			Timestamp:              e.formatTime(dp.Timestamp().AsTime()),
			Attributes:             encodedAttributes,
			StartTime:              e.formatTime(dp.StartTimestamp().AsTime()),
			Count:                  int64(dp.Count()),
			Sum:                    sum,
			BucketCounts:           newBucketCounts,
			ExplicitBounds:         encodedBounds,
			Exemplars:              e.encodeExemplars(dp.Exemplars(), stats),
			Min:                    minValue,
			Max:                    maxValue,
			AggregationTemporality: pm.Histogram().AggregationTemporality().String(),
		}
		m.data = append(m.data, metric)
	}

	return nil
}
