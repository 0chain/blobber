//go:build js && wasm

// Package worker provides a Web Workers driver for Go code compiled to WebAssembly.
package worker

import (
	"context"

	"github.com/hack-pad/safejs"
)

var (
	jsWorker = safejs.MustGetGlobal("Worker")
	jsURL    = safejs.MustGetGlobal("URL")
	jsBlob   = safejs.MustGetGlobal("Blob")
)

// Worker is a Web Worker, which represents a background task created via a script.
// Use Listen() and PostMessage() to communicate with the worker.
type Worker struct {
	worker safejs.Value
	port   *messagePort
}

// Options contains optional configuration for new Workers
type Options struct {
	// Name specifies an identifying name for the DedicatedWorkerGlobalScope representing the scope of the worker, which is mainly useful for debugging purposes.
	Name string
}

func (w Options) toJSValue() (safejs.Value, error) {
	options := make(map[string]any)
	if w.Name != "" {
		options["name"] = w.Name
	}
	return safejs.ValueOf(options)
}

// New starts a worker with the given script's URL and returns it
func New(url string, options Options) (*Worker, error) {
	jsOptions, err := options.toJSValue()
	if err != nil {
		return nil, err
	}
	worker, err := jsWorker.New(url, jsOptions)
	if err != nil {
		return nil, err
	}
	port, err := wrapMessagePort(worker)
	if err != nil {
		return nil, err
	}
	return &Worker{
		port:   port,
		worker: worker,
	}, nil
}

// NewFromScript is like New, but starts the worker with the given script (in JavaScript)
func NewFromScript(jsScript string, options Options) (*Worker, error) {
	blob, err := jsBlob.New([]any{jsScript}, map[string]any{
		"type": "text/javascript",
	})
	if err != nil {
		return nil, err
	}
	objectURL, err := jsURL.Call("createObjectURL", blob)
	if err != nil {
		return nil, err
	}
	objectURLStr, err := objectURL.String()
	if err != nil {
		return nil, err
	}
	return New(objectURLStr, options)
}

// Terminate immediately terminates the Worker.
// This does not offer the worker an opportunity to finish its operations; it is stopped at once.
func (w *Worker) Terminate() error {
	_, err := w.worker.Call("terminate")
	return err
}

// PostMessage sends data in a message to the worker, optionally transferring ownership of all items in transfers.
//
// The data may be any value handled by the "structured clone algorithm", which includes cyclical references.
//
// Transfers is an optional array of Transferable objects to transfer ownership of.
// If the ownership of an object is transferred, it becomes unusable in the context it was sent from and becomes available only to the worker it was sent to.
// Transferable objects are instances of classes like ArrayBuffer, MessagePort or ImageBitmap objects that can be transferred.
// null is not an acceptable value for transfer.
func (w *Worker) PostMessage(data safejs.Value, transfers []safejs.Value) error {
	return w.port.PostMessage(data, transfers)
}

// Listen sends message events on a channel for events fired by self.postMessage() calls inside the Worker's global scope.
// Stops the listener and closes the channel when ctx is canceled.
func (w *Worker) Listen(ctx context.Context) (<-chan MessageEvent, error) {
	return w.port.Listen(ctx)
}
