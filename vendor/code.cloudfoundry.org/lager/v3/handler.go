//go:build go1.21

package lager

import (
	"context"
	"fmt"
	"log/slog"
)

// NewHandler wraps the logger as a slog.Handler
// The supplied Logger must be a lager.logger
// type created by lager.NewLogger(), otherwise
// it panics.
//
// Note the following log level conversions:
//
//	slog.LevelDebug -> lager.DEBUG
//	slog.LevelError -> lager.ERROR
//	slog.LevelError -> lager.FATAL
//	default         -> lager.INFO
func NewHandler(l Logger) slog.Handler {
	switch ll := l.(type) {
	case *logger:
		return &handler{logger: ll}
	default:
		panic("lager.Logger must be an instance of lager.logger")
	}
}

// Type decorator is used to decorate the attributes with groups and more attributes
type decorator func(map[string]any) map[string]any

// Type handler is a slog.Handler that wraps a lager logger.
// It uses the logger concrete type rather than the Logger interface
// because it uses methods not available on the interface.
type handler struct {
	logger     *logger
	decorators []decorator
}

// Enabled always returns true
func (h *handler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// Handle converts a slog.Record into a lager.LogFormat and passes it to every Sink
func (h *handler) Handle(_ context.Context, r slog.Record) error {
	log := LogFormat{
		time:      r.Time,
		Timestamp: formatTimestamp(r.Time),
		Source:    h.logger.component,
		Message:   fmt.Sprintf("%s.%s", h.logger.task, r.Message),
		LogLevel:  toLogLevel(r.Level),
		Data:      h.logger.baseData(h.decorate(attrFromRecord(r))),
	}

	for _, sink := range h.logger.sinks {
		sink.Log(log)
	}

	return nil
}

// WithAttrs returns a new slog.Handler which always adds the specified attributes
func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &handler{
		logger:     h.logger,
		decorators: append(h.decorators, attrDecorator(attrs)),
	}
}

// WithGroup returns a new slog.Handler which always logs attributes in the specified group
func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{
		logger:     h.logger,
		decorators: append(h.decorators, groupDecorator(name)),
	}
}

// decorate will decorate a body using the decorators that have been defined
func (h *handler) decorate(body map[string]any) map[string]any {
	for i := len(h.decorators) - 1; i >= 0; i-- { // reverse iteration
		body = h.decorators[i](body)
	}
	return body
}

// attrDecorator returns a decorator for the specified attributes
func attrDecorator(attrs []slog.Attr) decorator {
	return func(body map[string]any) map[string]any {
		if body == nil {
			body = make(map[string]any)
		}
		processAttrs(attrs, body)
		return body
	}
}

// groupDecorator returns a decorator for the specified group name
func groupDecorator(group string) decorator {
	return func(body map[string]any) map[string]any {
		switch len(body) {
		case 0:
			return nil
		default:
			return map[string]any{group: body}
		}
	}
}

// attrFromRecord extracts and processes the attributes from a record
func attrFromRecord(r slog.Record) map[string]any {
	if r.NumAttrs() == 0 {
		return nil
	}

	body := make(map[string]any, r.NumAttrs())
	r.Attrs(func(attr slog.Attr) bool {
		processAttr(attr, body)
		return true
	})

	return body
}

// processAttrs calls processAttr() for each attribute
func processAttrs(attrs []slog.Attr, target map[string]any) {
	for _, attr := range attrs {
		processAttr(attr, target)
	}
}

// processAttr adds the attribute to the target with appropriate transformations
func processAttr(attr slog.Attr, target map[string]any) {
	rv := attr.Value.Resolve()

	switch {
	case rv.Kind() == slog.KindGroup && attr.Key != "":
		nt := make(map[string]any)
		processAttrs(attr.Value.Group(), nt)
		target[attr.Key] = nt
	case rv.Kind() == slog.KindGroup && attr.Key == "":
		processAttrs(attr.Value.Group(), target)
	case attr.Key == "":
		// skip
	default:
		if rvAsError, isError := rv.Any().(error); isError {
			target[attr.Key] = rvAsError.Error()
		} else {
			target[attr.Key] = rv.Any()
		}
	}
}

// toLogLevel converts from slog levels to lager levels
func toLogLevel(l slog.Level) LogLevel {
	switch l {
	case slog.LevelDebug:
		return DEBUG
	case slog.LevelError, slog.LevelWarn:
		return ERROR
	default:
		return INFO
	}
}
