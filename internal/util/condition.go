package util

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func GetCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

func UpsertCondition(conditions *[]metav1.Condition, newC metav1.Condition) (changed bool) {
	now := metav1.Now()
	for i := range *conditions {
		c := &(*conditions)[i]
		if c.Type == newC.Type {
			if c.Status == newC.Status && c.Reason == newC.Reason && c.Message == newC.Message {
				return false
			}
			c.Status, c.Reason, c.Message = newC.Status, newC.Reason, newC.Message
			c.LastTransitionTime = now
			return true
		}
	}
	newC.LastTransitionTime = now
	*conditions = append(*conditions, newC)
	return true
}
