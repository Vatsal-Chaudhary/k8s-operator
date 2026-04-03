package controller

import (
	"math"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CalculateReplicas returns the desired replica count for the current lag.
func CalculateReplicas(lag int64, lagPerReplica int64, min int32, max int32) int32 {
	if lag <= 0 {
		return min
	}

	if lagPerReplica <= 0 {
		return min
	}

	desired := int32(math.Ceil(float64(lag) / float64(lagPerReplica)))
	if desired < min {
		return min
	}

	if desired > max {
		return max
	}

	return desired
}

// ShouldScale returns whether reconciliation should perform a scaling action.
func ShouldScale(
	lastScaleTime *metav1.Time,
	cooldownSeconds int,
	currentReplicas int32,
	desiredReplicas int32,
) bool {
	if currentReplicas == desiredReplicas {
		return false
	}

	if lastScaleTime == nil {
		return true
	}

	elapsed := time.Since(lastScaleTime.Time)
	return elapsed >= time.Duration(cooldownSeconds)*time.Second
}
