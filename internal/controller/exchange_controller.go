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

package controller

import (
	"context"
	"fmt"
	"math"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	conduitv1alpha1 "github.com/tonyd33/conduit/api/v1alpha1"
	natslib "github.com/tonyd33/conduit/internal/nats"
	podbuilder "github.com/tonyd33/conduit/internal/pod"
)

const (
	exchangeFinalizer = "conduit.mnke.org/finalizer"

	// Exponential backoff constants
	initialBackoffDelay = 10 * time.Second // Start with 10 seconds
	maxBackoffDelay     = 5 * time.Minute  // Cap at 5 minutes
)

// ExchangeReconciler reconciles a Exchange object
type ExchangeReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      record.EventRecorder
	NATSClient    *natslib.Client
	StreamManager *natslib.StreamManager
}

// +kubebuilder:rbac:groups=conduit.mnke.org,resources=exchanges,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=conduit.mnke.org,resources=exchanges/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=conduit.mnke.org,resources=exchanges/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// The reconciliation logic for Exchange resources:
// 1. Fetch the Exchange resource
// 2. Add finalizer for cleanup
// 3. Handle deletion (cleanup NATS stream)
// 4. Ensure NATS stream exists (Phase 2 - for now we'll stub this)
// 5. Ensure Pod exists and is running
// 6. Update Exchange status
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *ExchangeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Exchange resource
	exchange := &conduitv1alpha1.Exchange{}
	if err := r.Get(ctx, req.NamespacedName, exchange); err != nil {
		if errors.IsNotFound(err) {
			// Resource deleted, nothing to do
			logger.Info("Exchange resource not found, likely deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Exchange")
		return ctrl.Result{}, err
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(exchange, exchangeFinalizer) {
		logger.Info("Adding finalizer to Exchange")
		controllerutil.AddFinalizer(exchange, exchangeFinalizer)
		if err := r.Update(ctx, exchange); err != nil {
			logger.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Handle deletion
	if !exchange.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, exchange)
	}

	// Normal reconciliation
	return r.reconcileNormal(ctx, exchange)
}

// reconcileNormal handles the normal reconciliation logic
func (r *ExchangeReconciler) reconcileNormal(ctx context.Context, exchange *conduitv1alpha1.Exchange) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Generate stream and consumer names
	streamName := r.getStreamName(exchange)
	consumerName := r.getConsumerName(exchange)

	// Update status with stream/consumer names if not set
	if exchange.Status.StreamName == "" || exchange.Status.ConsumerName == "" {
		exchange.Status.StreamName = streamName
		exchange.Status.ConsumerName = consumerName
		if err := r.Status().Update(ctx, exchange); err != nil {
			logger.Error(err, "Failed to update Exchange status")
			return ctrl.Result{}, err
		}
	}

	// Ensure NATS stream exists
	if r.StreamManager != nil {
		logger.Info("Creating or updating NATS stream", "stream", streamName)
		if err := r.StreamManager.CreateOrUpdateStream(streamName, exchange); err != nil {
			logger.Error(err, "Failed to create NATS stream")
			r.Recorder.Event(exchange, corev1.EventTypeWarning, "StreamCreationFailed", fmt.Sprintf("Failed to create NATS stream: %v", err))
			exchange.Status.Phase = conduitv1alpha1.ExchangePhaseFailed
			r.setCondition(exchange, "StreamReady", metav1.ConditionFalse, "StreamCreationFailed", err.Error())
			_ = r.Status().Update(ctx, exchange)
			return ctrl.Result{}, err
		}
		r.Recorder.Event(exchange, corev1.EventTypeNormal, "StreamCreated", fmt.Sprintf("Created NATS stream %s", streamName))

		// Create durable consumer
		logger.Info("Creating NATS consumer", "consumer", consumerName)
		exchangeID := string(exchange.UID)
		subjects := []string{fmt.Sprintf("exchange.%s.input", exchangeID)}
		if err := r.StreamManager.CreateConsumer(streamName, consumerName, subjects); err != nil {
			logger.Error(err, "Failed to create NATS consumer")
			r.Recorder.Event(exchange, corev1.EventTypeWarning, "ConsumerCreationFailed", fmt.Sprintf("Failed to create NATS consumer: %v", err))
			exchange.Status.Phase = conduitv1alpha1.ExchangePhaseFailed
			r.setCondition(exchange, "StreamReady", metav1.ConditionFalse, "ConsumerCreationFailed", err.Error())
			_ = r.Status().Update(ctx, exchange)
			return ctrl.Result{}, err
		}
		r.Recorder.Event(exchange, corev1.EventTypeNormal, "ConsumerCreated", fmt.Sprintf("Created NATS consumer %s", consumerName))

		// Update status to indicate stream is ready
		r.setCondition(exchange, "StreamReady", metav1.ConditionTrue, "StreamReady", "NATS stream and consumer created")
		logger.Info("NATS stream and consumer ready")
	} else {
		logger.Info("NATS client not configured, skipping stream creation")
		r.setCondition(exchange, "StreamReady", metav1.ConditionUnknown, "NATSNotConfigured", "NATS client not configured")
	}

	// Set phase to Pending if not set
	if exchange.Status.Phase == "" {
		exchange.Status.Phase = conduitv1alpha1.ExchangePhasePending
		if err := r.Status().Update(ctx, exchange); err != nil {
			logger.Error(err, "Failed to update Exchange phase")
			return ctrl.Result{}, err
		}
	}

	// Ensure Pod exists
	pod := &corev1.Pod{}
	podName := fmt.Sprintf("%s-pod", exchange.Name)
	err := r.Get(ctx, client.ObjectKey{Namespace: exchange.Namespace, Name: podName}, pod)

	if err != nil && errors.IsNotFound(err) {
		// Pod doesn't exist, create it
		logger.Info("Creating Pod for Exchange")
		newPod, buildErr := podbuilder.BuildPodForExchange(exchange, streamName, consumerName)
		if buildErr != nil {
			logger.Error(buildErr, "Failed to build Pod spec")
			return ctrl.Result{}, buildErr
		}

		// Set Exchange as owner of the Pod
		if err := controllerutil.SetControllerReference(exchange, newPod, r.Scheme); err != nil {
			logger.Error(err, "Failed to set controller reference")
			return ctrl.Result{}, err
		}

		if err := r.Create(ctx, newPod); err != nil {
			logger.Error(err, "Failed to create Pod")
			r.Recorder.Event(exchange, corev1.EventTypeWarning, "PodCreationFailed", fmt.Sprintf("Failed to create pod: %v", err))
			exchange.Status.Phase = conduitv1alpha1.ExchangePhaseFailed
			r.setCondition(exchange, "PodReady", metav1.ConditionFalse, "PodCreationFailed", err.Error())
			_ = r.Status().Update(ctx, exchange)
			return ctrl.Result{}, err
		}

		// Update status
		exchange.Status.PodName = newPod.Name
		exchange.Status.Phase = conduitv1alpha1.ExchangePhasePending
		r.setCondition(exchange, "PodReady", metav1.ConditionFalse, "PodCreated", "Pod created, waiting for Running")
		if err := r.Status().Update(ctx, exchange); err != nil {
			logger.Error(err, "Failed to update Exchange status")
			return ctrl.Result{}, err
		}

		r.Recorder.Event(exchange, corev1.EventTypeNormal, "PodCreated", fmt.Sprintf("Created worker pod %s", newPod.Name))
		logger.Info("Pod created successfully", "pod", newPod.Name)
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get Pod")
		return ctrl.Result{}, err
	}

	// Pod exists, update status based on pod phase
	exchange.Status.PodName = pod.Name
	exchange.Status.PodPhase = pod.Status.Phase

	switch pod.Status.Phase {
	case corev1.PodRunning:
		if exchange.Status.Phase != conduitv1alpha1.ExchangePhaseRunning {
			logger.Info("Exchange is now running")
			exchange.Status.Phase = conduitv1alpha1.ExchangePhaseRunning
			r.setCondition(exchange, "PodReady", metav1.ConditionTrue, "PodRunning", "Pod is running")
			r.Recorder.Event(exchange, corev1.EventTypeNormal, "Running", "Exchange worker is now running")
		}
	case corev1.PodPending:
		if exchange.Status.Phase == "" || exchange.Status.Phase == conduitv1alpha1.ExchangePhaseCreated {
			exchange.Status.Phase = conduitv1alpha1.ExchangePhasePending
			r.setCondition(exchange, "PodReady", metav1.ConditionFalse, "PodPending", "Pod is pending")
		}
	case corev1.PodFailed, corev1.PodSucceeded:
		// Pod completed or failed - handle recovery
		logger.Info("Pod completed or failed", "phase", pod.Status.Phase)
		r.Recorder.Event(exchange, corev1.EventTypeWarning, "PodFailed", fmt.Sprintf("Worker pod %s failed with status %s", pod.Name, pod.Status.Phase))
		return r.handlePodFailure(ctx, exchange, pod)
	}

	// Update status
	if err := r.Status().Update(ctx, exchange); err != nil {
		logger.Error(err, "Failed to update Exchange status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// handlePodFailure handles pod failure and implements recovery logic with exponential backoff
func (r *ExchangeReconciler) handlePodFailure(ctx context.Context, exchange *conduitv1alpha1.Exchange, failedPod *corev1.Pod) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Get max restarts from spec (default to 5)
	maxRestarts := int32(5)
	if exchange.Spec.Recovery.MaxRestarts > 0 {
		maxRestarts = exchange.Spec.Recovery.MaxRestarts
	}

	// Check if we've exceeded max restarts
	if exchange.Status.Recovery.RestartCount >= maxRestarts {
		logger.Info("Max restarts exceeded", "restartCount", exchange.Status.Recovery.RestartCount)
		r.Recorder.Event(exchange, corev1.EventTypeWarning, "MaxRestartsExceeded",
			fmt.Sprintf("Maximum restart attempts (%d) exceeded", maxRestarts))
		exchange.Status.Phase = conduitv1alpha1.ExchangePhaseFailed
		r.setCondition(exchange, "PodReady", metav1.ConditionFalse, "MaxRestartsExceeded", "Maximum restart attempts exceeded")
		if err := r.Status().Update(ctx, exchange); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Check if this is a new failure or if we're still in backoff period
	now := time.Now()
	if exchange.Status.Recovery.LastRestartTime != nil {
		lastRestart := exchange.Status.Recovery.LastRestartTime.Time
		backoffDelay := calculateBackoff(exchange.Status.Recovery.RestartCount)
		nextRetryTime := lastRestart.Add(backoffDelay)

		if now.Before(nextRetryTime) {
			// Still in backoff period, requeue for later
			remainingDelay := nextRetryTime.Sub(now)
			logger.Info("In backoff period, requeueing",
				"restartCount", exchange.Status.Recovery.RestartCount,
				"backoffDelay", backoffDelay,
				"remainingDelay", remainingDelay,
				"nextRetryTime", nextRetryTime)

			exchange.Status.Phase = conduitv1alpha1.ExchangePhaseRecovering
			msg := fmt.Sprintf("Waiting for backoff period (retry in %s)", remainingDelay.Round(time.Second))
			r.setCondition(exchange, "PodReady", metav1.ConditionFalse, "BackoffPeriod", msg)
			r.Recorder.Event(exchange, corev1.EventTypeNormal, "BackoffWaiting",
				fmt.Sprintf("Waiting %s before retry (attempt %d/%d)", remainingDelay.Round(time.Second), exchange.Status.Recovery.RestartCount, maxRestarts))

			if err := r.Status().Update(ctx, exchange); err != nil {
				logger.Error(err, "Failed to update Exchange status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{RequeueAfter: remainingDelay}, nil
		}
	}

	// Backoff period has passed or this is first failure - proceed with recovery
	metaNow := metav1.Now()
	exchange.Status.Recovery.RestartCount++
	exchange.Status.Recovery.LastRestartTime = &metaNow
	exchange.Status.Phase = conduitv1alpha1.ExchangePhaseRecovering

	backoffDelay := calculateBackoff(exchange.Status.Recovery.RestartCount - 1) // Calculate based on previous count
	logger.Info("Recovering Exchange",
		"restartCount", exchange.Status.Recovery.RestartCount,
		"backoffDelay", backoffDelay)

	r.Recorder.Event(exchange, corev1.EventTypeNormal, "Recovering",
		fmt.Sprintf("Recovering from pod failure (attempt %d/%d)", exchange.Status.Recovery.RestartCount, maxRestarts))

	// Delete the failed pod
	if err := r.Delete(ctx, failedPod); err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "Failed to delete failed Pod")
		return ctrl.Result{}, err
	}

	// Update status
	r.setCondition(exchange, "PodReady", metav1.ConditionFalse, "Recovering", fmt.Sprintf("Recovering from pod failure (restart %d/%d)", exchange.Status.Recovery.RestartCount, maxRestarts))
	if err := r.Status().Update(ctx, exchange); err != nil {
		logger.Error(err, "Failed to update Exchange status")
		return ctrl.Result{}, err
	}

	// Requeue to create new pod
	logger.Info("Requeuing to create new pod")
	return ctrl.Result{Requeue: true}, nil
}

// calculateBackoff calculates the exponential backoff delay based on restart count
// Formula: delay = min(initialDelay * 2^restartCount, maxDelay)
func calculateBackoff(restartCount int32) time.Duration {
	if restartCount <= 0 {
		return initialBackoffDelay
	}

	// Calculate exponential backoff: initialDelay * 2^restartCount
	exponentialDelay := float64(initialBackoffDelay) * math.Pow(2, float64(restartCount))

	// Cap at maxBackoffDelay
	if exponentialDelay > float64(maxBackoffDelay) {
		return maxBackoffDelay
	}

	return time.Duration(exponentialDelay)
}

// reconcileDelete handles Exchange deletion
func (r *ExchangeReconciler) reconcileDelete(ctx context.Context, exchange *conduitv1alpha1.Exchange) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Deleting Exchange")

	// Delete NATS stream and consumer
	if r.StreamManager != nil && exchange.Status.StreamName != "" {
		streamName := exchange.Status.StreamName
		consumerName := exchange.Status.ConsumerName

		// Delete consumer first
		if consumerName != "" {
			logger.Info("Deleting NATS consumer", "consumer", consumerName)
			if err := r.StreamManager.DeleteConsumer(streamName, consumerName); err != nil {
				logger.Error(err, "Failed to delete NATS consumer", "consumer", consumerName)
				// Continue with stream deletion even if consumer deletion fails
			}
		}

		// Delete stream
		logger.Info("Deleting NATS stream", "stream", streamName)
		if err := r.StreamManager.DeleteStream(streamName); err != nil {
			logger.Error(err, "Failed to delete NATS stream", "stream", streamName)
			return ctrl.Result{}, err
		}
		logger.Info("NATS resources cleaned up successfully")
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(exchange, exchangeFinalizer)
	if err := r.Update(ctx, exchange); err != nil {
		logger.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	logger.Info("Exchange deleted successfully")
	return ctrl.Result{}, nil
}

// getStreamName returns the NATS stream name for the Exchange
func (r *ExchangeReconciler) getStreamName(exchange *conduitv1alpha1.Exchange) string {
	return fmt.Sprintf("stream-exchange-%s", exchange.UID)
}

// getConsumerName returns the NATS consumer name for the Exchange
func (r *ExchangeReconciler) getConsumerName(exchange *conduitv1alpha1.Exchange) string {
	return fmt.Sprintf("exchange-%s-consumer", exchange.UID)
}

// setCondition updates or adds a condition to the Exchange status
func (r *ExchangeReconciler) setCondition(exchange *conduitv1alpha1.Exchange, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: exchange.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	// Find and update existing condition or append new one
	found := false
	for i, c := range exchange.Status.Conditions {
		if c.Type == conditionType {
			// Only update if status changed
			if c.Status != status {
				exchange.Status.Conditions[i] = condition
			}
			found = true
			break
		}
	}
	if !found {
		exchange.Status.Conditions = append(exchange.Status.Conditions, condition)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExchangeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&conduitv1alpha1.Exchange{}).
		Owns(&corev1.Pod{}).
		Named("exchange").
		Complete(r)
}
