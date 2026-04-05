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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExchangeValidation(t *testing.T) {
	tests := []struct {
		name      string
		exchange  *Exchange
		expectErr bool
	}{
		{
			name: "valid exchange",
			exchange: &Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-exchange",
					Namespace: "default",
				},
				Spec: ExchangeSpec{
					Image:    "test-image:latest",
					Recovery: RecoveryConfig{},
				},
			},
			expectErr: false,
		},
		{
			name: "missing image",
			exchange: &Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-exchange",
					Namespace: "default",
				},
				Spec: ExchangeSpec{
					Image: "",
				},
			},
			expectErr: true,
		},
		{
			name: "negative max restarts",
			exchange: &Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-exchange",
					Namespace: "default",
				},
				Spec: ExchangeSpec{
					Image: "test-image:latest",
					Recovery: RecoveryConfig{
						MaxRestarts: -1,
					},
				},
			},
			expectErr: true,
		},
		{
			name: "negative retention max age",
			exchange: &Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-exchange",
					Namespace: "default",
				},
				Spec: ExchangeSpec{
					Image: "test-image:latest",
					Stream: StreamConfig{
						Retention: RetentionConfig{
							MaxAge: metav1.Duration{Duration: -1},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "negative retention max messages",
			exchange: &Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-exchange",
					Namespace: "default",
				},
				Spec: ExchangeSpec{
					Image: "test-image:latest",
					Stream: StreamConfig{
						Retention: RetentionConfig{
							MaxMessages: -100,
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "invalid subject with spaces",
			exchange: &Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-exchange",
					Namespace: "default",
				},
				Spec: ExchangeSpec{
					Image: "test-image:latest",
					Stream: StreamConfig{
						Subjects: SubjectConfig{
							Input: "exchange.test input",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "invalid subject starting with dot",
			exchange: &Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-exchange",
					Namespace: "default",
				},
				Spec: ExchangeSpec{
					Image: "test-image:latest",
					Stream: StreamConfig{
						Subjects: SubjectConfig{
							Input: ".exchange.test",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "invalid subject ending with dot",
			exchange: &Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-exchange",
					Namespace: "default",
				},
				Spec: ExchangeSpec{
					Image: "test-image:latest",
					Stream: StreamConfig{
						Subjects: SubjectConfig{
							Input: "exchange.test.",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "invalid subject with consecutive dots",
			exchange: &Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-exchange",
					Namespace: "default",
				},
				Spec: ExchangeSpec{
					Image: "test-image:latest",
					Stream: StreamConfig{
						Subjects: SubjectConfig{
							Input: "exchange..test",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "duplicate subjects",
			exchange: &Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-exchange",
					Namespace: "default",
				},
				Spec: ExchangeSpec{
					Image: "test-image:latest",
					Stream: StreamConfig{
						Subjects: SubjectConfig{
							Input:  "exchange.test",
							Output: "exchange.test",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "reserved env var prefix EXCHANGE_",
			exchange: &Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-exchange",
					Namespace: "default",
				},
				Spec: ExchangeSpec{
					Image: "test-image:latest",
					Env: []corev1.EnvVar{
						{
							Name:  "EXCHANGE_ID",
							Value: "custom-value",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "reserved env var prefix NATS_",
			exchange: &Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-exchange",
					Namespace: "default",
				},
				Spec: ExchangeSpec{
					Image: "test-image:latest",
					Env: []corev1.EnvVar{
						{
							Name:  "NATS_URL",
							Value: "custom-url",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "valid custom subject",
			exchange: &Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-exchange",
					Namespace: "default",
				},
				Spec: ExchangeSpec{
					Image: "test-image:latest",
					Stream: StreamConfig{
						Subjects: SubjectConfig{
							Input:  "my.app.input",
							Output: "my.app.output",
						},
					},
					Recovery: RecoveryConfig{},
				},
			},
			expectErr: false,
		},
		{
			name: "valid wildcard subject",
			exchange: &Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-exchange",
					Namespace: "default",
				},
				Spec: ExchangeSpec{
					Image: "test-image:latest",
					Stream: StreamConfig{
						Subjects: SubjectConfig{
							Input: "exchange.*.input",
						},
					},
					Recovery: RecoveryConfig{},
				},
			},
			expectErr: false,
		},
		{
			name: "negative max bytes",
			exchange: &Exchange{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-exchange",
					Namespace: "default",
				},
				Spec: ExchangeSpec{
					Image: "test-image:latest",
					Stream: StreamConfig{
						Retention: RetentionConfig{
							MaxBytes: resource.NewQuantity(-1, resource.BinarySI),
						},
					},
				},
			},
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
	exchange := &Exchange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-exchange",
			Namespace: "default",
		},
		Spec: ExchangeSpec{
			Image: "test-image:latest",
		},
	}

	// Call Default to set defaults
	exchange.Default()

	// Check that defaults were set
	if exchange.Spec.ImagePullPolicy != "IfNotPresent" {
		t.Errorf("Expected ImagePullPolicy to be 'IfNotPresent', got %v", exchange.Spec.ImagePullPolicy)
	}

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
