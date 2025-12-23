// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/a-h/templ"
	"github.com/rs/zerolog/log"

	"codeberg.org/pixivfe/pixivfe/v3/assets/components/fragments"
	"codeberg.org/pixivfe/pixivfe/v3/assets/components/partials"
	"codeberg.org/pixivfe/pixivfe/v3/core/cookie"
	"codeberg.org/pixivfe/pixivfe/v3/core/untrusted"
)

var (
	// errMissingKeyForToggle is returned when a toggle action is missing the 'key'
	// form value, which specifies the name of the cookie to toggle.
	errMissingKeyForToggle = errors.New("missing 'key' for toggle_cookie action")

	// errInvalidCookieNameForToggle is returned when a toggle action is attempted
	// on a cookie name not present in the allowlist.
	errInvalidCookieNameForToggle = errors.New("invalid cookie name for toggle")

	// componentActions is a dispatcher map for server-driven component actions.
	componentActions = map[string]componentAction{
		"toggle_cookie": toggleCookieAction,
	}
)

// componentAction defines the signature for a server-driven action that returns
// a new state for a component.
//
// It takes the request and response writer, and returns the list of inputs for
// the next state of the component, or an error if the action fails.
type componentAction func(w http.ResponseWriter, r *http.Request) ([]fragments.ComponentReturnInput, error)

// handleComponentAction dispatches server-driven UI actions based on the
// 'component_return_action' form value.
//
// It finds the appropriate handler in the componentActions map, executes it,
// and renders the component's new state as an HTML fragment.
func handleComponentAction(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)

		return
	}

	// Route the request to the correct action handler.
	action, ok := componentActions[r.FormValue("component_return_action")]
	if !ok {
		http.Error(w, "Missing or unknown component action", http.StatusBadRequest)

		return
	}

	// Execute the action.
	newInputs, err := action(w, r)
	if err != nil {
		// The action handler is responsible for creating a user-friendly error message.
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	// Render the response fragment with the new state.
	renderComponentStateFragment(w, r, newInputs)
}

// renderComponentStateFragment renders an HTML fragment containing the new state of a component.
//
// The state is encoded as a series of hidden input fields within a form, suitable for an htmx response.
func renderComponentStateFragment(w http.ResponseWriter, r *http.Request, inputs []fragments.ComponentReturnInput) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	component := fragments.ComponentReturn(fragments.ComponentReturnProps{
		Inputs: inputs,
	})
	if err := component.Render(r.Context(), w); err != nil {
		// If rendering fails, headers are likely already sent. Log the error.
		log.Err(err).Msg("Error rendering component state fragment")
	}
}

// toggleCookieAction is a componentAction that toggles a boolean-like cookie
// value ("true" or "false").
//
// It reads the cookie name from the 'key' form value, calculates the new value,
// sets the cookie, and returns the inputs required to render the component's next state for htmx.
func toggleCookieAction(w http.ResponseWriter, r *http.Request) ([]fragments.ComponentReturnInput, error) {
	key := r.FormValue("key")
	if key == "" {
		return nil, errMissingKeyForToggle
	}

	// Validate that the key is a legitimate, known cookie name.
	cookieName := cookie.CookieName(key)
	if !slices.Contains(cookie.AllCookieNames, cookieName) {
		return nil, fmt.Errorf("%w: %s", errInvalidCookieNameForToggle, key)
	}

	// The current value is sent from the client via hx-include.
	// This component's state is designed to always send "true" or "false".

	// Toggle the boolean value.
	newValue := "true"
	if r.FormValue(key) == "true" {
		newValue = "false"
	}

	// Perform the state-changing action: set the cookie.
	untrusted.SetCookie(w, r, cookieName, newValue)

	// Return the complete state required for the next htmx request.
	// Ensures that the component remains functional for subsequent interactions.
	return []fragments.ComponentReturnInput{
		{Name: "return_component", Value: "true", Type: "hidden"},
		{Name: "component_return_action", Value: "toggle_cookie", Type: "hidden"},
		{Name: "key", Value: key, Type: "hidden"},
		{Name: key, Value: newValue, Type: "hidden"},
	}, nil
}

// handleAJAXResponse renders a self-dismissing alert component for an AJAX request.
//
// If err is non-nil, it renders a warning-level alert with the error message
// and writes an http.StatusBadRequest status code.
//
// If err is nil, it renders an info-level alert with the provided message and
// writes an http.StatusOK status code.
func handleAJAXResponse(w http.ResponseWriter, r *http.Request, message string, err error) {
	var (
		component  templ.Component
		statusCode int
	)

	if err != nil {
		statusCode = http.StatusBadRequest
		component = partials.Alert(partials.AlertProps{Message: err.Error(), Level: "warning", Dismissible: true})
	} else {
		statusCode = http.StatusOK
		component = partials.Alert(partials.AlertProps{Message: message, Level: "info", Dismissible: true})
	}

	w.WriteHeader(statusCode)

	if renderErr := component.Render(r.Context(), w); renderErr != nil {
		log.Err(renderErr).
			Msg("Error rendering form response component")
	}
}
