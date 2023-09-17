package log

import (
	"bytes"
	"context"
	"time"

	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testTime = time.Date(2023, time.September, 17, 06, 44, 13, 0, time.UTC)

func TestInfoLevelOutput(t *testing.T) {
	assert := assert.New(t)
	var buf bytes.Buffer

	h := New(&buf, nil)
	logger := slog.New(logtimeHandler{testTime, h})
	logger.Log(context.Background(), slog.LevelInfo, "testing jerm custom slog handler")

	actual := buf.String()
	actual = actual[:len(actual)-1]
	expected := "2023-09-17T06:44:13Z INFO testing jerm custom slog handler"
	assert.Equal(expected, actual)
}

func TestDebugLevelOutput(t *testing.T) {
	assert := assert.New(t)
	var buf bytes.Buffer

	logLevel := &slog.LevelVar{}
	logLevel.Set(slog.LevelDebug)
	h := New(&buf, &slog.HandlerOptions{
		Level: logLevel,
	})
	logger := slog.New(logtimeHandler{testTime, h})
	logger.Log(context.Background(), slog.LevelDebug, "testing jerm custom slog handler")

	actual := buf.String()
	actual = actual[:len(actual)-1]
	expected := "2023-09-17T06:44:13Z DEBUG testing jerm custom slog handler"
	assert.Equal(expected, actual)
}

func TestErrorLevelOutput(t *testing.T) {
	assert := assert.New(t)
	var buf bytes.Buffer

	logLevel := &slog.LevelVar{}
	logLevel.Set(slog.LevelError)
	h := New(&buf, &slog.HandlerOptions{
		Level: logLevel,
	})
	logger := slog.New(logtimeHandler{testTime, h})
	logger.Log(context.Background(), slog.LevelError, "testing jerm custom slog handler")

	actual := buf.String()
	actual = actual[:len(actual)-1]
	expected := "2023-09-17T06:44:13Z ERROR testing jerm custom slog handler"
	assert.Equal(expected, actual)
}

func TestWarnLevelOutput(t *testing.T) {
	assert := assert.New(t)
	var buf bytes.Buffer

	logLevel := &slog.LevelVar{}
	logLevel.Set(slog.LevelWarn)
	h := New(&buf, &slog.HandlerOptions{
		Level: logLevel,
	})
	logger := slog.New(logtimeHandler{testTime, h})
	logger.Log(context.Background(), slog.LevelWarn, "testing jerm custom slog handler")

	actual := buf.String()
	actual = actual[:len(actual)-1]
	expected := "2023-09-17T06:44:13Z WARN testing jerm custom slog handler"
	assert.Equal(expected, actual)
}

func TestOutputWithAttr(t *testing.T) {
	assert := assert.New(t)
	var buf bytes.Buffer

	h := New(&buf, nil)
	logger := slog.New(logtimeHandler{testTime, h})
	attr := []slog.Attr{slog.String("t", "test"), slog.Bool("b", true)}
	logger.LogAttrs(context.Background(), slog.LevelInfo, "testing jerm custom slog handler", attr...)

	actual := buf.String()
	actual = actual[:len(actual)-1]
	expected := "2023-09-17T06:44:13Z INFO testing jerm custom slog handler t=test b=true"
	assert.Equal(expected, actual)
}

func TestOutputWithGroup(t *testing.T) {
	assert := assert.New(t)
	var buf bytes.Buffer

	h := New(&buf, nil)
	logger := slog.New(logtimeHandler{testTime, h})
	attr := []slog.Attr{
		slog.String("t", "test"),
		slog.Group("g", slog.Int("a", 1), slog.Int("b", 2)),
		slog.Bool("b", true),
	}
	logger.LogAttrs(context.Background(), slog.LevelInfo, "testing jerm custom slog handler", attr...)

	actual := buf.String()
	actual = actual[:len(actual)-1]
	expected := "2023-09-17T06:44:13Z INFO testing jerm custom slog handler t=test g.a=1 g.b=2 b=true"
	assert.Equal(expected, actual)
}

type logtimeHandler struct {
	t time.Time
	h slog.Handler
}

func (h logtimeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.h.Enabled(ctx, level)
}

func (h logtimeHandler) WithGroup(name string) slog.Handler {
	return logtimeHandler{h.t, h.h.WithGroup(name)}
}

func (h logtimeHandler) Handle(ctx context.Context, r slog.Record) error {
	r.Time = h.t
	return h.h.Handle(ctx, r)
}

func (h logtimeHandler) WithAttrs(as []slog.Attr) slog.Handler {
	return logtimeHandler{h.t, h.h.WithAttrs(as)}
}
