package util

import (
	"k8s.io/apimachinery/pkg/api/validate"
)

func ListIncludes[T any](arr []T, obj T, compareFunc validate.MatchFunc[T]) bool {
	for _, e := range arr {
		if compareFunc(e, obj) {
			return true
		}
	}
	return false
}

func ListMissing[T any](old []T, new []T, compareFunc validate.MatchFunc[T]) []T {
	result := make([]T, 0)
	for _, e := range old {
		if !ListIncludes(new, e, compareFunc) {
			result = append(result, e)
		}
	}
	return result
}
