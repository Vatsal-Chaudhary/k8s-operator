// Package scaler contains the Kubernetes-agnostic scaling decision logic.
//
// It intentionally operates on plain Go values rather than Kubernetes API
// objects, controller-runtime types, or this operator's CRD types. Keeping the
// arithmetic and cooldown rules decoupled makes the decision logic independently
// testable and allows the same package to drive a non-Kubernetes scaling target
// in the future.
package scaler
