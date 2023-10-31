package evaluation

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type PromClient struct {
	client api.Client
}

const (
	DefaultPromAddress = "http://prometheus-kube-prometheus-stack-prometheus:9090"
)

// NewPromClient returns a prometheus client
// TODO: TLS connect
func NewPromClient(promAddress string) (*PromClient, error) {

	targetAddress := DefaultPromAddress
	if len(promAddress) > 0 {
		targetAddress = promAddress
	}

	client, err := api.NewClient(
		api.Config{
			Address: targetAddress,
		},
	)
	if err != nil {
		return nil, err
	}

	return &PromClient{client: client}, nil
}

func (pc *PromClient) FetchQueryRange(ctx context.Context, query string, timeout time.Duration, start, end time.Time, step time.Duration, logger logr.Logger) (model.Value, error) {
	pv1 := v1.NewAPI(pc.client)
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	value, warnings, err := pv1.QueryRange(timeoutCtx, query, v1.Range{
		Step:  step,
		Start: start,
		End:   end,
	})

	if len(warnings) > 0 {
		logger.V(2).Info("Warning from prom: %v", warnings)
	}

	if err != nil {
		return nil, err
	}

	return value, err
}
