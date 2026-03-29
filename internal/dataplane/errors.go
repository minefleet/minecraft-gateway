package dataplane

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"
)

type RouteConflictError struct {
	Conflicting map[types.NamespacedName]types.NamespacedName
}

func (e RouteConflictError) Error() string {
	return fmt.Sprintf("Conflicting routes: %v", e.Conflicting)
}
