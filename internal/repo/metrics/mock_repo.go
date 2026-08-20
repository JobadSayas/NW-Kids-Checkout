package metrics

import (
	"context"
	"fmt"
)

type MockRepo struct {
	ListDailyMetricsFunc func(ctx context.Context, filter Filter) ([]DailyMetric, error)
}

func (m *MockRepo) ListDailyMetrics(ctx context.Context, filter Filter) ([]DailyMetric, error) {
	if m.ListDailyMetricsFunc == nil {
		return nil, fmt.Errorf("ListDailyMetrics not implemented")
	}
	return m.ListDailyMetricsFunc(ctx, filter)
}
