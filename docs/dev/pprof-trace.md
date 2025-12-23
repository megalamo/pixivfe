# See flamegraph at runtime

1. Run pixivfe in dev mode
2. Do some activities
3. `curl http://localhost:8282/debug/flight --output trace-file`
4. `go tool trace trace-file`

HTTP requests/responses are shown as named Tasks.

`/debug/flight` returns past data. Use `/debug/pprof/trace?seconds=N` to return future data.
