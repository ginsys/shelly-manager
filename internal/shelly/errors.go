package shelly

import (
	"errors"
	"fmt"
)

var (
	// ErrDeviceNotFound indicates the device could not be reached
	ErrDeviceNotFound = errors.New("device not found or unreachable")

	// ErrAuthRequired indicates authentication is required but not provided
	ErrAuthRequired = errors.New("authentication required")

	// ErrAuthFailed indicates authentication failed
	ErrAuthFailed = errors.New("authentication failed")

	// ErrInvalidGeneration indicates an unsupported device generation
	ErrInvalidGeneration = errors.New("unsupported device generation")

	// ErrOperationNotSupported indicates the operation is not supported by this device
	ErrOperationNotSupported = errors.New("operation not supported by this device")

	// ErrInvalidResponse indicates the device returned an invalid or unexpected response
	ErrInvalidResponse = errors.New("invalid response from device")

	// ErrTimeout indicates the operation timed out
	ErrTimeout = errors.New("operation timed out")

	// ErrConnectionFailed indicates a connection could not be established
	ErrConnectionFailed = errors.New("connection failed")

	// ErrConfigurationInvalid indicates invalid configuration parameters
	ErrConfigurationInvalid = errors.New("invalid configuration")

	// ErrFirmwareUpdateInProgress indicates a firmware update is already in progress
	ErrFirmwareUpdateInProgress = errors.New("firmware update already in progress")

	// ErrNoUpdateAvailable indicates no firmware update is available
	ErrNoUpdateAvailable = errors.New("no firmware update available")

	// ErrDeviceBusy indicates the device is busy processing another request
	ErrDeviceBusy = errors.New("device is busy")

	// ErrOvertemperature indicates the device is overheating
	ErrOvertemperature = errors.New("device overtemperature protection active")

	// ErrPowerLimit indicates power limit has been exceeded
	ErrPowerLimit = errors.New("power limit exceeded")
)

// DeviceError represents an error from a Shelly device
type DeviceError struct {
	IP         string
	Generation int
	Operation  string
	StatusCode int
	Message    string
	Err        error
}

// Error returns the error message
func (e *DeviceError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("device %s (gen%d) %s failed: %v", e.IP, e.Generation, e.Operation, e.Err)
	}
	if e.StatusCode != 0 {
		return fmt.Sprintf("device %s (gen%d) %s failed with status %d: %s",
			e.IP, e.Generation, e.Operation, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("device %s (gen%d) %s failed: %s", e.IP, e.Generation, e.Operation, e.Message)
}

// Unwrap returns the underlying error
func (e *DeviceError) Unwrap() error {
	return e.Err
}

// IsAuthError checks if the error is authentication-related
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrAuthRequired) || errors.Is(err, ErrAuthFailed)
}

// IsNetworkError checks if the error is network-related
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrDeviceNotFound) ||
		errors.Is(err, ErrTimeout) ||
		errors.Is(err, ErrConnectionFailed)
}

// IsDeviceError checks if the error is a device-specific error
func IsDeviceError(err error) bool {
	if err == nil {
		return false
	}
	var deviceErr *DeviceError
	return errors.As(err, &deviceErr)
}

// RPCError represents an RPC error from Gen2+ devices
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error returns the error message
func (e *RPCError) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

// Common RPC error codes for Gen2+ devices
const (
	RPCErrorInvalidArgument    = -103
	RPCErrorDeadlineExceeded   = -104
	RPCErrorNotFound           = -105
	RPCErrorResourceExhausted  = -108
	RPCErrorFailedPrecondition = -109
	RPCErrorUnavailable        = -114
	RPCErrorInternal           = -32603
	RPCErrorMethodNotFound     = -32601
	RPCErrorInvalidParams      = -32602
)
