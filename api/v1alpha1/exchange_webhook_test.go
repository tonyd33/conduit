package v1alpha1

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Helper function to create a minimal Exchange with a container
func newTestExchange(name, namespace, image string) *Exchange { //nolint:unparam
	return &Exchange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: ExchangeSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "exchange",
							Image: image,
						},
					},
				},
			},
		},
	}
}

func TestExchangeValidation(t *testing.T) {
	tests := []struct {
		name      string
		exchange  *Exchange
		expectErr bool
	}{
		{
			name:      "valid exchange",
			exchange:  newTestExchange("test-exchange", "default", "test-image:latest"),
			expectErr: false,
		},
		{
			name: "missing containers",
			exchange: &Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-exchange",
					Namespace: "default",
				},
				Spec: ExchangeSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "negative max restarts",
			exchange: func() *Exchange {
				ex := newTestExchange("test-exchange", "default", "test-image:latest")
				ex.Spec.Recovery.MaxRestarts = -1
				return ex
			}(),
			expectErr: true,
		},
		{
			name: "negative retention max age",
			exchange: func() *Exchange {
				ex := newTestExchange("test-exchange", "default", "test-image:latest")
				ex.Spec.Stream.Retention.MaxAge = metav1.Duration{Duration: -1}
				return ex
			}(),
			expectErr: true,
		},
		{
			name: "negative retention max messages",
			exchange: func() *Exchange {
				ex := newTestExchange("test-exchange", "default", "test-image:latest")
				ex.Spec.Stream.Retention.MaxMessages = -100
				return ex
			}(),
			expectErr: true,
		},
		{
			name: "invalid subject with spaces",
			exchange: func() *Exchange {
				ex := newTestExchange("test-exchange", "default", "test-image:latest")
				ex.Spec.Stream.Subjects.Input = "exchange.test input"
				return ex
			}(),
			expectErr: true,
		},
		{
			name: "invalid subject starting with dot",
			exchange: func() *Exchange {
				ex := newTestExchange("test-exchange", "default", "test-image:latest")
				ex.Spec.Stream.Subjects.Input = ".exchange.test"
				return ex
			}(),
			expectErr: true,
		},
		{
			name: "invalid subject ending with dot",
			exchange: func() *Exchange {
				ex := newTestExchange("test-exchange", "default", "test-image:latest")
				ex.Spec.Stream.Subjects.Input = "exchange.test."
				return ex
			}(),
			expectErr: true,
		},
		{
			name: "invalid subject with consecutive dots",
			exchange: func() *Exchange {
				ex := newTestExchange("test-exchange", "default", "test-image:latest")
				ex.Spec.Stream.Subjects.Input = "exchange..test"
				return ex
			}(),
			expectErr: true,
		},
		{
			name: "duplicate subjects",
			exchange: func() *Exchange {
				ex := newTestExchange("test-exchange", "default", "test-image:latest")
				ex.Spec.Stream.Subjects.Input = "exchange.test"
				ex.Spec.Stream.Subjects.Output = "exchange.test"
				return ex
			}(),
			expectErr: true,
		},
		{
			name: "reserved env var prefix EXCHANGE_",
			exchange: func() *Exchange {
				ex := newTestExchange("test-exchange", "default", "test-image:latest")
				ex.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
					{Name: "EXCHANGE_ID", Value: "custom-value"},
				}
				return ex
			}(),
			expectErr: true,
		},
		{
			name: "reserved env var prefix NATS_",
			exchange: func() *Exchange {
				ex := newTestExchange("test-exchange", "default", "test-image:latest")
				ex.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
					{Name: "NATS_URL", Value: "custom-url"},
				}
				return ex
			}(),
			expectErr: true,
		},
		{
			name: "valid custom subject",
			exchange: func() *Exchange {
				ex := newTestExchange("test-exchange", "default", "test-image:latest")
				ex.Spec.Stream.Subjects.Input = "my.app.input"
				ex.Spec.Stream.Subjects.Output = "my.app.output"
				return ex
			}(),
			expectErr: false,
		},
		{
			name: "valid wildcard subject",
			exchange: func() *Exchange {
				ex := newTestExchange("test-exchange", "default", "test-image:latest")
				ex.Spec.Stream.Subjects.Input = "exchange.*.input"
				return ex
			}(),
			expectErr: false,
		},
		{
			name: "negative max bytes",
			exchange: func() *Exchange {
				ex := newTestExchange("test-exchange", "default", "test-image:latest")
				ex.Spec.Stream.Retention.MaxBytes = resource.NewQuantity(-1, resource.BinarySI)
				return ex
			}(),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test ValidateCreate
			_, err := tt.exchange.ValidateCreate()
			if (err != nil) != tt.expectErr {
				t.Errorf("ValidateCreate() error = %v, expectErr %v", err, tt.expectErr)
			}

			// Test ValidateUpdate (with same old object)
			_, err = tt.exchange.ValidateUpdate(tt.exchange)
			if (err != nil) != tt.expectErr {
				t.Errorf("ValidateUpdate() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestExchangeDefault(t *testing.T) {
	exchange := newTestExchange("test-exchange", "default", "test-image:latest")

	// Call Default to set defaults
	exchange.Default()

	// Check that defaults were set
	if exchange.Spec.Stream.Retention.MaxMessages != 100000 {
		t.Errorf("Expected MaxMessages to be 100000, got %v", exchange.Spec.Stream.Retention.MaxMessages)
	}

	if exchange.Spec.Recovery.MaxRestarts != 5 {
		t.Errorf("Expected MaxRestarts to be 5, got %v", exchange.Spec.Recovery.MaxRestarts)
	}

	if exchange.Spec.Recovery.BackoffLimit != 3 {
		t.Errorf("Expected BackoffLimit to be 3, got %v", exchange.Spec.Recovery.BackoffLimit)
	}
}

func TestValidateSubjectName(t *testing.T) {
	tests := []struct {
		name      string
		subject   string
		expectErr bool
	}{
		{"valid simple", "exchange.test", false},
		{"valid with wildcard", "exchange.*.input", false},
		{"valid with multiple parts", "my.app.exchange.input", false},
		{"empty subject", "", true},
		{"with spaces", "exchange test", true},
		{"starts with dot", ".exchange", true},
		{"ends with dot", "exchange.", true},
		{"consecutive dots", "exchange..test", true},
		{"invalid character", "exchange@test", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSubjectName(tt.subject)
			if (err != nil) != tt.expectErr {
				t.Errorf("validateSubjectName(%q) error = %v, expectErr %v", tt.subject, err, tt.expectErr)
			}
		})
	}
}
