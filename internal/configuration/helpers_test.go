package configuration

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBoolPtr(t *testing.T) {
	ptr := BoolPtr(true)
	assert.NotNil(t, ptr)
	assert.True(t, *ptr)

	ptr2 := BoolPtr(false)
	assert.NotNil(t, ptr2)
	assert.False(t, *ptr2)
}

func TestStringPtr(t *testing.T) {
	ptr := StringPtr("test")
	assert.NotNil(t, ptr)
	assert.Equal(t, "test", *ptr)

	ptr2 := StringPtr("")
	assert.NotNil(t, ptr2)
	assert.Equal(t, "", *ptr2)
}

func TestIntPtr(t *testing.T) {
	ptr := IntPtr(42)
	assert.NotNil(t, ptr)
	assert.Equal(t, 42, *ptr)

	ptr2 := IntPtr(0)
	assert.NotNil(t, ptr2)
	assert.Equal(t, 0, *ptr2)
}

func TestFloat64Ptr(t *testing.T) {
	ptr := Float64Ptr(3.14)
	assert.NotNil(t, ptr)
	assert.Equal(t, 3.14, *ptr)

	ptr2 := Float64Ptr(0.0)
	assert.NotNil(t, ptr2)
	assert.Equal(t, 0.0, *ptr2)
}

func TestBoolVal(t *testing.T) {
	truePtr := BoolPtr(true)
	assert.True(t, BoolVal(truePtr, false))

	assert.False(t, BoolVal(nil, false))

	assert.True(t, BoolVal(nil, true))
}

func TestStringVal(t *testing.T) {
	ptr := StringPtr("hello")
	assert.Equal(t, "hello", StringVal(ptr, "default"))

	assert.Equal(t, "default", StringVal(nil, "default"))

	assert.Equal(t, "", StringVal(nil, ""))
}

func TestIntVal(t *testing.T) {
	ptr := IntPtr(100)
	assert.Equal(t, 100, IntVal(ptr, 0))

	assert.Equal(t, 0, IntVal(nil, 0))

	assert.Equal(t, -1, IntVal(nil, -1))
}

func TestFloat64Val(t *testing.T) {
	ptr := Float64Ptr(2.71)
	assert.Equal(t, 2.71, Float64Val(ptr, 0.0))

	assert.Equal(t, 0.0, Float64Val(nil, 0.0))

	assert.Equal(t, 1.5, Float64Val(nil, 1.5))
}
