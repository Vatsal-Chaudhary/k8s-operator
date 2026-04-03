package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	inCooldown := metav1.NewTime(now.Add(-10 * time.Second))
	cooldownElapsed := metav1.NewTime(now.Add(-200 * time.Second))

	tests := []struct {
		name            string
		lastScaleTime   *metav1.Time
		cooldownSeconds int
		currentReplicas int32
		desiredReplicas int32
		expected        bool
	}{
		{
			name:            "returns false when replicas already match",
			lastScaleTime:   nil,
			cooldownSeconds: 120,
			currentReplicas: 5,
			desiredReplicas: 5,
			expected:        false,
		},
		{
			name:            "returns true when never scaled before",
			lastScaleTime:   nil,
			cooldownSeconds: 120,
			currentReplicas: 2,
			desiredReplicas: 5,
			expected:        true,
		},
		{
			name:            "returns false while in cooldown",
			lastScaleTime:   &inCooldown,
			cooldownSeconds: 120,
			currentReplicas: 2,
			desiredReplicas: 5,
			expected:        false,
		},
		{
			name:            "returns true after cooldown elapses",
			lastScaleTime:   &cooldownElapsed,
			cooldownSeconds: 120,
			currentReplicas: 2,
			desiredReplicas: 5,
			expected:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldScale(tt.lastScaleTime, tt.cooldownSeconds, tt.currentReplicas, tt.desiredReplicas)
			if result != tt.expected {
				t.Fatalf("ShouldScale(%v, %d, %d, %d) = %t, want %t", tt.lastScaleTime, tt.cooldownSeconds, tt.currentReplicas, tt.desiredReplicas, result, tt.expected)
			}
		})
	}
}
