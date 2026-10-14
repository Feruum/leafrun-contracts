package renderwire

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"testing"
)

func readFile(d *Decoder) error {
	return d.Read(File, func(_ Header, r io.Reader) error { _, err := io.Copy(io.Discard, r); return err })
}

func TestCodecRoundTrip(t *testing.T) {
	var wire bytes.Buffer
	e := NewEncoder(&wire, InputLimit)
	if err := e.WriteBytes(Request, []byte(`{"entrypoint":"main.typ"}`)); err != nil {
		t.Fatal(err)
	}
	if err := e.WriteBytes(Terminal, []byte(`{"complete":true}`)); err != nil {
		t.Fatal(err)
	}
	d := NewDecoder(&wire, InputLimit)
	for range 2 {
		if kind, err := d.PeekKind(); err != nil || kind != Request {
			t.Fatal(kind, err)
		}
	}
	got, err := d.ReadBytes(Request)
	if err != nil || string(got) != `{"entrypoint":"main.typ"}` {
		t.Fatal(string(got), err)
	}
	if _, err = d.ReadBytes(Terminal); err != nil {
		t.Fatal(err)
	}
	if err = d.Finish(); err != nil {
		t.Fatal(err)
	}
	if err = e.WriteBytes(File, []byte("late")); err == nil {
		t.Fatal("write after terminal")
	}
}
func TestCodecCorruptionTruncationAndTrailing(t *testing.T) {
	var b bytes.Buffer
	e := NewEncoder(&b, InputLimit)
	_ = e.WriteBytes(File, []byte("abc"))
	_ = e.WriteBytes(Terminal, []byte(`{}`))
	original := bytes.Clone(b.Bytes())
	for _, tc := range []struct {
		name string
		wire []byte
	}{
		{"checksum", append(bytes.Clone(original[:48]), append([]byte("xyz"), original[51:]...)...)},
		{"header", original[:20]}, {"body", original[:50]}, {"missing terminal", original[:51]},
		{"trailing", append(bytes.Clone(original), 'x')},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDecoder(bytes.NewReader(tc.wire), InputLimit)
			err := readFile(d)
			if err == nil {
				_, err = d.ReadBytes(Terminal)
			}
			if err == nil {
				err = d.Finish()
			}
			if err == nil {
				t.Fatal("accepted broken stream")
			}
		})
	}
}
func TestCodecLimitsAndConsumption(t *testing.T) {
	var b bytes.Buffer
	e := NewEncoder(&b, InputLimit)
	_ = e.WriteBytes(File, []byte("abc"))
	if err := readFile(NewDecoder(bytes.NewReader(b.Bytes()), InputLimit)); err != nil {
		t.Fatal("valid binary frame", err)
	}
	d := NewDecoder(bytes.NewReader(b.Bytes()), 50)
	if err := readFile(d); err == nil {
		t.Fatal("stream limit")
	}
	d = NewDecoder(bytes.NewReader(b.Bytes()), InputLimit)
	if err := d.Read(File, func(Header, io.Reader) error { return nil }); err == nil {
		t.Fatal("unconsumed body")
	}
	if err := readFile(d); err == nil {
		t.Fatal("decoder reused after error")
	}
	e = NewEncoder(io.Discard, InputLimit)
	h := sha256.Sum256(nil)
	if err := e.WriteFrom(File, SnapshotLimit+1, h, bytes.NewReader(nil)); err == nil {
		t.Fatal("oversize frame")
	}
	if err := NewEncoder(io.Discard, InputLimit).WriteFrom(File, 3, h, bytes.NewReader([]byte("abc"))); err == nil {
		t.Fatal("wrong source digest")
	}
	if err := NewDecoder(bytes.NewReader(nil), InputLimit).Read(File, func(Header, io.Reader) error { return nil }); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("missing frame: %v", err)
	}
}
func TestCodecRejectsUnknownKindAndMagic(t *testing.T) {
	var b bytes.Buffer
	e := NewEncoder(&b, InputLimit)
	_ = e.WriteBytes(File, nil)
	for _, idx := range []int{0, 4, 5} {
		wire := bytes.Clone(b.Bytes())
		wire[idx] = 255
		d := NewDecoder(bytes.NewReader(wire), InputLimit)
		if err := readFile(d); err == nil {
			t.Fatalf("accepted byte %d", idx)
		}
	}
}

func TestCodecBinaryTransparencyAndStrictMetadata(t *testing.T) {
	payload := []byte{0, 255, 128, 10, 13, 0, 1, 254}
	var b bytes.Buffer
	e := NewEncoder(&b, OutputLimit)
	if err := e.WriteBytes(File, payload); err != nil {
		t.Fatal(err)
	}
	d := NewDecoder(&b, OutputLimit)
	var got []byte
	if err := d.Read(File, func(_ Header, r io.Reader) error { var err error; got, err = io.ReadAll(r); return err }); err != nil || !bytes.Equal(got, payload) {
		t.Fatal(got, err)
	}
	for _, data := range []string{`{"tree":"font","tree":"project","path":"a","size":0,"sha256":"x"}`, `{"Tree":"project","path":"a","size":0,"sha256":"x"}`, `{"tree":"project","path":"\ud800","size":0,"sha256":"x"}`} {
		var f FileDescriptor
		if err := DecodeJSON([]byte(data), &f); err == nil {
			t.Errorf("accepted ambiguous metadata %s", data)
		}
	}
}
