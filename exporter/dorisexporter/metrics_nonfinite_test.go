// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestEncodeJSONValue(t *testing.T) {
	value := map[string]any{
		"nan": math.NaN(),
		"nested": []float64{
			math.Inf(1),
			1,
		},
	}

	_, ok := encodeJSONValue(value, false)
	require.False(t, ok)

	encoded, ok := encodeJSONValue(value, true)
	require.True(t, ok)
	require.Equal(t, map[string]any{
		"nan": "NaN",
		"nested": []any{
			"Infinity",
			float64(1),
		},
	}, encoded)
}

func TestMetricModelsDropNonFiniteDatapoints(t *testing.T) {
	tests := []struct {
		name  string
		build func() pmetric.Metric
		model metricModel
	}{
		{
			name: "gauge value",
			build: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(math.NaN())
				return metric
			},
			model: &metricModelGauge{},
		},
		{
			name: "sum value",
			build: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetEmptySum().DataPoints().AppendEmpty().SetDoubleValue(math.Inf(1))
				return metric
			},
			model: &metricModelSum{},
		},
		{
			name: "histogram sum",
			build: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
				dp.SetSum(math.NaN())
				return metric
			},
			model: &metricModelHistogram{},
		},
		{
			name: "exponential histogram zero threshold",
			build: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				dp := metric.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
				dp.SetZeroThreshold(math.NaN())
				return metric
			},
			model: &metricModelExponentialHistogram{},
		},
		{
			name: "summary sum",
			build: func() pmetric.Metric {
				metric := pmetric.NewMetric()
				metric.SetEmptySummary().DataPoints().AppendEmpty().SetSum(math.NaN())
				return metric
			},
			model: &metricModelSummary{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.ClientConfig.Endpoint = "http://localhost:8030"
			cfg.CreateSchema = false
			require.NoError(t, cfg.Validate())
			exporter := newMetricsExporter(zap.NewNop(), cfg, componenttest.NewNopTelemetrySettings())
			stats := &metricDropStats{}
			require.NoError(t, tt.model.add(tt.build(), &dMetric{}, exporter, stats))
			require.Equal(t, 0, tt.model.size())
			require.Equal(t, 1, stats.datapoints)
		})
	}
}

func TestMetricModelsPreserveNonFiniteAttributes(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig.Endpoint = "http://localhost:8030"
	cfg.CreateSchema = false
	require.NoError(t, cfg.Validate())
	exporter := newMetricsExporter(zap.NewNop(), cfg, componenttest.NewNopTelemetrySettings())
	metric := pmetric.NewMetric()
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(1)
	dp.Attributes().PutDouble("non_finite", math.NaN())

	model := &metricModelGauge{}
	stats := &metricDropStats{}
	require.NoError(t, model.add(metric, &dMetric{}, exporter, stats))
	require.Len(t, model.data, 1)
	require.Equal(t, "NaN", model.data[0].Attributes["non_finite"])
	require.Equal(t, 0, stats.datapoints)
}

func TestEncodeExemplarsDropsOnlyNonFiniteExemplars(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.ClientConfig.Endpoint = "http://localhost:8030"
	cfg.CreateSchema = false
	require.NoError(t, cfg.Validate())
	exporter := newMetricsExporter(zap.NewNop(), cfg, componenttest.NewNopTelemetrySettings())
	metric := pmetric.NewMetric()
	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(1)
	dp.Exemplars().AppendEmpty().SetDoubleValue(math.NaN())
	dp.Exemplars().AppendEmpty().SetDoubleValue(2)

	model := &metricModelGauge{}
	stats := &metricDropStats{}
	require.NoError(t, model.add(metric, &dMetric{}, exporter, stats))
	require.Len(t, model.data, 1)
	require.Len(t, model.data[0].Exemplars, 1)
	require.Equal(t, 1, stats.exemplars)
}
