package shelly

import (
	"errors"
	"testing"
)

// Helper functions are now in testhelpers_test.go

func TestPredefinedErrors(t *testing.T) {
	// Test that all predefined errors have meaningful messages
	assertEqual(t, "device not found or unreachable", ErrDeviceNotFound.Error())
	assertEqual(t, "authentication required", ErrAuthRequired.Error())
	assertEqual(t, "authentication failed", ErrAuthFailed.Error())
	assertEqual(t, "unsupported device generation", ErrInvalidGeneration.Error())
	assertEqual(t, "operation not supported by this device", ErrOperationNotSupported.Error())
	assertEqual(t, "invalid response from device", ErrInvalidResponse.Error())
	assertEqual(t, "operation timed out", ErrTimeout.Error())
	assertEqual(t, "connection failed", ErrConnectionFailed.Error())
	assertEqual(t, "invalid configuration", ErrConfigurationInvalid.Error())
	assertEqual(t, "firmware update already in progress", ErrFirmwareUpdateInProgress.Error())
	assertEqual(t, "no firmware update available", ErrNoUpdateAvailable.Error())
	assertEqual(t, "device is busy", ErrDeviceBusy.Error())
	assertEqual(t, "device overtemperature protection active", ErrOvertemperature.Error())
	assertEqual(t, "power limit exceeded", ErrPowerLimit.Error())
}

func TestDeviceError_ErrorWithErr(t *testing.T) {
	underlying := errors.New("connection timeout")
	deviceErr := &DeviceError{
		IP:         "192.168.1.100",
		Generation: 2,
		Operation:  "GetInfo",
		Err:        underlying,
	}

	expected := "device 192.168.1.100 (gen2) GetInfo failed: connection timeout"
	assertEqual(t, expected, deviceErr.Error())
}

func TestDeviceError_ErrorWithStatusCode(t *testing.T) {
	deviceErr := &DeviceError{
		IP:         "192.168.1.100",
		Generation: 1,
		Operation:  "GetStatus",
		StatusCode: 404,
		Message:    "Not Found",
	}

	expected := "device 192.168.1.100 (gen1) GetStatus failed with status 404: Not Found"
	assertEqual(t, expected, deviceErr.Error())
}

func TestDeviceError_ErrorWithMessage(t *testing.T) {
	deviceErr := &DeviceError{
		IP:         "192.168.1.100",
		Generation: 2,
		Operation:  "SetConfig",
		Message:    "Invalid configuration parameter",
	}

	expected := "device 192.168.1.100 (gen2) SetConfig failed: Invalid configuration parameter"
	assertEqual(t, expected, deviceErr.Error())
}

func TestDeviceError_Unwrap(t *testing.T) {
	underlying := errors.New("network error")
	deviceErr := &DeviceError{
		IP:         "192.168.1.100",
		Generation: 2,
		Operation:  "GetInfo",
		Err:        underlying,
	}

	unwrapped := deviceErr.Unwrap()
	assertEqual(t, underlying, unwrapped)
}

func TestDeviceError_UnwrapNil(t *testing.T) {
	deviceErr := &DeviceError{
		IP:         "192.168.1.100",
		Generation: 2,
		Operation:  "GetInfo",
		Message:    "Some error",
	}

	unwrapped := deviceErr.Unwrap()
	assertEqual(t, error(nil), unwrapped)
}

func TestIsAuthError(t *testing.T) {
	// Test with nil error
	assertEqual(t, false, IsAuthError(nil))

	// Test with authentication required error
	assertTrue(t, IsAuthError(ErrAuthRequired))

	// Test with authentication failed error
	assertTrue(t, IsAuthError(ErrAuthFailed))

	// Test with wrapped authentication errors
	wrappedAuthRequired := errors.New("wrapped: " + ErrAuthRequired.Error())
	assertEqual(t, false, IsAuthError(wrappedAuthRequired))

	// Test with non-auth error
	assertEqual(t, false, IsAuthError(ErrDeviceNotFound))

	// Test with device error containing auth error
	deviceErr := &DeviceError{
		IP:         "192.168.1.100",
		Generation: 2,
		Operation:  "GetInfo",
		Err:        ErrAuthRequired,
	}
	assertTrue(t, IsAuthError(deviceErr))
}

func TestIsNetworkError(t *testing.T) {
	// Test with nil error
	assertEqual(t, false, IsNetworkError(nil))

	// Test with device not found error
	assertTrue(t, IsNetworkError(ErrDeviceNotFound))

	// Test with timeout error
	assertTrue(t, IsNetworkError(ErrTimeout))

	// Test with connection failed error
	assertTrue(t, IsNetworkError(ErrConnectionFailed))

	// Test with non-network error
	assertEqual(t, false, IsNetworkError(ErrAuthRequired))

	// Test with device error containing network error
	deviceErr := &DeviceError{
		IP:         "192.168.1.100",
		Generation: 2,
		Operation:  "GetInfo",
		Err:        ErrTimeout,
	}
	assertTrue(t, IsNetworkError(deviceErr))
}

func TestIsDeviceError(t *testing.T) {
	// Test with nil error
	assertEqual(t, false, IsDeviceError(nil))

	// Test with device error
	deviceErr := &DeviceError{
		IP:         "192.168.1.100",
		Generation: 2,
		Operation:  "GetInfo",
		Message:    "Some error",
	}
	assertTrue(t, IsDeviceError(deviceErr))

	// Test with non-device error
	assertEqual(t, false, IsDeviceError(ErrAuthRequired))

	// Test with wrapped device error
	wrapped := errors.New("wrapped error")
	assertEqual(t, false, IsDeviceError(wrapped))
}

func TestRPCError_Error(t *testing.T) {
	rpcErr := &RPCError{
		Code:    -32601,
		Message: "Method not found",
	}

	expected := "RPC error -32601: Method not found"
	assertEqual(t, expected, rpcErr.Error())
}

func TestRPCError_EmptyMessage(t *testing.T) {
	rpcErr := &RPCError{
		Code:    -32603,
		Message: "",
	}

	expected := "RPC error -32603: "
	assertEqual(t, expected, rpcErr.Error())
}

func TestRPCError_Constants(t *testing.T) {
	// Test that RPC error constants have expected values
	assertEqual(t, -103, RPCErrorInvalidArgument)
	assertEqual(t, -104, RPCErrorDeadlineExceeded)
	assertEqual(t, -105, RPCErrorNotFound)
	assertEqual(t, -108, RPCErrorResourceExhausted)
	assertEqual(t, -109, RPCErrorFailedPrecondition)
	assertEqual(t, -114, RPCErrorUnavailable)
	assertEqual(t, -32603, RPCErrorInternal)
	assertEqual(t, -32601, RPCErrorMethodNotFound)
	assertEqual(t, -32602, RPCErrorInvalidParams)
}

func TestDeviceError_AllFields(t *testing.T) {
	underlying := errors.New("network timeout")
	deviceErr := &DeviceError{
		IP:         "192.168.1.100",
		Generation: 2,
		Operation:  "GetInfo",
		StatusCode: 500,
		Message:    "Internal Server Error",
		Err:        underlying,
	}

	// When Err is present, it takes priority in error message
	expected := "device 192.168.1.100 (gen2) GetInfo failed: network timeout"
	assertEqual(t, expected, deviceErr.Error())

	// But unwrap should still return the underlying error
	assertEqual(t, underlying, deviceErr.Unwrap())
}

func TestDeviceError_MinimalFields(t *testing.T) {
	deviceErr := &DeviceError{
		IP:        "192.168.1.100",
		Operation: "TestConnection",
	}

	// Generation defaults to 0, Message is empty
	expected := "device 192.168.1.100 (gen0) TestConnection failed: "
	assertEqual(t, expected, deviceErr.Error())
}

func TestErrorChaining(t *testing.T) {
	// Test that errors.Is works with device errors
	underlying := ErrAuthRequired
	deviceErr := &DeviceError{
		IP:         "192.168.1.100",
		Generation: 2,
		Operation:  "GetInfo",
		Err:        underlying,
	}

	// Should be able to detect the underlying auth error
	assertTrue(t, errors.Is(deviceErr, ErrAuthRequired))
	assertEqual(t, false, errors.Is(deviceErr, ErrDeviceNotFound))
}

func TestErrorAs(t *testing.T) {
	// Test that errors.As works with device errors
	deviceErr := &DeviceError{
		IP:         "192.168.1.100",
		Generation: 2,
		Operation:  "GetInfo",
		Message:    "Test error",
	}

	var target *DeviceError
	found := errors.As(deviceErr, &target)
	assertTrue(t, found)
	assertEqual(t, deviceErr, target)

	// Test with non-device error
	regularErr := errors.New("regular error")
	found = errors.As(regularErr, &target)
	assertEqual(t, false, found)
}

func TestRPCError_Interface(t *testing.T) {
	// Test that RPCError implements error interface
	var err error = &RPCError{
		Code:    -32601,
		Message: "Method not found",
	}

	assertEqual(t, "RPC error -32601: Method not found", err.Error())
}

func TestDeviceError_Interface(t *testing.T) {
	// Test that DeviceError implements error interface
	var err error = &DeviceError{
		IP:         "192.168.1.100",
		Generation: 2,
		Operation:  "GetInfo",
		Message:    "Test error",
	}

	assertEqual(t, "device 192.168.1.100 (gen2) GetInfo failed: Test error", err.Error())
}
