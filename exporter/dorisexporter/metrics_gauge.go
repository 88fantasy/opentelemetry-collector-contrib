// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	_ "embed"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

//go:embed sql/metrics_gauge_ddl.sql
var metricsGaugeDDL string

// dMetricGauge Gauge Metric to Doris
type dMetricGauge struct {
	*dMetric   `json:",inline"`
	Timestamp  string         `json:"timestamp"`
	Attributes map[string]any `json:"attributes"`
	StartTime  string         `json:"start_time"`
	Value      any            `json:"value"`
	Exemplars  []*dExemplar   `json:"exemplars"`
}

type metricModelGauge struct {
	metricModelCommon[dMetricGauge]
}

func (*metricModelGauge) metricType() pmetric.MetricType {
	return pmetric.MetricTypeGauge
}

func (*metricModelGauge) tableSuffix() string {
	return "_gauge"
}

func (m *metricModelGauge) add(pm pmetric.Metric, dm *dMetric, e *metricsExporter, stats *metricDropStats) error {
	if pm.Type() != pmetric.MetricTypeGauge {
		return fmt.Errorf("metric type is not gauge: %v", pm.Type().String())
	}

	dataPoints := pm.Gauge().DataPoints()
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

		metric := &dMetricGauge{
			dMetric:    dm,
			Timestamp:  e.formatTime(dp.Timestamp().AsTime()),
			Attributes: encodedAttributes,
			StartTime:  e.formatTime(dp.StartTimestamp().AsTime()),
			Value:      value,
			Exemplars:  e.encodeExemplars(dp.Exemplars(), stats),
		}
		m.data = append(m.data, metric)
	}

	return nil
}
