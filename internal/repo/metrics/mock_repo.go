package metrics

import (
	"context"
	"fmt"
)

type MockRepo struct {
	ListDailyMetricsFunc func(ctx context.Context, filter Filter) ([]DailyMetric, error)
	ListFetchLatencyFunc func(ctx context.Context, filter Filter) ([]FetchLatencyMetric, error)
}

func (m *MockRepo) ListDailyMetrics(ctx context.Context, filter Filter) ([]DailyMetric, error) {
	if m.ListDailyMetricsFunc == nil {
		return nil, fmt.Errorf("ListDailyMetrics not implemented")
	}
	return m.ListDailyMetricsFunc(ctx, filter)
}

func (m *MockRepo) ListFetchLatency(ctx context.Context, filter Filter) ([]FetchLatencyMetric, error) {
	if m.ListFetchLatencyFunc == nil {
		return nil, fmt.Errorf("ListFetchLatency not implemented")
	}
	return m.ListFetchLatencyFunc(ctx, filter)
}
