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
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var exchangelog = logf.Log.WithName("exchange-resource")

// SetupWebhookWithManager sets up the webhook with the Manager.
func (r *Exchange) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-conduit-mnke-org-v1alpha1-exchange,mutating=true,failurePolicy=fail,sideEffects=None,groups=conduit.mnke.org,resources=exchanges,verbs=create;update,versions=v1alpha1,name=mexchange.kb.io,admissionReviewVersions=v1

// Default implements defaulting for the Exchange type
func (r *Exchange) Default() {
	exchangelog.Info("default", "name", r.Name)

	// Set default retention config
	if r.Spec.Stream.Retention.MaxAge.Duration == 0 {
		r.Spec.Stream.Retention.MaxAge.Duration = 168 * 3600 * 1000000000 // 168 hours
	}
	if r.Spec.Stream.Retention.MaxMessages == 0 {
		r.Spec.Stream.Retention.MaxMessages = 100000
	}

	// Set default recovery config
	if r.Spec.Recovery.MaxRestarts == 0 {
		r.Spec.Recovery.MaxRestarts = 5
	}
	if r.Spec.Recovery.BackoffLimit == 0 {
		r.Spec.Recovery.BackoffLimit = 3
	}
}

// +kubebuilder:webhook:path=/validate-conduit-mnke-org-v1alpha1-exchange,mutating=false,failurePolicy=fail,sideEffects=None,groups=conduit.mnke.org,resources=exchanges,verbs=create;update,versions=v1alpha1,name=vexchange.kb.io,admissionReviewVersions=v1

// ValidateCreate implements validation for Exchange creation
func (r *Exchange) ValidateCreate() (admission.Warnings, error) {
	exchangelog.Info("validate create", "name", r.Name)

	var allErrs field.ErrorList

	// Validate spec
	if err := r.validateExchangeSpec(); err != nil {
		allErrs = append(allErrs, err...)
	}

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, allErrs.ToAggregate()
}

// ValidateUpdate implements validation for Exchange updates
func (r *Exchange) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	exchangelog.Info("validate update", "name", r.Name)

	var allErrs field.ErrorList

	// Validate spec
	if err := r.validateExchangeSpec(); err != nil {
		allErrs = append(allErrs, err...)
	}

	// Validate immutable fields
	oldExchange := old.(*Exchange)
	if err := r.validateImmutableFields(oldExchange); err != nil {
		allErrs = append(allErrs, err...)
	}

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, allErrs.ToAggregate()
}

// ValidateDelete implements validation for Exchange deletion
func (r *Exchange) ValidateDelete() (admission.Warnings, error) {
	exchangelog.Info("validate delete", "name", r.Name)

	// No validation needed for delete
	return nil, nil
}

// validateExchangeSpec validates the Exchange spec
func (r *Exchange) validateExchangeSpec() field.ErrorList {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")

	// Validate that at least one container is specified in the template
	if len(r.Spec.Template.Spec.Containers) == 0 {
		allErrs = append(allErrs, field.Required(
			specPath.Child("template", "spec", "containers"),
			"at least one container must be specified",
		))
	}

	// Validate retention config
	if err := r.validateRetentionConfig(); err != nil {
		allErrs = append(allErrs, err...)
	}

	// Validate subject config
	if err := r.validateSubjectConfig(); err != nil {
		allErrs = append(allErrs, err...)
	}

	// Validate recovery config
	if err := r.validateRecoveryConfig(); err != nil {
		allErrs = append(allErrs, err...)
	}

	// Validate environment variables in all containers
	if err := r.validateEnvVars(); err != nil {
		allErrs = append(allErrs, err...)
	}

	return allErrs
}

// validateRetentionConfig validates retention configuration
func (r *Exchange) validateRetentionConfig() field.ErrorList {
	var allErrs field.ErrorList
	retentionPath := field.NewPath("spec", "stream", "retention")

	// Validate MaxAge is positive
	if r.Spec.Stream.Retention.MaxAge.Duration < 0 {
		allErrs = append(allErrs, field.Invalid(
			retentionPath.Child("maxAge"),
			r.Spec.Stream.Retention.MaxAge.Duration,
			"maxAge must be positive",
		))
	}

	// Validate MaxMessages is positive
	if r.Spec.Stream.Retention.MaxMessages < 0 {
		allErrs = append(allErrs, field.Invalid(
			retentionPath.Child("maxMessages"),
			r.Spec.Stream.Retention.MaxMessages,
			"maxMessages must be positive",
		))
	}

	// Validate MaxBytes if set
	if r.Spec.Stream.Retention.MaxBytes != nil {
		if r.Spec.Stream.Retention.MaxBytes.Sign() < 0 {
			allErrs = append(allErrs, field.Invalid(
				retentionPath.Child("maxBytes"),
				r.Spec.Stream.Retention.MaxBytes.String(),
				"maxBytes must be positive",
			))
		}
	}

	return allErrs
}

// validateSubjectConfig validates NATS subject configuration
func (r *Exchange) validateSubjectConfig() field.ErrorList {
	var allErrs field.ErrorList
	subjectsPath := field.NewPath("spec", "stream", "subjects")

	// Validate input subject if specified
	if r.Spec.Stream.Subjects.Input != "" {
		if err := validateSubjectName(r.Spec.Stream.Subjects.Input); err != nil {
			allErrs = append(allErrs, field.Invalid(
				subjectsPath.Child("input"),
				r.Spec.Stream.Subjects.Input,
				err.Error(),
			))
		}
	}

	// Validate output subject if specified
	if r.Spec.Stream.Subjects.Output != "" {
		if err := validateSubjectName(r.Spec.Stream.Subjects.Output); err != nil {
			allErrs = append(allErrs, field.Invalid(
				subjectsPath.Child("output"),
				r.Spec.Stream.Subjects.Output,
				err.Error(),
			))
		}
	}

	// Validate control subject if specified
	if r.Spec.Stream.Subjects.Control != "" {
		if err := validateSubjectName(r.Spec.Stream.Subjects.Control); err != nil {
			allErrs = append(allErrs, field.Invalid(
				subjectsPath.Child("control"),
				r.Spec.Stream.Subjects.Control,
				err.Error(),
			))
		}
	}

	// Ensure subjects are unique if specified
	subjects := []string{}
	if r.Spec.Stream.Subjects.Input != "" {
		subjects = append(subjects, r.Spec.Stream.Subjects.Input)
	}
	if r.Spec.Stream.Subjects.Output != "" {
		subjects = append(subjects, r.Spec.Stream.Subjects.Output)
	}
	if r.Spec.Stream.Subjects.Control != "" {
		subjects = append(subjects, r.Spec.Stream.Subjects.Control)
	}

	seen := make(map[string]bool)
	for _, subject := range subjects {
		if seen[subject] {
			allErrs = append(allErrs, field.Invalid(
				subjectsPath,
				subject,
				"subjects must be unique (input, output, and control cannot be the same)",
			))
			break
		}
		seen[subject] = true
	}

	return allErrs
}

// validateSubjectName validates a NATS subject name
func validateSubjectName(subject string) error {
	if subject == "" {
		return fmt.Errorf("subject cannot be empty")
	}

	// NATS subject restrictions:
	// - Cannot contain spaces
	// - Cannot start or end with '.'
	// - Cannot have consecutive dots '..'
	// - Valid characters: alphanumeric, '.', '-', '_', '*', '>'

	if strings.Contains(subject, " ") {
		return fmt.Errorf("subject cannot contain spaces")
	}

	if strings.HasPrefix(subject, ".") || strings.HasSuffix(subject, ".") {
		return fmt.Errorf("subject cannot start or end with '.'")
	}

	if strings.Contains(subject, "..") {
		return fmt.Errorf("subject cannot contain consecutive dots")
	}

	// Check for valid characters
	validChars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.-_*>"
	for _, c := range subject {
		if !strings.ContainsRune(validChars, c) {
			return fmt.Errorf("subject contains invalid character '%c' (valid: alphanumeric, '.', '-', '_', '*', '>')", c)
		}
	}

	return nil
}

// validateRecoveryConfig validates recovery configuration
func (r *Exchange) validateRecoveryConfig() field.ErrorList {
	var allErrs field.ErrorList
	recoveryPath := field.NewPath("spec", "recovery")

	// Validate MaxRestarts is non-negative
	if r.Spec.Recovery.MaxRestarts < 0 {
		allErrs = append(allErrs, field.Invalid(
			recoveryPath.Child("maxRestarts"),
			r.Spec.Recovery.MaxRestarts,
			"maxRestarts must be non-negative",
		))
	}

	// Validate BackoffLimit is non-negative
	if r.Spec.Recovery.BackoffLimit < 0 {
		allErrs = append(allErrs, field.Invalid(
			recoveryPath.Child("backoffLimit"),
			r.Spec.Recovery.BackoffLimit,
			"backoffLimit must be non-negative",
		))
	}

	return allErrs
}

// validateEnvVars validates environment variables
func (r *Exchange) validateEnvVars() field.ErrorList {
	var allErrs field.ErrorList

	// Reserved environment variable prefixes that the operator uses
	reservedPrefixes := []string{
		"EXCHANGE_",
		"NATS_",
		"CONDUIT_",
	}

	// Check environment variables in all containers
	containersPath := field.NewPath("spec", "template", "spec", "containers")
	for containerIdx, container := range r.Spec.Template.Spec.Containers {
		envPath := containersPath.Index(containerIdx).Child("env")
		for i, env := range container.Env {
			for _, prefix := range reservedPrefixes {
				if strings.HasPrefix(env.Name, prefix) {
					allErrs = append(allErrs, field.Forbidden(
						envPath.Index(i).Child("name"),
						fmt.Sprintf("environment variable name cannot start with reserved prefix '%s'", prefix),
					))
				}
			}
		}
	}

	return allErrs
}

// validateImmutableFields validates that immutable fields have not changed
func (r *Exchange) validateImmutableFields(old *Exchange) field.ErrorList {
	var allErrs field.ErrorList

	// Currently, we don't have any strictly immutable fields
	// Image can be updated (for rolling updates)
	// Stream subjects can technically be updated (though it may cause issues)
	// Recovery config can be updated

	// If we want to make subjects immutable in the future:
	/*
		if r.Spec.Stream.Subjects.Input != old.Spec.Stream.Subjects.Input {
			allErrs = append(allErrs, field.Forbidden(
				field.NewPath("spec", "stream", "subjects", "input"),
				"input subject is immutable after creation",
			))
		}
	*/

	return allErrs
}
