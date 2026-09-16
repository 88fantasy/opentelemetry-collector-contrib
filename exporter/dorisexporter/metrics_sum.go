// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	_ "embed"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

//go:embed sql/metrics_sum_ddl.sql
var metricsSumDDL string

// dMetricSum Sum Metric to Doris
type dMetricSum struct {
	*dMetric               `json:",inline"`
	Timestamp              string         `json:"timestamp"`
	Attributes             map[string]any `json:"attributes"`
	StartTime              string         `json:"start_time"`
	Value                  any            `json:"value"`
	Exemplars              []*dExemplar   `json:"exemplars"`
	AggregationTemporality string         `json:"aggregation_temporality"`
	IsMonotonic            bool           `json:"is_monotonic"`
}

type metricModelSum struct {
	metricModelCommon[dMetricSum]
}

func (*metricModelSum) metricType() pmetric.MetricType {
	return pmetric.MetricTypeSum
}

func (*metricModelSum) tableSuffix() string {
	return "_sum"
}

func (m *metricModelSum) add(pm pmetric.Metric, dm *dMetric, e *metricsExporter, stats *metricDropStats) error {
	if pm.Type() != pmetric.MetricTypeSum {
		return fmt.Errorf("metric type is not sum: %v", pm.Type().String())
	}

	dataPoints := pm.Sum().DataPoints()
	for i := 0; i < dataPoints.Len(); i++ {
		dp := dataPoints.At(i)

		value, ok := e.getNumberDataPointValue(dp)
		if !ok {
			stats.datapoints++
			continue
		}
		encodedAttributes, ok := encodeDataPointAttributes(dp.Attributes(), stats)
		if !ok {
			continue
		}

		metric := &dMetricSum{
			dMetric:                dm,
			Timestamp:              e.formatTime(dp.Timestamp().AsTime()),
			Attributes:             encodedAttributes,
			StartTime:              e.formatTime(dp.StartTimestamp().AsTime()),
			Value:                  value,
			Exemplars:              e.encodeExemplars(dp.Exemplars(), stats),
			AggregationTemporality: pm.Sum().AggregationTemporality().String(),
			IsMonotonic:            pm.Sum().IsMonotonic(),
		}
		m.data = append(m.data, metric)
	}

	return nil
}
