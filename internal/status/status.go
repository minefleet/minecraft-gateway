package status

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Status interface {
	EvaluateConditions() []metav1.Condition
}
