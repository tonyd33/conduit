/*
Copyright 2026 tonyd33.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ExchangeSpec defines the desired state of Exchange.
type ExchangeSpec struct {
	// Template describes the pod that will be created for this Exchange.
	// The template will be modified by the controller to inject NATS connection
	// information and other Exchange-specific configuration.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:XPreserveUnknownFields
	Template corev1.PodTemplateSpec `json:"template"`

	// Stream defines the NATS JetStream configuration for this Exchange
	Stream StreamConfig `json:"stream,omitempty"`

	// Recovery defines the recovery behavior when the pod fails
	Recovery RecoveryConfig `json:"recovery,omitempty"`
}

// StreamConfig defines NATS JetStream stream configuration
type StreamConfig struct {
	// Retention defines how long messages are kept in the stream
	Retention RetentionConfig `json:"retention,omitempty"`

	// Subjects defines the NATS subjects for this exchange
	// If not specified, defaults will be used based on Exchange ID
	Subjects SubjectConfig `json:"subjects,omitempty"`
}

// RetentionConfig defines message retention policies
type RetentionConfig struct {
	// MaxAge is the maximum age of messages to retain (e.g., "168h" for 7 days)
	// +kubebuilder:default="168h"
	MaxAge metav1.Duration `json:"maxAge,omitempty"`

	// MaxMessages is the maximum number of messages to retain
	// +kubebuilder:default=100000
	MaxMessages int64 `json:"maxMessages,omitempty"`

	// MaxBytes is the maximum total size of messages to retain
	// +kubebuilder:default="1Gi"
	MaxBytes *resource.Quantity `json:"maxBytes,omitempty"`
}

// SubjectConfig defines NATS subject patterns
type SubjectConfig struct {
	// Input is the subject pattern for messages TO the application
	// Defaults to "exchange.{exchange-id}.input"
	Input string `json:"input,omitempty"`

	// Output is the subject pattern for messages FROM the application
	// Defaults to "exchange.{exchange-id}.output"
	Output string `json:"output,omitempty"`

	// Control is the subject pattern for control messages
	// Defaults to "exchange.{exchange-id}.control"
	Control string `json:"control,omitempty"`
}

// RecoveryConfig defines recovery behavior
type RecoveryConfig struct {
	// MaxRestarts is the maximum number of times to restart a failed workload
	// +kubebuilder:default=5
	MaxRestarts int32 `json:"maxRestarts,omitempty"`

	// BackoffLimit is the number of retries before applying backoff
	// +kubebuilder:default=3
	BackoffLimit int32 `json:"backoffLimit,omitempty"`
}

// ExchangePhase represents the current phase of the Exchange lifecycle
// +kubebuilder:validation:Enum=Created;Pending;Running;Recovering;Completed;Failed;Archived
type ExchangePhase string

const (
	ExchangePhaseCreated    ExchangePhase = "Created"
	ExchangePhasePending    ExchangePhase = "Pending"
	ExchangePhaseRunning    ExchangePhase = "Running"
	ExchangePhaseRecovering ExchangePhase = "Recovering"
	ExchangePhaseCompleted  ExchangePhase = "Completed"
	ExchangePhaseFailed     ExchangePhase = "Failed"
	ExchangePhaseArchived   ExchangePhase = "Archived"
)

// ExchangeStatus defines the observed state of Exchange.
type ExchangeStatus struct {
	// Phase is the current phase of the Exchange
	Phase ExchangePhase `json:"phase,omitempty"`

	// PodName is the name of the current or last workload pod
	PodName string `json:"podName,omitempty"`

	// PodPhase is the phase of the workload pod
	PodPhase corev1.PodPhase `json:"podPhase,omitempty"`

	// MessageStats tracks message counts and sequences
	MessageStats MessageStats `json:"messageStats,omitempty"`

	// Recovery tracks recovery attempts and status
	Recovery RecoveryStatus `json:"recovery,omitempty"`

	// Archival tracks archival status (for external archival systems)
	Archival ArchivalStatus `json:"archival,omitempty"`

	// Conditions represent the latest available observations of the Exchange's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// StreamName is the name of the NATS JetStream stream
	StreamName string `json:"streamName,omitempty"`

	// ConsumerName is the name of the durable NATS consumer
	ConsumerName string `json:"consumerName,omitempty"`
}

// MessageStats tracks message statistics
type MessageStats struct {
	// TotalInput is the total number of input messages
	TotalInput int64 `json:"totalInput,omitempty"`

	// TotalOutput is the total number of output messages
	TotalOutput int64 `json:"totalOutput,omitempty"`

	// LastInputSeq is the sequence number of the last input message
	LastInputSeq uint64 `json:"lastInputSeq,omitempty"`

	// LastOutputSeq is the sequence number of the last output message
	LastOutputSeq uint64 `json:"lastOutputSeq,omitempty"`

	// LastProcessedSeq is the sequence number of the last processed message
	LastProcessedSeq uint64 `json:"lastProcessedSeq,omitempty"`
}

// RecoveryStatus tracks recovery attempts
type RecoveryStatus struct {
	// RestartCount is the number of times the workload has been restarted
	RestartCount int32 `json:"restartCount,omitempty"`

	// LastRestartTime is when the workload was last restarted
	LastRestartTime *metav1.Time `json:"lastRestartTime,omitempty"`

	// LastRecoveryDuration is how long the last recovery took
	LastRecoveryDuration *metav1.Duration `json:"lastRecoveryDuration,omitempty"`
}

// ArchivalStatus tracks archival status for external systems
type ArchivalStatus struct {
	// LastArchivedSeq is the sequence number of the last archived message
	LastArchivedSeq uint64 `json:"lastArchivedSeq,omitempty"`

	// ArchivedAt is when the exchange was archived
	ArchivedAt *metav1.Time `json:"archivedAt,omitempty"`

	// ExternalReference is a reference to the archived data (e.g., S3 path)
	ExternalReference string `json:"externalReference,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ex
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Pod",type=string,JSONPath=`.status.podName`
// +kubebuilder:printcolumn:name="Restarts",type=integer,JSONPath=`.status.recovery.restartCount`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Exchange is the Schema for the exchanges API.
type Exchange struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExchangeSpec   `json:"spec,omitempty"`
	Status ExchangeStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ExchangeList contains a list of Exchange.
type ExchangeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Exchange `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Exchange{}, &ExchangeList{})
}
