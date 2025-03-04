// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fiddlerreceiver/internal/metadata"

import (
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fiddlerreceiver/internal/client"
)

// MetricBuilder is a helper to create metrics from Fiddler API responses
type MetricBuilder struct {
	metrics pmetric.Metrics
	logger  *zap.Logger
}

// NewMetricBuilder creates a new MetricBuilder
func NewMetricBuilder(logger *zap.Logger) *MetricBuilder {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &MetricBuilder{
		metrics: pmetric.NewMetrics(),
		logger:  logger,
	}
}

// Build finalizes the metrics and returns them
func (b *MetricBuilder) Build() pmetric.Metrics {
	return b.metrics
}

// AddDataPoints adds metrics data points from Fiddler API query results
func (b *MetricBuilder) AddDataPoints(projectName string, results map[string]client.QueryResult) {
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

		if len(result.Data) == 0 || len(result.ColNames) == 0 {
			continue
		}

		for _, row := range result.Data {
			if len(row) == 0 || len(row) != len(result.ColNames) {
				continue
			}

			var pointTimestamp time.Time
			var hasTimestamp bool

			var timestampColIdx int = -1
			for i, colName := range result.ColNames {
				if colName == "timestamp" {
					timestampColIdx = i
					break
				}
			}

			// Extract and parse timestamp if column exists
			if timestampColIdx >= 0 && timestampColIdx < len(row) {
				if tsStr, ok := row[timestampColIdx].(string); ok {
					var err error
					pointTimestamp, err = time.Parse(time.RFC3339, tsStr)
					if err != nil {
						b.logger.Error("Failed to parse timestamp string",
							zap.String("project", projectName),
							zap.String("metric", result.Metric),
							zap.String("model", modelName),
							zap.String("timestamp", tsStr),
							zap.Error(err))
						continue
					}
					hasTimestamp = true
				}
			}

			if !hasTimestamp {
				b.logger.Error("Missing timestamp in row data",
					zap.String("project", projectName),
					zap.String("metric", result.Metric),
					zap.String("model", modelName))
				continue
			}

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
				if colIdx >= len(row) {
					continue
				}

				var val float64
				switch v := row[colIdx].(type) {
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
				dp.SetTimestamp(pcommon.NewTimestampFromTime(pointTimestamp))
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
}

// splitColumnName splits a column name like "feature,metric_name" into parts
func splitColumnName(name string) []string {
	if name == "" {
		return []string{}
	}

	return strings.Split(name, ",")
}
