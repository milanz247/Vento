package support

// Map applies fn to every element of in, returning a new slice of the
// results - Go's shape of Laravel's $collection->map(), most often reached
// for to turn a slice of DB models into a slice of API response DTOs
// without a hand-written for loop on every list endpoint:
//
//	dtos := support.Map(users, func(u models.User) UserResponse {
//	    return UserResponse{ID: u.ID, Name: u.Name}
//	})
func Map[T, U any](in []T, fn func(T) U) []U {
	out := make([]U, len(in))
	for i, v := range in {
		out[i] = fn(v)
	}
	return out
}

// Filter returns the elements of in for which keep reports true - Go's
// shape of Laravel's $collection->filter(). Always returns a non-nil
// slice (even when nothing matches), so callers can range over the result
// or json.Marshal it as [] rather than null without a nil check.
func Filter[T any](in []T, keep func(T) bool) []T {
	out := make([]T, 0, len(in))
	for _, v := range in {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}
