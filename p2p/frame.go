package p2p

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Frame is a typed length-prefixed message on the wire:
//
//	uint32be length || uint8 type || payload
//
// length counts type + payload bytes (not including the 4-byte length field).
type Frame struct {
	Type    uint8
	Payload []byte
}

// WriteFrame writes one framed message to w.
func WriteFrame(w io.Writer, typ uint8, payload []byte, maxSize int) error {
	if maxSize <= 0 {
		maxSize = DefaultMaxMsgSize
	}
	n := 1 + len(payload)
	if n > maxSize {
		return fmt.Errorf("p2p: message too large: %d > %d", n, maxSize)
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(n))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := w.Write([]byte{typ}); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := w.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

// ReadFrame reads one framed message from r.
func ReadFrame(r io.Reader, maxSize int) (Frame, error) {
	if maxSize <= 0 {
		maxSize = DefaultMaxMsgSize
	}
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return Frame{}, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n == 0 {
		return Frame{}, fmt.Errorf("p2p: empty frame")
	}
	if int(n) > maxSize {
		return Frame{}, fmt.Errorf("p2p: frame length %d exceeds max %d", n, maxSize)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return Frame{}, err
	}
	return Frame{Type: buf[0], Payload: buf[1:]}, nil
}
