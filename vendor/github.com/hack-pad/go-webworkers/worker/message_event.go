//go:build js && wasm

package worker

import (
	"github.com/hack-pad/safejs"
	"github.com/pkg/errors"
)

// MessageEvent is received from the channel returned by Listen().
// Represents a JS MessageEvent.
type MessageEvent struct {
	data   safejs.Value
	err    error
	target *messagePort
}

// Data returns this event's data or a parse error
func (e MessageEvent) Data() (safejs.Value, error) {
	return e.data, errors.Wrapf(e.err, "failed to parse MessageEvent %+v", e.data)
}

func parseMessageEvent(v safejs.Value) MessageEvent {
	value, err := v.Get("target")
	if err != nil {
		return MessageEvent{err: err}
	}
	target, err := wrapMessagePort(value)
	if err != nil {
		return MessageEvent{err: err}
	}
	data, err := v.Get("data")
	if err != nil {
		return MessageEvent{err: err}
	}
	return MessageEvent{
		data:   data,
		target: target,
	}
}
