// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fiddlerreceiver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fiddlerreceiver/internal/client"
)

func TestNewReceiver(t *testing.T) {
	consumer := consumertest.NewNop()
	settings := receivertest.NewNopSettings(typeStr)
	fr := newFiddlerReceiver(&Config{
		Endpoint: "https://app.fiddler.ai",
		Token:    "test-token",
		Interval: 10 * time.Minute,
		Timeout:  5 * time.Minute,
	}, consumer, settings)

	assert.NotNil(t, fr)
	assert.Same(t, consumer, fr.consumer)
}

func TestStartAndShutdown(t *testing.T) {
	// Setup mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Return models list on models endpoint
		if r.URL.Path == "/v3/models" {
			modelsResponse := struct {
				Data struct {
					Items []client.Model `json:"items"`
				} `json:"data"`
			}{}
			modelsResponse.Data.Items = []client.Model{
				{
					ID:   "model1",
					Name: "Model 1",
					Project: client.Project{
						ID:   "project1",
						Name: "Project 1",
					},
				},
			}
			json.NewEncoder(w).Encode(modelsResponse)
		}
	}))
	defer ts.Close()

	config := &Config{
		Endpoint: ts.URL,
		Token:    "test-token",
		Interval: 10 * time.Minute,
		Timeout:  1 * time.Minute,
	}
	consumer := consumertest.NewNop()
	settings := receivertest.NewNopSettings(typeStr)
	fr := newFiddlerReceiver(config, consumer, settings)

	// Test start
	err := fr.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Test shutdown
	err = fr.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestCollect(t *testing.T) {
	// Sample response data
	modelsResponse := struct {
		Data struct {
			Items []client.Model `json:"items"`
		} `json:"data"`
	}{}
	modelsResponse.Data.Items = []client.Model{
		{
			ID:   "model1",
			Name: "Model 1",
			Project: client.Project{
				ID:   "project1",
				Name: "Project 1",
			},
		},
	}

	metricsResponse := struct {
		Data struct {
			Metrics []client.Metric `json:"metrics"`
			Columns []client.Column `json:"columns"`
		} `json:"data"`
	}{}
	metricsResponse.Data.Metrics = []client.Metric{
		{
			ID:               "traffic",
			Type:             "traffic",
			Columns:          []string{"timestamp", "count"},
			RequiresBaseline: false,
		},
	}

	queryResponse := client.QueryResponse{}
	queryResponse.Data.Project = client.Project{ID: "project1", Name: "Project 1"}
	queryResponse.Data.Results = map[string]client.QueryResult{
		"traffic": {
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
	}

	// Setup mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Return appropriate response based on the endpoint
		if r.URL.Path == "/v3/models" {
			json.NewEncoder(w).Encode(modelsResponse)
		} else if r.URL.Path == "/v3/models/model1/metrics" {
			json.NewEncoder(w).Encode(metricsResponse)
		} else if r.URL.Path == "/v3/queries" {
			json.NewEncoder(w).Encode(queryResponse)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	// Create receiver with test consumer
	config := &Config{
		Endpoint: ts.URL,
		Token:    "test-token",
		Interval: 1 * time.Minute,
		Timeout:  1 * time.Minute,
	}
	sink := new(consumertest.MetricsSink)
	logger := zap.NewNop()
	settings := receivertest.NewNopSettings(typeStr)
	settings.Logger = logger
	fr := newFiddlerReceiver(config, sink, settings)

	// Start the receiver
	err := fr.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Force a collection
	err = fr.collect(context.Background())
	require.NoError(t, err)

	// Verify that metrics were received
	assert.Eventually(t, func() bool {
		return len(sink.AllMetrics()) > 0
	}, 2*time.Second, 10*time.Millisecond, "no metrics were collected")

	// Verify metrics content
	mtrcs := sink.AllMetrics()
	require.Len(t, mtrcs, 1)

	// Shutdown
	err = fr.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestHelperFunctions(t *testing.T) {
	// Test formatTime
	t1 := time.Date(2021, 6, 1, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, "2021-06-01 12:00:00", formatTime(t1))

	// Test getBinSizeString
	assert.Equal(t, "Hour", getBinSizeString(30*time.Minute))
	assert.Equal(t, "Day", getBinSizeString(12*time.Hour))
	assert.Equal(t, "Week", getBinSizeString(7*24*time.Hour))
	assert.Equal(t, "Month", getBinSizeString(30*24*time.Hour))

	// Test isMetricEnabled
	enabledMetrics := []string{"traffic", "drift"}
	assert.True(t, isMetricEnabled("traffic", enabledMetrics))
	assert.True(t, isMetricEnabled("drift", enabledMetrics))
	assert.False(t, isMetricEnabled("performance", enabledMetrics))
	assert.True(t, isMetricEnabled("anything", []string{})) // empty list means all enabled
}
