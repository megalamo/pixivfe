// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

// UnauthorizedError is a rich error type used to signal that a user must be
// authenticated to proceed. It contains the necessary return paths for the
// login flow.
//
// The error handling middleware is expected to catch this error, set the HTTP
// status to 401 Unauthorized, and render the appropriate login prompt page.
type UnauthorizedError struct {
	// NoAuthReturnPath is the return path if the user exits the login flow at any stage.
	NoAuthReturnPath string
	// LoginReturnPath is the return path if the user completes the login flow.
	LoginReturnPath string
}

// Error implements the error interface. The message is simple, as the primary
// purpose of this type is to carry structured data to the error handler.
func (e *UnauthorizedError) Error() string {
	return "unauthorized"
}

// NewUnauthorizedError creates an ErrUnauthorized error.
//
// Route handlers should return this error if a user lacks authentication for an
// action that requires a personal pixiv account. The error handling middleware
// will then render the login page.
func NewUnauthorizedError(noAuthReturnPath, loginReturnPath string) error {
	return &UnauthorizedError{
		NoAuthReturnPath: noAuthReturnPath,
		LoginReturnPath:  loginReturnPath,
	}
}
