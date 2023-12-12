lager
=====

**Note**: This repository should be imported as `code.cloudfoundry.org/lager`.

Lager is a logging library for go.

## Usage

Instantiate a logger with the name of your component.

```go
import (
  "code.cloudfoundry.org/lager/v3"
)

logger := lager.NewLogger("my-app")
```

### Lager and [`log/slog`](https://pkg.go.dev/log/slog)
Lager was written long before Go 1.21 introduced structured logging in the standard library.
There are some wrapper functions for interoperability between Lager and `slog`,
which are only available when using Go 1.21 and higher.

Lager can be used as an [`slog.Handler`](https://pkg.go.dev/log/slog#Handler) using the `NewHandler()` function:

```go
func codeThatAcceptsSlog(l *slog.Logger) { ... }

lagerLogger := lager.NewLogger("my-lager-logger")

codeThatAcceptsSlog(slog.New(lager.NewHandler(lagerLogger)))
```

An `slog.Logger` can be used as a Lager `Sink` using the `NewSlogSink()` function:
```go
var *slog.Logger l = codeThatReturnsSlog()

lagerLogger := lager.NewLogger("my-lager-logger")

lagerLogger.RegisterSink(lager.NewSlogSink(l))
```

### Sinks

Lager can write logs to a variety of destinations. You can specify the destinations
using Lager sinks:

To write to an arbitrary `Writer` object:

```go
logger.RegisterSink(lager.NewWriterSink(myWriter, lager.INFO))
```

### Emitting logs

Lager supports the usual level-based logging, with an optional argument for arbitrary key-value data.

```go
logger.Info("doing-stuff", lager.Data{
  "informative": true,
})
```

output:
```json
{ "source": "my-app", "message": "doing-stuff", "data": { "informative": true }, "timestamp": 1232345, "log_level": 1 }
```

Error messages also take an `Error` object:

```go
logger.Error("failed-to-do-stuff", errors.New("Something went wrong"))
```

output:
```json
{ "source": "my-app", "message": "failed-to-do-stuff", "data": { "error": "Something went wrong" }, "timestamp": 1232345, "log_level": 1 }
```

### Sessions

You can avoid repetition of contextual data using 'Sessions':

```go

contextualLogger := logger.Session("my-task", lager.Data{
  "request-id": 5,
})

contextualLogger.Info("my-action")
```

output:

```json
{ "source": "my-app", "message": "my-task.my-action", "data": { "request-id": 5 }, "timestamp": 1232345, "log_level": 1 }
```

## License

Lager is [Apache 2.0](https://github.com/cloudfoundry/lager/blob/master/LICENSE) licensed.
