The `Server-Timing` header is used to include requests sent by the server.

PixivFE users should be able to report this to us.

## Metric name syntax

metric name must be `token`

[definition of metric name](https://www.w3.org/TR/server-timing/#the-server-timing-header-field)

[definition of `token`](https://httpwg.org/specs/rfc7230.html#rule.token.separators)

```
  token          = 1*tchar

  tchar          = "!" / "#" / "$" / "%" / "&" / "'" / "*"
                 / "+" / "-" / "." / "^" / "_" / "`" / "|" / "~" 
                 / DIGIT / ALPHA
                 ; any VCHAR, except delimiters
```

Note: no `/:=`

## Why HTTP Server-Timing has no start time

From https://www.w3.org/TR/server-timing/:

> Because there can be no guarantee of clock synchronization between client, server, and intermediaries, it is impossible to map a meaningful startTime onto the clients timeline. For that reason, any startTime attribution is purposely omitted from this specification. If the developers want to establish a relationship between multiple entries, they can do so by communicating custom data via metric names and/or descriptions.
