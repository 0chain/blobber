//go:build js && wasm

package worker

import (
	"context"

	"github.com/hack-pad/safejs"
)

// GlobalSelf represents the global scope, named "self", in the context of using Workers.
// Supports sending and receiving messages via PostMessage() and Listen().
type GlobalSelf struct {
	self safejs.Value
	port *messagePort
}

// Self returns the global "self"
func Self() (*GlobalSelf, error) {
	self, err := safejs.Global().Get("self")
	if err != nil {
		return nil, err
	}
	port, err := wrapMessagePort(self)
	if err != nil {
		return nil, err
	}
	return &GlobalSelf{
		self: self,
		port: port,
	}, nil
}

// PostMessage sends data in a message to the main thread that spawned it,
// optionally transferring ownership of all items in transfers.
//
// The data may be any value handled by the "structured clone algorithm", which includes cyclical references.
//
// Transfers is an optional array of Transferable objects to transfer ownership of.
// If the ownership of an object is transferred, it becomes unusable in the context it was sent from and becomes available only to the worker it was sent to.
// Transferable objects are instances of classes like ArrayBuffer, MessagePort or ImageBitmap objects that can be transferred.
// null is not an acceptable value for transfer.
func (s *GlobalSelf) PostMessage(message safejs.Value, transfers []safejs.Value) error {
	return s.port.PostMessage(message, transfers)
}

// Listen sends message events on a channel for events fired by worker.postMessage() calls inside the main thread's global scope.
// Stops the listener and closes the channel when ctx is canceled.
func (s *GlobalSelf) Listen(ctx context.Context) (<-chan MessageEvent, error) {
	return s.port.Listen(ctx)
}

// Close discards any tasks queued in the global scope's event loop, effectively closing this particular scope.
func (s *GlobalSelf) Close() error {
	_, err := s.self.Call("close")
	return err
}

// Name returns the name that the Worker was (optionally) given when it was created.
func (s *GlobalSelf) Name() (string, error) {
	name, err := s.self.Get("name")
	if err != nil {
		return "", err
	}
	return name.String()
}
