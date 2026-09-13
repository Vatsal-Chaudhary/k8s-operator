package scaler

import (
	"math"
	"time"
)

// Reason describes why a scaling decision should or should not be acted on.
type Reason string

const (
	// ReasonReplicasUnchanged means the target already has the desired replica count.
	ReasonReplicasUnchanged Reason = "ReplicasUnchanged"
	// ReasonNeverScaled means no previous scale timestamp blocks the action.
	ReasonNeverScaled Reason = "NeverScaled"
	// ReasonCooldownActive means a replica change is desired but cooldown has not elapsed.
	ReasonCooldownActive Reason = "CooldownActive"
	// ReasonCooldownElapsed means a replica change is desired and cooldown has elapsed.
	ReasonCooldownElapsed Reason = "CooldownElapsed"
)

// Input contains the plain values needed to make a scaling decision.
type Input struct {
	CurrentLag      int64
	MinReplicas     int32
	MaxReplicas     int32
	LagPerReplica   int64
	Cooldown        time.Duration
	LastScaleTime   *time.Time
	CurrentReplicas int32
}

// Decision is the result of evaluating the scaler input.
type Decision struct {
	DesiredReplicas int32
	ScaleNow        bool
	Reason          Reason
}

// Decide returns the desired replica count and whether to scale now.
func Decide(input Input) Decision {
	desiredReplicas := CalculateReplicas(
		input.CurrentLag,
		input.LagPerReplica,
		input.MinReplicas,
		input.MaxReplicas,
	)

	scaleNow, reason := ShouldScale(
		input.LastScaleTime,
		input.Cooldown,
		input.CurrentReplicas,
		desiredReplicas,
	)

	return Decision{
		DesiredReplicas: desiredReplicas,
		ScaleNow:        scaleNow,
		Reason:          reason,
	}
}

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
	lastScaleTime *time.Time,
	cooldown time.Duration,
	currentReplicas int32,
	desiredReplicas int32,
) (bool, Reason) {
	if currentReplicas == desiredReplicas {
		return false, ReasonReplicasUnchanged
	}

	if lastScaleTime == nil {
		return true, ReasonNeverScaled
	}

	elapsed := time.Since(*lastScaleTime)
	if elapsed >= cooldown {
		return true, ReasonCooldownElapsed
	}

	return false, ReasonCooldownActive
}
