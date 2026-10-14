// Package renderwire defines the bounded Leafrun renderer stream protocol v1.
// All payloads are length-prefixed and SHA-256 checked. EOF is not a terminal
// record. Receiving a valid stream does not establish container exit status.
package renderwire

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	InputLimit    int64 = 128 << 20
	OutputLimit   int64 = 160 << 20
	SnapshotLimit int64 = 100 << 20
	MetadataLimit int64 = 12 << 20
	PDFLimit      int64 = 25 << 20
	EnvelopeLimit int64 = 1 << 20
	headerSize          = 48
)

type Kind byte

const (
	Request Kind = iota + 1
	Manifest
	FileHeader
	File
	PDFHeader
	PDF
	Terminal
)

type Header struct {
	Kind   Kind
	Size   int64
	SHA256 [32]byte
}

func bound(kind Kind) int64 {
	switch kind {
	case Request, Terminal:
		return EnvelopeLimit
	case Manifest:
		return MetadataLimit
	case FileHeader, PDFHeader:
		return 16 << 10
	case File:
		return SnapshotLimit
	case PDF:
		return PDFLimit
	default:
		return -1
	}
}

type Encoder struct {
	out      io.Writer
	left     int64
	terminal bool
	err      error
}

func NewEncoder(out io.Writer, maxStream int64) *Encoder { return &Encoder{out: out, left: maxStream} }
func (e *Encoder) WriteBytes(kind Kind, data []byte) error {
	return e.WriteFrom(kind, int64(len(data)), sha256.Sum256(data), bytes.NewReader(data))
}

// WriteFrom copies exactly size bytes and verifies the source digest. Callers
// must provide an independently bounded reader; excess source bytes are not sent.
func (e *Encoder) WriteFrom(kind Kind, size int64, hash [32]byte, src io.Reader) (err error) {
	if e.err != nil {
		return e.err
	}
	defer func() {
		if err != nil {
			e.err = err
		}
	}()
	if e.terminal {
		return errors.New("frame after terminal")
	}
	if size < 0 || bound(kind) < 0 || size > bound(kind) || e.left < headerSize || size > e.left-headerSize {
		return errors.New("frame or stream limit exceeded")
	}
	var header [headerSize]byte
	copy(header[:4], "LRF1")
	header[4] = byte(kind)
	binary.BigEndian.PutUint64(header[8:16], uint64(size))
	copy(header[16:], hash[:])
	n, err := e.out.Write(header[:])
	if err != nil {
		return err
	}
	if n != len(header) {
		return io.ErrShortWrite
	}
	digest := sha256.New()
	n64, err := io.CopyN(io.MultiWriter(e.out, digest), src, size)
	if err != nil {
		return err
	}
	if n64 != size {
		return io.ErrUnexpectedEOF
	}
	if !bytes.Equal(digest.Sum(nil), hash[:]) {
		return errors.New("source checksum mismatch")
	}
	e.left -= headerSize + size
	e.terminal = kind == Terminal
	return nil
}

type Decoder struct {
	in       io.Reader
	left     int64
	terminal bool
	err      error
	pending  *Header
}

func NewDecoder(in io.Reader, maxStream int64) *Decoder { return &Decoder{in: in, left: maxStream} }

// Read supplies only the current frame payload to consume. consume must read the
// whole payload; any error poisons the decoder. Headers are checked before any
// payload allocation. Call Finish after the terminal frame to reject extra bytes.
func (d *Decoder) Read(expected Kind, consume func(Header, io.Reader) error) (err error) {
	if d.err != nil {
		return d.err
	}
	defer func() {
		if err != nil {
			d.err = err
		}
	}()
	if d.terminal {
		return errors.New("read after terminal")
	}
	if _, err = d.PeekKind(); err != nil {
		return err
	}
	h := *d.pending
	d.pending = nil
	if h.Kind != expected {
		return errors.New("unexpected frame order")
	}
	digest := sha256.New()
	limited := &io.LimitedReader{R: d.in, N: h.Size}
	if err = consume(h, io.TeeReader(limited, digest)); err != nil {
		return err
	}
	if limited.N != 0 {
		return fmt.Errorf("frame payload incomplete: %d bytes remaining", limited.N)
	}
	if !bytes.Equal(h.SHA256[:], digest.Sum(nil)) {
		return errors.New("frame checksum mismatch")
	}
	d.left -= headerSize + h.Size
	d.terminal = h.Kind == Terminal
	return nil
}

// PeekKind inspects and bounds a frame header without consuming its payload.
// This permits a failure Terminal in place of the successful bundle/PDF sequence.
func (d *Decoder) PeekKind() (kind Kind, err error) {
	if d.err != nil {
		return 0, d.err
	}
	defer func() {
		if err != nil {
			d.err = err
		}
	}()
	if d.terminal {
		return 0, errors.New("frame after terminal")
	}
	if d.pending != nil {
		return d.pending.Kind, nil
	}
	var raw [headerSize]byte
	if _, err = io.ReadFull(d.in, raw[:]); err != nil {
		if errors.Is(err, io.EOF) {
			err = io.ErrUnexpectedEOF
		}
		return 0, err
	}
	kind = Kind(raw[4])
	size := binary.BigEndian.Uint64(raw[8:16])
	if string(raw[:4]) != "LRF1" || raw[5] != 0 || raw[6] != 0 || raw[7] != 0 || bound(kind) < 0 {
		return 0, errors.New("invalid frame header")
	}
	if size > uint64(bound(kind)) || d.left < headerSize || size > uint64(d.left-headerSize) {
		return 0, errors.New("frame or stream limit exceeded")
	}
	h := Header{Kind: kind, Size: int64(size)}
	copy(h.SHA256[:], raw[16:])
	d.pending = &h
	return kind, nil
}

// ReadBytes is for bounded metadata. Binary file/PDF frames must use streaming
// Read to prevent large untrusted allocations in a controller.
func (d *Decoder) ReadBytes(kind Kind) (out []byte, err error) {
	if kind == File || kind == PDF {
		return nil, errors.New("binary frames require streaming Read")
	}
	err = d.Read(kind, func(h Header, r io.Reader) error { var e error; out, e = io.ReadAll(r); return e })
	return out, err
}
func (d *Decoder) Finish() error {
	if d.err != nil {
		return d.err
	}
	if !d.terminal {
		return errors.New("missing terminal frame")
	}
	var b [1]byte
	n, err := io.ReadFull(d.in, b[:])
	if n != 0 || err != io.EOF {
		d.err = errors.New("trailing stream data or missing EOF")
		return d.err
	}
	return nil
}
