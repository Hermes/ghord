package ghord

import (
	"encoding/gob"
	"encoding/json"
	"errors"
	"io"
)

var (
	encoders    map[string]*encoderPlugin // List of registered encoders
	currEncoder string                    // Current encoder in use
)

// Function types for creating a new encoder/decoder
// used to register new encoder/decoders
type newEncoderFunc func(io.Writer) *Encoder
type newDecoderFunc func(io.Reader) *Decoder

// Standard encoder interface, usually created from an io.Writer
type Encoder interface {
	// Encode given interface, or error
	Encode(v interface{}) error
}

// Standatd decoder interface, usually created from an io.Reader
type Decoder interface {
	// Decode into the given interface, or error
	Decode(v interface{}) error
}

type encoderPlugin struct {
	// Encoder name (ie json, gob, etc...)
	name string

	// Function used to create a new encoder
	newEncFn newEncoderFunc
	newDecFn newDecoderFunc
}

// Register an encoder to use for communication between nodes
func RegisterEncoder(name string, newEncFn newEncoderFunc, newDecFn newDecoderFunc) {
	plugin := &encoderPlugin{
		name:  name,
		encFn: newEncFn,
		decFn, newDecFn,
	}
	encoders[name] = plugin
}

func UseEncoder(name string) error {
	if _, exists := encoders[name]; !exists {
		return errors.New("Unknown encoder: " + name)
	}
	currEncoder = name
	return nil
}

// Create an encoder from the io.Writer using the
// desired encoder via the UseEncoder() function
func NewEncoder(w io.Writer) *Encoder {
	encoder := encoders[currEncoder]
	return encoder.newEncFn(w)
}

// Create a decoder from the io.Reader using the
// selected decoder via the UseEncoder() function
func NewDecoder(r io.Reader) *Decoder {
	encoder := encoders[currEncoder]
	return encoder.newDecFn(r)
}

// register json and gob encoders
func init() {
	RegisterEncoder("json", json.NewEncoder, json.NewDecoder)
	RegisterEncoder("gob", gob.NewEncoder, gob.NewDecoder)
	UseEncoder("json") // encoding defaults to gob
}
