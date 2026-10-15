package renderwire

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	projectsv1 "github.com/leafrun/leafrun-contracts/projects/v1"
	"io"
	"testing"
)

func inputFilesFixture() ([]projectsv1.PreparedFile, []byte) {
	body := []byte(`<svg/>`)
	digest := sha256.Sum256(body)
	hash := hex.EncodeToString(digest[:])
	return []projectsv1.PreparedFile{{Pointer: "/logo", ResourceID: "resource", Filename: "logo.svg", SHA256: hash, SizeBytes: int64(len(body)), Path: ".leafrun-input/" + hash + ".svg"}}, body
}
func TestInputFileFramesRoundTripAndDeduplicatePaths(t *testing.T) {
	files, body := inputFilesFixture()
	second := files[0]
	second.Pointer = "/other"
	files = append(files, second)
	var stream bytes.Buffer
	encoder := NewEncoder(&stream, InputLimit)
	opens := 0
	if err := WriteInputFiles(encoder, files, func(projectsv1.PreparedFile) (io.ReadCloser, error) {
		opens++
		return io.NopCloser(bytes.NewReader(body)), nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := encoder.WriteBytes(Terminal, []byte(`{"complete":true}`)); err != nil {
		t.Fatal(err)
	}
	decoder := NewDecoder(bytes.NewReader(stream.Bytes()), InputLimit)
	reads := 0
	if err := ReadInputFiles(decoder, files, func(file projectsv1.PreparedFile, r io.Reader) error {
		reads++
		got, err := io.ReadAll(r)
		if !bytes.Equal(got, body) {
			t.Errorf("changed input bytes: %s", got)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if opens != 1 || reads != 1 {
		t.Fatalf("duplicate path streamed: opens=%d reads=%d", opens, reads)
	}
	if _, err := decoder.ReadBytes(Terminal); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Finish(); err != nil {
		t.Fatal(err)
	}
}
func TestInputFileFramesRejectChangedBindingAndBytes(t *testing.T) {
	files, body := inputFilesFixture()
	var stream bytes.Buffer
	if err := WriteInputFiles(NewEncoder(&stream, InputLimit), files, func(projectsv1.PreparedFile) (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }); err != nil {
		t.Fatal(err)
	}
	changed := append([]projectsv1.PreparedFile(nil), files...)
	changed[0].ResourceID = "foreign"
	if err := ReadInputFiles(NewDecoder(bytes.NewReader(stream.Bytes()), InputLimit), changed, func(_ projectsv1.PreparedFile, r io.Reader) error { _, err := io.Copy(io.Discard, r); return err }); err == nil {
		t.Fatal("changed input binding accepted")
	}
	stream.Reset()
	if err := WriteInputFiles(NewEncoder(&stream, InputLimit), files, func(projectsv1.PreparedFile) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bytes.Repeat([]byte{'x'}, len(body)))), nil
	}); err == nil {
		t.Fatal("wrong input digest accepted")
	}
}

func TestInputFilePlanRejectsBudgetsAndPathEscapes(t *testing.T) {
	files, _ := inputFilesFixture()
	for _, mutate := range []func([]projectsv1.PreparedFile) []projectsv1.PreparedFile{
		func(x []projectsv1.PreparedFile) []projectsv1.PreparedFile {
			x[0].Path = ".leafrun-input/../source.typ"
			return x
		},
		func(x []projectsv1.PreparedFile) []projectsv1.PreparedFile { x[0].Path += ".png"; return x },
		func(x []projectsv1.PreparedFile) []projectsv1.PreparedFile { x[0].SizeBytes = 25<<20 + 1; return x },
		func(x []projectsv1.PreparedFile) []projectsv1.PreparedFile { return append(x, x[0]) },
		func(x []projectsv1.PreparedFile) []projectsv1.PreparedFile {
			x[0].SizeBytes = 25 << 20
			other := x[0]
			other.Pointer = "/other"
			other.Filename = "logo.png"
			other.Path = other.Path[:len(other.Path)-4] + ".png"
			return append(x, other)
		},
	} {
		input := mutate(append([]projectsv1.PreparedFile(nil), files...))
		if _, err := PlanInputFiles(input); err == nil {
			t.Fatalf("invalid plan accepted: %+v", input)
		}
	}
}

func TestInputFileDataRequiresStreamingRead(t *testing.T) {
	var stream bytes.Buffer
	if err := NewEncoder(&stream, InputLimit).WriteBytes(InputFile, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDecoder(bytes.NewReader(stream.Bytes()), InputLimit).ReadBytes(InputFile); err == nil {
		t.Fatal("input file allocated through metadata API")
	}
}

func TestAggregateMetadataBudgetIncludesInputDescriptors(t *testing.T) {
	var stream bytes.Buffer
	encoder := NewEncoder(&stream, InputLimit)
	if err := encoder.WriteBytes(Manifest, bytes.Repeat([]byte{'x'}, int(MetadataLimit))); err != nil {
		t.Fatal(err)
	}
	if err := encoder.WriteBytes(InputFileHeader, []byte(`{}`)); err == nil {
		t.Fatal("encoder accepted input metadata beyond aggregate ceiling")
	}
	var extra bytes.Buffer
	if err := NewEncoder(&extra, InputLimit).WriteBytes(InputFileHeader, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	// The peer can construct each frame separately; receiving must enforce the
	// same aggregate budget before reading the second payload.
	decoder := NewDecoder(io.MultiReader(bytes.NewReader(stream.Bytes()), bytes.NewReader(extra.Bytes())), InputLimit)
	if _, err := decoder.ReadBytes(Manifest); err != nil {
		t.Fatal(err)
	}
	if _, err := decoder.ReadBytes(InputFileHeader); err == nil {
		t.Fatal("decoder accepted aggregate metadata overflow")
	}
}
