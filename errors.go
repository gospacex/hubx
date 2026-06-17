package registry

import "errors"

var (
	ErrProviderNotFound = errors.New("provider not found")
	ErrInstanceNotFound = errors.New("instance not found")
	ErrConfigInvalid    = errors.New("invalid config")
	ErrBuildFailed      = errors.New("build failed")
	ErrRegistryClosed   = errors.New("registry is closed")

	// ErrShutdownFailed indicates that one or more shutdown hooks or
	// provider Close() calls returned a non-nil error. The aggregate
	// error returned by Shutdown wraps ErrShutdownFailed so callers can
	// detect shutdown failures via errors.Is regardless of how many
	// individual failures were joined.
	ErrShutdownFailed = errors.New("hubx: shutdown failed")
)
