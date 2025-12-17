package utils

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type Timing struct {
	Name     	string
	Duration 	time.Duration
	Description string
}

type Timings struct {
	mu      sync.Mutex
	entries []Timing
}

func AddServerTimingHeader() {
	w.Header().Add("Server-Timing", fmt.Sprintf(
		"%s;dur=%.2f;desc=\"%s\"",
		name,
		strconv.FormatFloat(float64(duration.Nanoseconds())/float64(time.Millisecond), 'f', -1, 64),
		description,
	))
}