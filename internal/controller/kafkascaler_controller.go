package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/Vatsal-Chaudhary/k8s-operator/api/v1alpha1"
)

const kafkaScalerFinalizer = "autoscaling.kafkascaler.io/finalizer"

// KafkaScalerReconciler reconciles a KafkaScaler object.
type KafkaScalerReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	LagFetcher LagFetcher
}

func (r *KafkaScalerReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("kafkascaler", req.NamespacedName)

	logger.Info("Reconciling KafkaScaler")

	scaler := &v1alpha1.KafkaScaler{}
	if err := r.Get(ctx, req.NamespacedName, scaler); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("KafkaScaler resource not found")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("getting KafkaScaler: %w", err)
	}

	if scaler.DeletionTimestamp != nil {
		logger.Info("KafkaScaler is being deleted")

		if controllerutil.ContainsFinalizer(scaler, kafkaScalerFinalizer) {
			if err := r.cleanup(ctx, scaler); err != nil {
				return ctrl.Result{}, fmt.Errorf("cleaning up KafkaScaler: %w", err)
			}

			controllerutil.RemoveFinalizer(scaler, kafkaScalerFinalizer)
			if err := r.Update(ctx, scaler); err != nil {
				return ctrl.Result{}, fmt.Errorf("removing KafkaScaler finalizer: %w", err)
			}
		}

		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(scaler, kafkaScalerFinalizer) {
		logger.Info("Adding finalizer to KafkaScaler")

		controllerutil.AddFinalizer(scaler, kafkaScalerFinalizer)
		if err := r.Update(ctx, scaler); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding KafkaScaler finalizer: %w", err)
		}

		return ctrl.Result{}, nil
	}

	logger.Info(
		"Fetching Kafka lag",
		"topic", scaler.Spec.Topic,
		"consumerGroup", scaler.Spec.ConsumerGroup,
		"brokers", scaler.Spec.KafkaBrokers,
	)

	lag, err := r.LagFetcher.GetConsumerLag(
		ctx,
		scaler.Spec.KafkaBrokers,
		scaler.Spec.Topic,
		scaler.Spec.ConsumerGroup,
	)
	if err != nil {
		wrappedErr := fmt.Errorf("fetching consumer lag: %w", err)
		logger.Error(wrappedErr, "Failed to fetch Kafka lag")

		setCondition(scaler, "Degraded", metav1.ConditionTrue, "KafkaLagFetchFailed", wrappedErr.Error())
		if statusErr := r.Status().Update(ctx, scaler); statusErr != nil {
			return ctrl.Result{}, fmt.Errorf("updating KafkaScaler status after lag fetch failure: %w", statusErr)
		}

		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	logger.Info("Fetched Kafka lag", "lag", lag)

	desiredReplicas := CalculateReplicas(
		lag,
		scaler.Spec.LagPerReplica,
		scaler.Spec.MinReplicas,
		scaler.Spec.MaxReplicas,
	)

	logger.Info(
		"Calculated desired replicas",
		"currentReplicas", scaler.Status.CurrentReplicas,
		"desiredReplicas", desiredReplicas,
		"lag", lag,
	)

	if !ShouldScale(
		scaler.Status.LastScaleTime,
		scaler.Spec.CooldownSeconds,
		scaler.Status.CurrentReplicas,
		desiredReplicas,
	) {
		logger.Info(
			"Skipping scale action",
			"currentReplicas", scaler.Status.CurrentReplicas,
			"desiredReplicas", desiredReplicas,
			"lastScaleTime", scaler.Status.LastScaleTime,
			"cooldownSeconds", scaler.Spec.CooldownSeconds,
		)

		scaler.Status.CurrentLag = lag
		if err := r.Status().Update(ctx, scaler); err != nil {
			return ctrl.Result{}, fmt.Errorf("updating KafkaScaler status without scaling: %w", err)
		}

		return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
	}

	deployment := &appsv1.Deployment{}
	deploymentKey := client.ObjectKey{
		Namespace: scaler.Spec.TargetNamespace,
		Name:      scaler.Spec.DeploymentName,
	}

	logger.Info(
		"Fetching target Deployment",
		"deploymentNamespace", deploymentKey.Namespace,
		"deploymentName", deploymentKey.Name,
	)

	if err := r.Get(ctx, deploymentKey, deployment); err != nil {
		if apierrors.IsNotFound(err) {
			message := fmt.Sprintf(
				"target deployment %s/%s not found",
				scaler.Spec.TargetNamespace,
				scaler.Spec.DeploymentName,
			)
			logger.Error(fmt.Errorf("getting target Deployment: %w", err), "Target Deployment not found")

			scaler.Status.CurrentLag = lag
			setCondition(scaler, "DeploymentNotFound", metav1.ConditionTrue, "DeploymentNotFound", message)
			if statusErr := r.Status().Update(ctx, scaler); statusErr != nil {
				return ctrl.Result{}, fmt.Errorf("updating KafkaScaler status for missing Deployment: %w", statusErr)
			}

			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}

		return ctrl.Result{}, fmt.Errorf("getting target Deployment: %w", err)
	}

	patch := client.MergeFrom(deployment.DeepCopy())
	deployment.Spec.Replicas = &desiredReplicas

	logger.Info(
		"Patching target Deployment replicas",
		"deploymentNamespace", deployment.Namespace,
		"deploymentName", deployment.Name,
		"replicas", desiredReplicas,
	)

	if err := r.Patch(ctx, deployment, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("patching target Deployment replicas: %w", err)
	}

	now := metav1.Now()
	scaler.Status.CurrentReplicas = desiredReplicas
	scaler.Status.CurrentLag = lag
	scaler.Status.LastScaleTime = &now
	setCondition(
		scaler,
		"Scaled",
		metav1.ConditionTrue,
		"LagThresholdExceeded",
		fmt.Sprintf("scaled deployment to %d replicas for lag %d", desiredReplicas, lag),
	)

	logger.Info(
		"Updating KafkaScaler status after scale action",
		"currentReplicas", scaler.Status.CurrentReplicas,
		"currentLag", scaler.Status.CurrentLag,
		"lastScaleTime", scaler.Status.LastScaleTime,
	)

	if err := r.Status().Update(ctx, scaler); err != nil {
		return ctrl.Result{}, fmt.Errorf("updating KafkaScaler status after scaling: %w", err)
	}

	logger.Info("Scale action completed", "requeueAfter", 60*time.Second)

	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil
}

func (r *KafkaScalerReconciler) cleanup(
	ctx context.Context,
	scaler *v1alpha1.KafkaScaler,
) error {
	logger := log.FromContext(ctx)
	logger.Info("Running KafkaScaler cleanup", "namespace", scaler.Namespace, "name", scaler.Name)

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *KafkaScalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.KafkaScaler{}).
		Complete(r); err != nil {
		return fmt.Errorf("setting up KafkaScaler controller: %w", err)
	}

	return nil
}

func setCondition(
	scaler *v1alpha1.KafkaScaler,
	condType string,
	status metav1.ConditionStatus,
	reason string,
	message string,
) {
	meta.SetStatusCondition(&scaler.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: scaler.Generation,
	})
}
