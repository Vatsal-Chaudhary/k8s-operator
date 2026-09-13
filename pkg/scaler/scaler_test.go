package scaler

import (
	"testing"
	"time"
)

func TestCalculateReplicas(t *testing.T) {
	tests := []struct {
		name          string
		lag           int64
		lagPerReplica int64
		min           int32
		max           int32
		expected      int32
	}{
		{
			name:          "returns min when lag is zero",
			lag:           0,
			lagPerReplica: 1000,
			min:           2,
			max:           20,
			expected:      2,
		},
		{
			name:          "returns min when lag is below threshold",
			lag:           500,
			lagPerReplica: 1000,
			min:           2,
			max:           20,
			expected:      2,
		},
		{
			name:          "returns exact single replica when lag matches threshold",
			lag:           1000,
			lagPerReplica: 1000,
			min:           1,
			max:           20,
			expected:      1,
		},
		{
			name:          "returns min when exact single replica is below min",
			lag:           1000,
			lagPerReplica: 1000,
			min:           2,
			max:           20,
			expected:      2,
		},
		{
			name:          "rounds up partial replica demand",
			lag:           4500,
			lagPerReplica: 1000,
			min:           2,
			max:           20,
			expected:      5,
		},
		{
			name:          "clamps desired replicas to max",
			lag:           25000,
			lagPerReplica: 1000,
			min:           2,
			max:           20,
			expected:      20,
		},
		{
			name:          "returns min when lag per replica is zero",
			lag:           5000,
			lagPerReplica: 0,
			min:           2,
			max:           20,
			expected:      2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateReplicas(tt.lag, tt.lagPerReplica, tt.min, tt.max)
			if result != tt.expected {
				t.Fatalf("CalculateReplicas(%d, %d, %d, %d) = %d, want %d", tt.lag, tt.lagPerReplica, tt.min, tt.max, result, tt.expected)
			}
		})
	}
}

func TestShouldScale(t *testing.T) {
	now := time.Now()
	inCooldown := now.Add(-10 * time.Second)
	cooldownElapsed := now.Add(-200 * time.Second)

	tests := []struct {
		name            string
		lastScaleTime   *time.Time
		cooldown        time.Duration
		currentReplicas int32
		desiredReplicas int32
		expected        bool
		expectedReason  Reason
	}{
		{
			name:            "returns false when replicas already match",
			lastScaleTime:   nil,
			cooldown:        120 * time.Second,
			currentReplicas: 5,
			desiredReplicas: 5,
			expected:        false,
			expectedReason:  ReasonReplicasUnchanged,
		},
		{
			name:            "returns true when never scaled before",
			lastScaleTime:   nil,
			cooldown:        120 * time.Second,
			currentReplicas: 2,
			desiredReplicas: 5,
			expected:        true,
			expectedReason:  ReasonNeverScaled,
		},
		{
			name:            "returns false while in cooldown",
			lastScaleTime:   &inCooldown,
			cooldown:        120 * time.Second,
			currentReplicas: 2,
			desiredReplicas: 5,
			expected:        false,
			expectedReason:  ReasonCooldownActive,
		},
		{
			name:            "returns true after cooldown elapses",
			lastScaleTime:   &cooldownElapsed,
			cooldown:        120 * time.Second,
			currentReplicas: 2,
			desiredReplicas: 5,
			expected:        true,
			expectedReason:  ReasonCooldownElapsed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, reason := ShouldScale(tt.lastScaleTime, tt.cooldown, tt.currentReplicas, tt.desiredReplicas)
			if result != tt.expected {
				t.Fatalf("ShouldScale(%v, %s, %d, %d) = %t, want %t", tt.lastScaleTime, tt.cooldown, tt.currentReplicas, tt.desiredReplicas, result, tt.expected)
			}

			if reason != tt.expectedReason {
				t.Fatalf("ShouldScale(%v, %s, %d, %d) reason = %q, want %q", tt.lastScaleTime, tt.cooldown, tt.currentReplicas, tt.desiredReplicas, reason, tt.expectedReason)
			}
		})
	}
}

func TestDecide(t *testing.T) {
	lastScaleTime := time.Now().Add(-200 * time.Second)

	decision := Decide(Input{
		CurrentLag:      4500,
		MinReplicas:     2,
		MaxReplicas:     20,
		LagPerReplica:   1000,
		Cooldown:        120 * time.Second,
		LastScaleTime:   &lastScaleTime,
		CurrentReplicas: 2,
	})

	if decision.DesiredReplicas != 5 {
		t.Fatalf("Decision.DesiredReplicas = %d, want 5", decision.DesiredReplicas)
	}

	if !decision.ScaleNow {
		t.Fatal("Decision.ScaleNow = false, want true")
	}

	if decision.Reason != ReasonCooldownElapsed {
		t.Fatalf("Decision.Reason = %q, want %q", decision.Reason, ReasonCooldownElapsed)
	}
}
