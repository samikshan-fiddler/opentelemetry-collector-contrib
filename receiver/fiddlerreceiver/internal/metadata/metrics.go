// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fiddlerreceiver/internal/metadata"

import (
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fiddlerreceiver/internal/client"
)

// MetricBuilder is a helper to create metrics from Fiddler API responses
type MetricBuilder struct {
	metrics pmetric.Metrics
}

// NewMetricBuilder creates a new MetricBuilder
func NewMetricBuilder() *MetricBuilder {
	return &MetricBuilder{
		metrics: pmetric.NewMetrics(),
	}
}

// Build finalizes the metrics and returns them
func (b *MetricBuilder) Build() pmetric.Metrics {
	return b.metrics
}

// AddDataPoints adds metrics data points from Fiddler API query results
func (b *MetricBuilder) AddDataPoints(projectName string, results map[string]client.QueryResult, timestamp time.Time) {
	// Resource scope for all metrics
	rm := b.metrics.ResourceMetrics().AppendEmpty()
	resource := rm.Resource()
	resource.Attributes().PutStr("service.name", "fiddler")
	resource.Attributes().PutStr("fiddler.project", projectName)

	// Scope for all metrics
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("fiddlerreceiver")

	// Process each query result
	for _, result := range results {
		modelName := result.Model.Name
		metricName := "fiddler." + result.Metric

		// Check if we have data
		if len(result.Data) == 0 || len(result.ColNames) == 0 {
			continue
		}

		// The last row has the most recent data
		lastRow := result.Data[len(result.Data)-1]

		// Process each column (skipping timestamp column)
		for colIdx, colName := range result.ColNames {
			if colName == "timestamp" {
				continue
			}

			// Split column name for tags if in format "feature,metric_name"
			var feature string
			colNameParts := splitColumnName(colName)
			if len(colNameParts) > 1 {
				feature = colNameParts[0]
			}

			// Extract metric value
			if colIdx >= len(lastRow) {
				continue
			}

			var val float64
			switch v := lastRow[colIdx].(type) {
			case float64:
				val = v
			case int:
				val = float64(v)
			case string:
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					val = f
				} else {
					continue
				}
			default:
				continue
			}

			// Create gauge metric
			metric := sm.Metrics().AppendEmpty()
			metric.SetName(metricName)
			metric.SetDescription("Fiddler " + result.Metric + " metric")
			metric.SetUnit("1")

			dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
			dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
			dp.SetDoubleValue(val)

			// Add attributes
			dp.Attributes().PutStr("model", modelName)
			if feature != "" {
				dp.Attributes().PutStr("feature", feature)
			}
			if colName != result.Metric {
				dp.Attributes().PutStr("metric_type", colName)
			}
		}
	}
}

// splitColumnName splits a column name like "feature,metric_name" into parts
func splitColumnName(name string) []string {
	if name == "" {
		return []string{}
	}

	return strings.Split(name, ",")
}
