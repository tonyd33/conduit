package apiserver

import (
	conduitv1alpha1 "github.com/tonyd33/conduit/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// CreateExchangeRequest represents the API request to create an Exchange
type CreateExchangeRequest struct {
	// Name of the Exchange (must be unique within namespace)
	Name string `json:"name"`

	// Namespace to create the Exchange in (defaults to "default")
	Namespace string `json:"namespace,omitempty"`

	// Image for the worker pod (defaults to conduit default image)
	Image string `json:"image,omitempty"`

	// ImagePullPolicy for the worker pod
	ImagePullPolicy string `json:"imagePullPolicy,omitempty"`

	// Environment variables for the worker pod
	Env []EnvVar `json:"env,omitempty"`

	// Resource requests and limits
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// JetStream stream configuration
	StreamConfig *StreamConfig `json:"streamConfig,omitempty"`

	// JetStream consumer configuration
	ConsumerConfig *ConsumerConfig `json:"consumerConfig,omitempty"`

	// Recovery configuration
	Recovery *RecoveryConfig `json:"recovery,omitempty"`
}

// EnvVar represents an environment variable
type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ResourceRequirements represents compute resource requirements
type ResourceRequirements struct {
	Requests *ResourceList `json:"requests,omitempty"`
	Limits   *ResourceList `json:"limits,omitempty"`
}

// ResourceList represents a list of resources
type ResourceList struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// StreamConfig represents JetStream stream configuration
type StreamConfig struct {
	Subjects     []string `json:"subjects,omitempty"`
	Retention    string   `json:"retention,omitempty"` // "limits", "interest", or "workqueue"
	MaxConsumers int      `json:"maxConsumers,omitempty"`
	MaxMsgs      int64    `json:"maxMsgs,omitempty"`
	MaxBytes     int64    `json:"maxBytes,omitempty"`
	MaxAge       string   `json:"maxAge,omitempty"` // Duration string like "24h"
	Replicas     int      `json:"replicas,omitempty"`
	NoAck        bool     `json:"noAck,omitempty"`
}

// ConsumerConfig represents JetStream consumer configuration
type ConsumerConfig struct {
	AckPolicy     string `json:"ackPolicy,omitempty"` // "explicit", "all", or "none"
	AckWait       string `json:"ackWait,omitempty"`   // Duration string like "30s"
	MaxDeliver    int    `json:"maxDeliver,omitempty"`
	FilterSubject string `json:"filterSubject,omitempty"`
	ReplayPolicy  string `json:"replayPolicy,omitempty"` // "instant" or "original"
	MaxAckPending int    `json:"maxAckPending,omitempty"`
	MaxWaiting    int    `json:"maxWaiting,omitempty"`
	HeadersOnly   bool   `json:"headersOnly,omitempty"`
}

// RecoveryConfig represents recovery configuration
type RecoveryConfig struct {
	MaxRestarts   int32  `json:"maxRestarts,omitempty"`
	RestartPolicy string `json:"restartPolicy,omitempty"` // "Always", "OnFailure", or "Never"
	BackoffLimit  int32  `json:"backoffLimit,omitempty"`
}

// ExchangeResponse represents the API response after creating an Exchange
type ExchangeResponse struct {
	// Exchange metadata
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	UID       string `json:"uid"`

	// Current status
	Phase   string `json:"phase"`
	Message string `json:"message,omitempty"`

	// Connection information for clients
	ConnectionInfo ConnectionInfo `json:"connectionInfo"`
}

// ConnectionInfo provides the information clients need to connect to the Exchange
type ConnectionInfo struct {
	// NATS connection details
	NATSURL string `json:"natsURL"`

	// Stream and subject information
	StreamName    string `json:"streamName"`
	InputSubject  string `json:"inputSubject"`
	OutputSubject string `json:"outputSubject"`

	// Worker pod information
	PodName  string `json:"podName,omitempty"`
	PodPhase string `json:"podPhase,omitempty"`

	// Status
	Ready bool `json:"ready"`
}

// ListExchangesResponse represents the response for listing exchanges
type ListExchangesResponse struct {
	Exchanges []ExchangeResponse `json:"exchanges"`
	Total     int                `json:"total"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Details string `json:"details,omitempty"`
}

// Helper functions to convert between API types and CRD types

// ToExchangeSpec converts CreateExchangeRequest to Exchange spec
func (r *CreateExchangeRequest) ToExchangeSpec() *conduitv1alpha1.ExchangeSpec {
	spec := &conduitv1alpha1.ExchangeSpec{}

	if r.Image != "" {
		spec.Image = r.Image
	}

	if r.ImagePullPolicy != "" {
		spec.ImagePullPolicy = corev1.PullPolicy(r.ImagePullPolicy)
	}

	if len(r.Env) > 0 {
		spec.Env = make([]corev1.EnvVar, len(r.Env))
		for i, env := range r.Env {
			spec.Env[i] = corev1.EnvVar{
				Name:  env.Name,
				Value: env.Value,
			}
		}
	}

	if r.Resources != nil {
		spec.Resources = corev1.ResourceRequirements{}
		if r.Resources.Requests != nil {
			spec.Resources.Requests = corev1.ResourceList{}
			// Note: Actual resource parsing would use resource.MustParse()
		}
		if r.Resources.Limits != nil {
			spec.Resources.Limits = corev1.ResourceList{}
		}
	}

	if r.StreamConfig != nil {
		spec.Stream.Subjects.Input = r.StreamConfig.Subjects[0] // simplified
		// Note: Full stream config conversion would require more complex mapping
	}

	if r.Recovery != nil {
		spec.Recovery = conduitv1alpha1.RecoveryConfig{
			MaxRestarts: r.Recovery.MaxRestarts,
		}
	}

	return spec
}

// FromExchange converts an Exchange CR to ExchangeResponse
func FromExchange(exchange *conduitv1alpha1.Exchange, natsURL string) *ExchangeResponse {
	// Get input/output subjects from spec or generate defaults
	inputSubject := exchange.Spec.Stream.Subjects.Input
	if inputSubject == "" {
		inputSubject = "exchange." + string(exchange.UID) + ".input"
	}

	outputSubject := exchange.Spec.Stream.Subjects.Output
	if outputSubject == "" {
		outputSubject = "exchange." + string(exchange.UID) + ".output"
	}

	resp := &ExchangeResponse{
		Name:      exchange.Name,
		Namespace: exchange.Namespace,
		UID:       string(exchange.UID),
		Phase:     string(exchange.Status.Phase),
		ConnectionInfo: ConnectionInfo{
			NATSURL:       natsURL,
			StreamName:    exchange.Status.StreamName,
			InputSubject:  inputSubject,
			OutputSubject: outputSubject,
			PodName:       exchange.Status.PodName,
			PodPhase:      string(exchange.Status.PodPhase),
			Ready:         exchange.Status.Phase == conduitv1alpha1.ExchangePhaseRunning,
		},
	}

	return resp
}
