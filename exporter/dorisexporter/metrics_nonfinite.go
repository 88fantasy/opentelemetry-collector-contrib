// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"math"
	"reflect"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type metricDropStats struct {
	datapoints int
	exemplars  int
}

func (s *metricDropStats) log(logger *zap.Logger) {
	if s.datapoints == 0 && s.exemplars == 0 {
		return
	}
	logger.Warn("dropped non-finite metric values",
		zap.Int("datapoints", s.datapoints),
		zap.Int("exemplars", s.exemplars),
	)
}

func encodeDataPointAttributes(attributes pcommon.Map, stats *metricDropStats) (map[string]any, bool) {
	encoded, ok := encodeJSONValue(attributes.AsRaw(), true)
	if !ok {
		stats.datapoints++
		return nil, false
	}
	return encoded.(map[string]any), true
}

func encodeFloat64(value float64, allowNonFinite bool) (any, bool) {
	switch {
	case math.IsNaN(value):
		if allowNonFinite {
			return "NaN", true
		}
	case math.IsInf(value, 1):
		if allowNonFinite {
			return "Infinity", true
		}
	case math.IsInf(value, -1):
		if allowNonFinite {
			return "-Infinity", true
		}
	default:
		return value, true
	}
	return nil, false
}

func encodeJSONValue(value any, allowNonFinite bool) (any, bool) {
	return encodeJSONReflectValue(reflect.ValueOf(value), allowNonFinite)
}

func encodeJSONReflectValue(value reflect.Value, allowNonFinite bool) (any, bool) {
	if !value.IsValid() {
		return nil, true
	}

	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil, true
		}
		return encodeJSONReflectValue(value.Elem(), allowNonFinite)
	}

	switch value.Kind() {
	case reflect.Float32, reflect.Float64:
		return encodeFloat64(value.Float(), allowNonFinite)
	case reflect.Map:
		if value.IsNil() {
			return value.Interface(), true
		}
		if value.Type().Key().Kind() != reflect.String {
			return value.Interface(), true
		}
		encoded := make(map[string]any, value.Len())
		iter := value.MapRange()
		for iter.Next() {
			item, ok := encodeJSONReflectValue(iter.Value(), allowNonFinite)
			if !ok {
				return nil, false
			}
			encoded[iter.Key().String()] = item
		}
		return encoded, true
	case reflect.Slice, reflect.Array:
		if value.Kind() == reflect.Slice && value.IsNil() {
			return value.Interface(), true
		}
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return value.Interface(), true
		}
		encoded := make([]any, value.Len())
		for i := 0; i < value.Len(); i++ {
			item, ok := encodeJSONReflectValue(value.Index(i), allowNonFinite)
			if !ok {
				return nil, false
			}
			encoded[i] = item
		}
		return encoded, true
	default:
		return value.Interface(), true
	}
}

func (e *metricsExporter) encodeExemplars(exemplars pmetric.ExemplarSlice, stats *metricDropStats) []*dExemplar {
	encoded := make([]*dExemplar, 0, exemplars.Len())
	for i := 0; i < exemplars.Len(); i++ {
		exemplar := exemplars.At(i)
		value, ok := e.getExemplarValue(exemplar)
		if !ok {
			stats.exemplars++
			continue
		}
		attributes, ok := encodeJSONValue(exemplar.FilteredAttributes().AsRaw(), true)
		if !ok {
			stats.exemplars++
			continue
		}

		encodedAttributes, _ := attributes.(map[string]any)
		encoded = append(encoded, &dExemplar{
			FilteredAttributes: encodedAttributes,
			Timestamp:          e.formatTime(exemplar.Timestamp().AsTime()),
			Value:              value,
			SpanID:             exemplar.SpanID().String(),
			TraceID:            exemplar.TraceID().String(),
		})
	}
	return encoded
}
