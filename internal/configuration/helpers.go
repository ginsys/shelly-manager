package configuration

// Pointer helper functions for easier struct initialization in tests and code.
// These functions create pointers to literal values, which is useful when
// working with pointer-based structs that support nil = "inherit" semantics.

// BoolPtr returns a pointer to the given bool value.
func BoolPtr(v bool) *bool {
	return &v
}

// StringPtr returns a pointer to the given string value.
func StringPtr(v string) *string {
	return &v
}

// IntPtr returns a pointer to the given int value.
func IntPtr(v int) *int {
	return &v
}

// Float64Ptr returns a pointer to the given float64 value.
func Float64Ptr(v float64) *float64 {
	return &v
}

// Dereference helpers with default values for safe nil handling.

// BoolVal returns the value pointed to by ptr, or def if ptr is nil.
func BoolVal(ptr *bool, def bool) bool {
	if ptr == nil {
		return def
	}
	return *ptr
}

// StringVal returns the value pointed to by ptr, or def if ptr is nil.
func StringVal(ptr *string, def string) string {
	if ptr == nil {
		return def
	}
	return *ptr
}

// IntVal returns the value pointed to by ptr, or def if ptr is nil.
func IntVal(ptr *int, def int) int {
	if ptr == nil {
		return def
	}
	return *ptr
}

// Float64Val returns the value pointed to by ptr, or def if ptr is nil.
func Float64Val(ptr *float64, def float64) float64 {
	if ptr == nil {
		return def
	}
	return *ptr
}
