package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// Exchange lifecycle metrics
	ExchangeTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "conduit_exchange_total",
			Help: "Total number of Exchanges by phase",
		},
		[]string{"phase", "namespace"},
	)

	ExchangeCreated = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "conduit_exchange_created_total",
			Help: "Total number of Exchanges created",
		},
		[]string{"namespace"},
	)

	ExchangeDeleted = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "conduit_exchange_deleted_total",
			Help: "Total number of Exchanges deleted",
		},
		[]string{"namespace"},
	)

	// Pod restart metrics
	ExchangeRestarts = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "conduit_exchange_restarts",
			Help: "Number of pod restarts for an Exchange",
		},
		[]string{"exchange", "namespace"},
	)

	// NATS stream metrics
	StreamMessages = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "conduit_stream_messages",
			Help: "Number of messages in the NATS stream",
		},
		[]string{"exchange", "namespace", "stream"},
	)

	StreamBytes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "conduit_stream_bytes",
			Help: "Number of bytes in the NATS stream",
		},
		[]string{"exchange", "namespace", "stream"},
	)

	StreamConsumers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "conduit_stream_consumers",
			Help: "Number of consumers for the NATS stream",
		},
		[]string{"exchange", "namespace", "stream"},
	)

	ConsumerPending = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "conduit_consumer_pending_messages",
			Help: "Number of pending messages for the consumer",
		},
		[]string{"exchange", "namespace", "consumer"},
	)

	ConsumerDelivered = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "conduit_consumer_delivered_messages",
			Help: "Number of messages delivered to the consumer",
		},
		[]string{"exchange", "namespace", "consumer"},
	)

	ConsumerAckPending = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "conduit_consumer_ack_pending",
			Help: "Number of messages pending acknowledgment",
		},
		[]string{"exchange", "namespace", "consumer"},
	)

	// Recovery metrics
	RecoveryAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "conduit_recovery_attempts_total",
			Help: "Total number of recovery attempts",
		},
		[]string{"exchange", "namespace"},
	)

	RecoverySuccess = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "conduit_recovery_success_total",
			Help: "Total number of successful recoveries",
		},
		[]string{"exchange", "namespace"},
	)

	RecoveryFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "conduit_recovery_failed_total",
			Help: "Total number of failed recoveries",
		},
		[]string{"exchange", "namespace"},
	)

	// Reconciliation metrics
	ReconciliationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "conduit_reconciliation_duration_seconds",
			Help:    "Time spent in reconciliation loop",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"namespace"},
	)

	ReconciliationErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "conduit_reconciliation_errors_total",
			Help: "Total number of reconciliation errors",
		},
		[]string{"namespace"},
	)
)

func init() {
	// Register custom metrics with controller-runtime's metrics registry
	metrics.Registry.MustRegister(
		ExchangeTotal,
		ExchangeCreated,
		ExchangeDeleted,
		ExchangeRestarts,
		StreamMessages,
		StreamBytes,
		StreamConsumers,
		ConsumerPending,
		ConsumerDelivered,
		ConsumerAckPending,
		RecoveryAttempts,
		RecoverySuccess,
		RecoveryFailed,
		ReconciliationDuration,
		ReconciliationErrors,
	)
}
