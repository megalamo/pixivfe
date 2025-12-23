// Copyright 2023 - 2025, VnPower and the PixivFE contributors
// SPDX-License-Identifier: AGPL-3.0-only

package audit

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"runtime/trace"
	"strconv"
	"time"

	servertiming "github.com/mitchellh/go-server-timing"
	"github.com/rs/zerolog/log"
)

// Span represents an HTTP request in flight.
type Span struct {
	// only these fields are set automatically
	task     *trace.Task
	start    time.Time
	duration time.Duration
	metric   *servertiming.Metric

	Destination TrafficDestination
	RequestID   string
	Method      string
	URL         string
	StatusCode  int
	Error       error
	Body        []byte // Body is not logged as is; only for response saving

	responseFilename string // responseFilename logs the filename of a saved response
}

// TrafficDestination describes the logical destination of an HTTP request.
type TrafficDestination string

// Constants for traffic destinations.
const (
	ToUser      TrafficDestination = "user"
	ToPixiv     TrafficDestination = "pixiv"
	ToTurnstile TrafficDestination = "turnstile"

	responseFilePermissions = 0o600
)

var (
	// SaveResponses indicates whether to save response bodies to storage.
	SaveResponses bool

	// ResponseDirectory is the directory where response bodies are saved.
	ResponseDirectory string
)

func (span Span) ServerTimingName() string {
	// must obey naming in docs/dev/server-timing.md
	// base64 without trailing '=' match the syntax
	return string(span.Destination) + "$" + span.Method + "$" + base64.RawURLEncoding.EncodeToString([]byte(span.URL))
}

func (span *Span) Begin(ctx context.Context) context.Context {
	span.start = time.Now()

	ctx, span.task = trace.NewTask(ctx, "http."+string(span.Destination))
	if servertimingContext := servertiming.FromContext(ctx); servertimingContext != nil {
		span.metric = servertimingContext.NewMetric(span.ServerTimingName())
		span.metric.Extra = make(map[string]string)
		span.metric.Extra["start"] = strconv.FormatFloat(float64(span.start.UnixNano())/float64(time.Millisecond), 'f', -1, 64)
	}

	return ctx
}

// LogSpan logs the span and saves it to a file if applicable.
func (span *Span) End() {
	// only log once
	if span.task != nil {
		span.duration = time.Since(span.start)
		span.task.End()

		if span.metric != nil {
			span.metric.Duration = span.duration
		}

		span.task = nil
	}
}

func (span Span) Log() {
	// Handle saving response body
	if span.Destination == ToPixiv && len(span.Body) > 0 && SaveResponses {
		filename := path.Join(ResponseDirectory, span.RequestID)

		if err := os.WriteFile(filename, span.Body, responseFilePermissions); err != nil {
			log.Err(err).
				Str("request_id", span.RequestID).
				Msg("Failed to save response")
		} else {
			span.responseFilename = filename
		}
	}

	event := log.Debug()

	event.Str("sys", "http")
	event.Str("method", span.Method)
	event.Str("url", span.URL)
	event.Int("status_code", span.StatusCode)
	event.Str("len", humanizeSize(len(span.Body)))
	event.Dur("dur", span.duration)
	event.Str("destination", string(span.Destination))
	event.Str("request_id", span.RequestID)

	if span.responseFilename != "" {
		event.Str("response_filename", span.responseFilename)
	}

	if span.Error != nil {
		event.Err(span.Error)
	}

	event.Send()
}

const (
	bytesInKB = 1024
	bytesInMB = bytesInKB * bytesInKB
	bytesInGB = bytesInMB * bytesInKB
)

func humanizeSize(x int) string {
	if x < bytesInKB {
		return strconv.Itoa(x)
	}

	if x < bytesInMB {
		return fmt.Sprintf("%.2fK", float64(x)/bytesInKB)
	}

	if x < bytesInGB {
		return fmt.Sprintf("%.2fM", float64(x)/bytesInMB)
	}

	return fmt.Sprintf("%.2fG", float64(x)/bytesInGB)
}
