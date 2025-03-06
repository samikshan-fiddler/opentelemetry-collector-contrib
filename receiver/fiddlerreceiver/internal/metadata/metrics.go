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
	metrics       pmetric.Metrics
	logger        *zap.Logger
	metricTypeMap map[string]string // Maps metric ID to its type
}

// NewMetricBuilder creates a new MetricBuilder
func NewMetricBuilder(logger *zap.Logger) *MetricBuilder {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &MetricBuilder{
		metrics:       pmetric.NewMetrics(),
		logger:        logger,
		metricTypeMap: make(map[string]string),
	}
}

// Build finalizes the metrics and returns them
func (b *MetricBuilder) Build() pmetric.Metrics {
	return b.metrics
}

// AddMetricType adds a metric type mapping
func (b *MetricBuilder) AddMetricType(metricID, metricType string) {
	b.metricTypeMap[metricID] = metricType
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
		metricName := result.Metric

		if len(result.Data) == 0 || len(result.ColNames) == 0 {
			continue
		}

		// Get the appropriate metric prefix based on metric type
		// metricPrefix := getMetricPrefix(metricType)

		for _, row := range result.Data {
			if len(row) == 0 || len(row) != len(result.ColNames) {
				continue
			}

			timestamp, found := extractTimestamp(row, result.ColNames)
			if !found {
				b.logger.Error("Missing timestamp in row data",
					zap.String("project", projectName),
					zap.String("metric", metricName),
					zap.String("model", modelName))
				continue
			}

			// Process each column (skipping timestamp column)
			for colIdx, colName := range result.ColNames {
				if colName == "timestamp" {
					continue
				}

				// Extract metric value
				if colIdx >= len(row) {
					continue
				}

				value, ok := extractValue(row[colIdx])
				if !ok {
					continue
				}

				// Create metrics based on column name
				b.addMetricFromColumn(sm, timestamp, projectName, modelName, metricName, colName, value)
			}
		}
	}
}

// addMetricFromColumn adds a metric based on the column name and metric type
func (b *MetricBuilder) addMetricFromColumn(sm pmetric.ScopeMetrics, timestamp time.Time, projectName, modelName, metricName, colName string, value float64) {
	// Get metric type from our mapping, if available
	metricType, exists := b.metricTypeMap[metricName]

	// Determine how to structure the metric name
	var fullMetricName string
	if exists {
		// Use the metric type directly for the category
		fullMetricName = "fiddler." + metricType + "." + metricName
	} else {
		// If we don't have type info, don't use a category
		fullMetricName = "fiddler." + metricName
	}

	// Find or create the metric
	var metric pmetric.Metric
	var dp pmetric.NumberDataPoint
	metricFound := false

	// Try to find an existing metric with the same name
	for i := 0; i < sm.Metrics().Len(); i++ {
		if sm.Metrics().At(i).Name() == fullMetricName {
			metric = sm.Metrics().At(i)
			metricFound = true
			break
		}
	}

	// If not found, create a new metric
	if !metricFound {
		metric = sm.Metrics().AppendEmpty()
		metric.SetName(fullMetricName)
		metric.SetDescription("Fiddler metric: " + metricName)
		metric.SetUnit("1")
		metric.SetEmptyGauge() // Set empty gauge only when creating a new metric
	}

	dp = metric.Gauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(timestamp))
	dp.SetDoubleValue(value)

	dp.Attributes().PutStr("model", modelName)
	dp.Attributes().PutStr("project", projectName)

	if exists && metricType != "" {
		dp.Attributes().PutStr("metric_type", metricType)
	}

	// Check if column name has additional information (e.g., "metricType,featureName")
	parts := splitColumnName(colName)
	if len(parts) > 1 {
		// For column names with format "metric,feature"
		if parts[0] == metricName {
			dp.Attributes().PutStr("feature", parts[1])
		} else {
			// For cases where column name doesn't start with metric name
			// Use the first part as metric type and second as a property
			if parts[1] != "" {
				dp.Attributes().PutStr("feature", parts[1])
			}
			if parts[0] != metricName && parts[0] != "" {
				dp.Attributes().PutStr("metric_type", parts[0])
			}
		}
	}
}

// extractTimestamp extracts the timestamp from a row
func extractTimestamp(row []interface{}, colNames []string) (time.Time, bool) {
	var timestampColIdx int = -1
	for i, colName := range colNames {
		if colName == "timestamp" {
			timestampColIdx = i
			break
		}
	}

	if timestampColIdx >= 0 && timestampColIdx < len(row) {
		if tsStr, ok := row[timestampColIdx].(string); ok {
			if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
				return t, true
			}
		}
	}

	return time.Time{}, false
}

// extractValue extracts a numeric value from various types
func extractValue(rawValue interface{}) (float64, bool) {
	switch v := rawValue.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// splitColumnName splits a column name like "metric,feature" into parts
func splitColumnName(name string) []string {
	if name == "" {
		return []string{}
	}

	return strings.Split(name, ",")
}
