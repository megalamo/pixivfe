package routes

import (
	"net/http"

	"codeberg.org/pixivfe/pixivfe/v3/assets/views"
	"codeberg.org/pixivfe/pixivfe/v3/server/request_context"
)

// ErrorPage renders an error page.
func ErrorPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	pageData := views.ErrorData{
		Title:      "Error",
		Error:      request_context.FromRequest(r).RequestError,
		StatusCode: request_context.FromRequest(r).StatusCode,
	}

	views.Error(pageData).Render(r.Context(), w)
}
