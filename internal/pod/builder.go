package pod

import (
	"fmt"

	conduitv1alpha1 "github.com/tonyd33/conduit/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Environment variables injected into Exchange pods
	EnvExchangeID     = "EXCHANGE_ID"
	EnvExchangeName   = "EXCHANGE_NAME"
	EnvExchangeNS     = "EXCHANGE_NAMESPACE"
	EnvStreamName     = "NATS_STREAM_NAME"
	EnvConsumerName   = "NATS_CONSUMER_NAME"
	EnvSubjectInput   = "NATS_SUBJECT_INPUT"
	EnvSubjectOutput  = "NATS_SUBJECT_OUTPUT"
	EnvSubjectControl = "NATS_SUBJECT_CONTROL"
	EnvNATSURL        = "NATS_URL"
)

// BuildPodForExchange creates a Pod spec for the given Exchange
func BuildPodForExchange(exchange *conduitv1alpha1.Exchange, streamName, consumerName string) (*corev1.Pod, error) {
	// Generate pod name based on Exchange name
	podName := fmt.Sprintf("%s-pod", exchange.Name)

	// Start with the user-provided template
	template := exchange.Spec.Template.DeepCopy()

	// Build Conduit-specific environment variables
	conduitEnv := buildEnvVars(exchange, streamName, consumerName)

	// Find or create the main container (first container, or create one if none exist)
	var mainContainer *corev1.Container
	if len(template.Spec.Containers) == 0 {
		// No containers defined, create a default one
		template.Spec.Containers = []corev1.Container{{
			Name: "exchange",
		}}
	}
	mainContainer = &template.Spec.Containers[0]

	// Inject Conduit environment variables (prepend so user vars can override if needed)
	mainContainer.Env = append(conduitEnv, mainContainer.Env...)

	// Ensure RestartPolicy is set to Never (controller handles restarts)
	template.Spec.RestartPolicy = corev1.RestartPolicyNever

	// Build the pod, merging template metadata with required labels
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        podName,
			Namespace:   exchange.Namespace,
			Labels:      template.Labels,
			Annotations: template.Annotations,
		},
		Spec: template.Spec,
	}

	// Ensure required labels are set
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels["app.kubernetes.io/name"] = "conduit"
	pod.Labels["app.kubernetes.io/instance"] = exchange.Name
	pod.Labels["app.kubernetes.io/component"] = "exchange"
	pod.Labels["app.kubernetes.io/managed-by"] = "conduit-operator"
	pod.Labels["conduit.mnke.org/exchange"] = exchange.Name

	return pod, nil
}

// buildEnvVars constructs the environment variables for the Exchange pod
func buildEnvVars(exchange *conduitv1alpha1.Exchange, streamName, consumerName string) []corev1.EnvVar {
	exchangeID := string(exchange.UID)

	// Build subject patterns (use defaults if not specified)
	subjectInput := fmt.Sprintf("exchange.%s.input", exchangeID)
	subjectOutput := fmt.Sprintf("exchange.%s.output", exchangeID)
	subjectControl := fmt.Sprintf("exchange.%s.control", exchangeID)

	if exchange.Spec.Stream.Subjects.Input != "" {
		subjectInput = exchange.Spec.Stream.Subjects.Input
	}
	if exchange.Spec.Stream.Subjects.Output != "" {
		subjectOutput = exchange.Spec.Stream.Subjects.Output
	}
	if exchange.Spec.Stream.Subjects.Control != "" {
		subjectControl = exchange.Spec.Stream.Subjects.Control
	}

	env := []corev1.EnvVar{
		{
			Name:  EnvExchangeID,
			Value: exchangeID,
		},
		{
			Name:  EnvExchangeName,
			Value: exchange.Name,
		},
		{
			Name:  EnvExchangeNS,
			Value: exchange.Namespace,
		},
		{
			Name:  EnvStreamName,
			Value: streamName,
		},
		{
			Name:  EnvConsumerName,
			Value: consumerName,
		},
		{
			Name:  EnvSubjectInput,
			Value: subjectInput,
		},
		{
			Name:  EnvSubjectOutput,
			Value: subjectOutput,
		},
		{
			Name:  EnvSubjectControl,
			Value: subjectControl,
		},
		{
			Name:  EnvNATSURL,
			Value: getNATSURL(),
		},
	}

	return env
}

// getNATSURL returns the NATS server URL
// TODO: Make this configurable via operator config or environment
func getNATSURL() string {
	// Default to NATS in the same cluster
	return "nats://nats.default.svc.cluster.local:4222"
}
