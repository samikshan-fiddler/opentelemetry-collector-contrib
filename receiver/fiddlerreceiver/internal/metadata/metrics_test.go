// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fiddlerreceiver/internal/client"
)

func TestMetricBuilder(t *testing.T) {
	mb := NewMetricBuilder()
	assert.NotNil(t, mb)

	metrics := mb.Build()
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())
}

func TestAddDataPoints(t *testing.T) {
	// Create sample query results
	results := map[string]client.QueryResult{
		"query1": {
			Model: client.Model{
				ID:   "model1",
				Name: "Model 1",
				Project: client.Project{
					ID:   "project1",
					Name: "Project 1",
				},
			},
			Metric:   "traffic",
			ColNames: []string{"timestamp", "count"},
			Data:     [][]interface{}{{1622505600000.0, 42.0}},
		},
		"query2": {
			Model: client.Model{
				ID:   "model1",
				Name: "Model 1",
				Project: client.Project{
					ID:   "project1",
					Name: "Project 1",
				},
			},
			Metric:   "drift",
			ColNames: []string{"timestamp", "feature1,drift_score"},
			Data:     [][]interface{}{{1622505600000.0, 0.85}},
		},
		"query3": {
			Model: client.Model{
				ID:   "model2",
				Name: "Model 2",
				Project: client.Project{
					ID:   "project1",
					Name: "Project 1",
				},
			},
			Metric:   "performance",
			ColNames: []string{"timestamp", "precision", "recall"},
			Data:     [][]interface{}{{1622505600000.0, 0.92, 0.88}},
		},
		"empty": {
			Model: client.Model{
				ID:   "model3",
				Name: "Model 3",
			},
			Metric:   "empty",
			ColNames: []string{"timestamp"},
			Data:     [][]interface{}{},
		},
	}

	// Test time
	timestamp := time.Date(2021, 6, 1, 12, 0, 0, 0, time.UTC)

	// Build metrics
	mb := NewMetricBuilder()
	mb.AddDataPoints("Project 1", results, timestamp)
	metrics := mb.Build()

	// Verify resource metrics
	assert.Equal(t, 1, metrics.ResourceMetrics().Len())
	rm := metrics.ResourceMetrics().At(0)

	// Verify resource attributes
	attrs := rm.Resource().Attributes()
	val, ok := attrs.Get("service.name")
	assert.True(t, ok)
	assert.Equal(t, "fiddler", val.Str())

	val, ok = attrs.Get("fiddler.project")
	assert.True(t, ok)
	assert.Equal(t, "Project 1", val.Str())

	// Verify scope metrics
	assert.Equal(t, 1, rm.ScopeMetrics().Len())
	sm := rm.ScopeMetrics().At(0)

	// Expected metrics (skipping the empty one)
	// - traffic with 1 data point
	// - drift with 1 data point
	// - performance with 2 data points (precision, recall)
	expectedMetricCount := 4 // 1 + 1 + 2
	assert.Equal(t, expectedMetricCount, sm.Metrics().Len())

	// Helper to find a specific metric by name
	findMetric := func(name string) (int, bool) {
		for i := 0; i < sm.Metrics().Len(); i++ {
			if sm.Metrics().At(i).Name() == name {
				return i, true
			}
		}
		return 0, false
	}

	// Check traffic metric
	idx, found := findMetric("fiddler.traffic")
	assert.True(t, found)
	trafficMetric := sm.Metrics().At(idx)
	assert.Equal(t, "fiddler.traffic", trafficMetric.Name())
	assert.Equal(t, "Fiddler traffic metric", trafficMetric.Description())
	assert.Equal(t, "1", trafficMetric.Unit())

	// Check traffic data points
	require.Equal(t, 1, trafficMetric.Gauge().DataPoints().Len())
	dp := trafficMetric.Gauge().DataPoints().At(0)
	assert.Equal(t, pcommon.NewTimestampFromTime(timestamp), dp.Timestamp())
	assert.Equal(t, 42.0, dp.DoubleValue())

	// Check traffic attributes
	dpAttrs := dp.Attributes()
	val, ok = dpAttrs.Get("model")
	assert.True(t, ok)
	assert.Equal(t, "Model 1", val.Str())

	// Check drift metric
	idx, found = findMetric("fiddler.drift")
	assert.True(t, found)
	driftMetric := sm.Metrics().At(idx)
	assert.Equal(t, "fiddler.drift", driftMetric.Name())

	// Check drift data points
	require.Equal(t, 1, driftMetric.Gauge().DataPoints().Len())
	dp = driftMetric.Gauge().DataPoints().At(0)
	assert.Equal(t, 0.85, dp.DoubleValue())

	// Check drift attributes
	dpAttrs = dp.Attributes()
	val, ok = dpAttrs.Get("feature")
	assert.True(t, ok)
	assert.Equal(t, "feature1", val.Str())
}

func TestSplitColumnName(t *testing.T) {
	testCases := []struct {
		input    string
		expected []string
	}{
		{
			input:    "feature1,drift_score",
			expected: []string{"feature1", "drift_score"},
		},
		{
			input:    "simple",
			expected: []string{"simple"},
		},
		{
			input:    "",
			expected: []string{},
		},
		{
			input:    "a,b,c",
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tc := range testCases {
		result := splitColumnName(tc.input)
		assert.Equal(t, tc.expected, result)
	}
}
