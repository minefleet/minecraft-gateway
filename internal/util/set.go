package util

// Set is a collection of unique elements
type Set[T comparable] struct {
	elements map[T]struct{}
}

// NewSet creates a new set
func NewSet[T comparable]() *Set[T] {
	return &Set[T]{
		elements: make(map[T]struct{}),
	}
}

// Add inserts an element into the set
func (s *Set[T]) Add(value T) {
	s.elements[value] = struct{}{}
}

// Remove deletes an element from the set
func (s *Set[T]) Remove(value T) {
	delete(s.elements, value)
}

// Contains checks if an element is in the set
func (s *Set[T]) Contains(value T) bool {
	_, found := s.elements[value]
	return found
}

// Size returns the number of elements in the set
func (s *Set[T]) Size() int {
	return len(s.elements)
}

// List returns all elements in the set as a slice
func (s *Set[T]) List() []T {
	if s.Size() == 0 {
		return nil
	}
	keys := make([]T, 0, len(s.elements))
	for key := range s.elements {
		keys = append(keys, key)
	}
	return keys
}
