package metrics

import (
	"context"
	"time"

	conduitv1alpha1 "github.com/tonyd33/conduit/api/v1alpha1"
	natsclient "github.com/tonyd33/conduit/internal/nats"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Collector periodically collects metrics from NATS and Kubernetes
type Collector struct {
	client     client.Client
	natsClient *natsclient.Client
	interval   time.Duration
}

// NewCollector creates a new metrics collector
func NewCollector(k8sClient client.Client, natsClient *natsclient.Client, interval time.Duration) *Collector {
	return &Collector{
		client:     k8sClient,
		natsClient: natsClient,
		interval:   interval,
	}
}

// Start begins collecting metrics in the background
// This implements the controller-runtime Runnable interface
func (c *Collector) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("metrics-collector")
	logger.Info("Starting metrics collector", "interval", c.interval)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// Collect once immediately
	c.collect(ctx)

	for {
		select {
		case <-ctx.Done():
			logger.Info("Stopping metrics collector")
			return nil // Return nil on graceful shutdown
		case <-ticker.C:
			c.collect(ctx)
		}
	}
}

// NeedLeaderElection returns false - metrics collection doesn't require leader election
func (c *Collector) NeedLeaderElection() bool {
	return false
}

// collect gathers metrics from Kubernetes and NATS
func (c *Collector) collect(ctx context.Context) {
	logger := log.FromContext(ctx)

	// Collect Exchange lifecycle metrics
	c.collectExchangeMetrics(ctx)

	// Collect NATS stream metrics (only if NATS client is available)
	if c.natsClient != nil {
		c.collectNATSMetrics(ctx)
	} else {
		logger.V(1).Info("Skipping NATS metrics collection (NATS client not available)")
	}
}

// collectExchangeMetrics collects metrics about Exchange resources
func (c *Collector) collectExchangeMetrics(ctx context.Context) {
	logger := log.FromContext(ctx)

	// List all Exchanges
	exchangeList := &conduitv1alpha1.ExchangeList{}
	if err := c.client.List(ctx, exchangeList); err != nil {
		logger.Error(err, "Failed to list Exchanges for metrics")
		return
	}

	// Reset phase counters
	phaseCounts := make(map[string]map[string]int) // namespace -> phase -> count

	for _, exchange := range exchangeList.Items {
		ns := exchange.Namespace
		phase := string(exchange.Status.Phase)

		if phaseCounts[ns] == nil {
			phaseCounts[ns] = make(map[string]int)
		}
		phaseCounts[ns][phase]++

		// Update restart count metric
		restartCount := exchange.Status.Recovery.RestartCount
		ExchangeRestarts.WithLabelValues(exchange.Name, ns).Set(float64(restartCount))
	}

	// Update phase gauges
	for ns, phases := range phaseCounts {
		for phase, count := range phases {
			ExchangeTotal.WithLabelValues(phase, ns).Set(float64(count))
		}
	}
}

// collectNATSMetrics collects metrics from NATS streams
func (c *Collector) collectNATSMetrics(ctx context.Context) {
	logger := log.FromContext(ctx)

	// List all Exchanges to get their stream names
	exchangeList := &conduitv1alpha1.ExchangeList{}
	if err := c.client.List(ctx, exchangeList); err != nil {
		logger.Error(err, "Failed to list Exchanges for NATS metrics")
		return
	}

	for _, exchange := range exchangeList.Items {
		streamName := exchange.Status.StreamName
		consumerName := exchange.Status.ConsumerName

		if streamName == "" {
			continue // Stream not yet created
		}

		// Get stream info
		streamInfo, err := c.natsClient.GetStreamInfo(streamName)
		if err != nil {
			logger.Error(err, "Failed to get stream info", "stream", streamName, "exchange", exchange.Name)
			continue
		}

		// Update stream metrics
		StreamMessages.WithLabelValues(
			exchange.Name,
			exchange.Namespace,
			streamName,
		).Set(float64(streamInfo.State.Msgs))

		StreamBytes.WithLabelValues(
			exchange.Name,
			exchange.Namespace,
			streamName,
		).Set(float64(streamInfo.State.Bytes))

		StreamConsumers.WithLabelValues(
			exchange.Name,
			exchange.Namespace,
			streamName,
		).Set(float64(streamInfo.State.Consumers))

		// Get consumer info if available
		if consumerName != "" {
			consumerInfo, err := c.natsClient.GetConsumerInfo(streamName, consumerName)
			if err != nil {
				logger.Error(err, "Failed to get consumer info", "consumer", consumerName, "exchange", exchange.Name)
				continue
			}

			// Update consumer metrics
			ConsumerPending.WithLabelValues(
				exchange.Name,
				exchange.Namespace,
				consumerName,
			).Set(float64(consumerInfo.NumPending))

			ConsumerDelivered.WithLabelValues(
				exchange.Name,
				exchange.Namespace,
				consumerName,
			).Set(float64(consumerInfo.Delivered.Stream))

			ConsumerAckPending.WithLabelValues(
				exchange.Name,
				exchange.Namespace,
				consumerName,
			).Set(float64(consumerInfo.NumAckPending))
		}
	}
}
