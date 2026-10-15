package renderwire

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"

	generationv1 "github.com/leafrun/leafrun-contracts/generation/v1"
	projectsv1 "github.com/leafrun/leafrun-contracts/projects/v1"
)

// PlanInputFiles bounds descriptors and selects one deterministic stream for
// each generated path. Multiple field references can share identical bytes.
func PlanInputFiles(files []projectsv1.PreparedFile) ([]projectsv1.PreparedFile, error) {
	if len(files) > 64 {
		return nil, errors.New("input file reference limit exceeded")
	}
	ordered := append([]projectsv1.PreparedFile(nil), files...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Path != ordered[j].Path {
			return ordered[i].Path < ordered[j].Path
		}
		return ordered[i].Pointer < ordered[j].Pointer
	})
	paths := map[string]projectsv1.PreparedFile{}
	pointers := map[string]bool{}
	plan := []projectsv1.PreparedFile{}
	var size int64
	for _, file := range ordered {
		hash, err := hex.DecodeString(file.SHA256)
		if err != nil || len(hash) != 32 || strings.ToLower(file.SHA256) != file.SHA256 || !generationv1.ValidFieldPointer(file.Pointer) || pointers[file.Pointer] || !projectsv1.ValidIdentifier(file.ResourceID) || !generationv1.SafeFilename(file.Filename) || file.Path != generationv1.GeneratedInputPath(file.SHA256, file.Filename) || file.SizeBytes < 0 || file.SizeBytes > 25<<20 {
			return nil, errors.New("invalid input file descriptor")
		}
		pointers[file.Pointer] = true
		metadata, _ := json.Marshal(file)
		if int64(len(metadata)) > bound(InputFileHeader) {
			return nil, errors.New("input file descriptor exceeds limit")
		}
		if previous, ok := paths[file.Path]; ok {
			if previous.SizeBytes != file.SizeBytes || previous.SHA256 != file.SHA256 {
				return nil, errors.New("conflicting generated input file")
			}
			continue
		}
		paths[file.Path] = file
		size += file.SizeBytes
		if size > 25<<20 {
			return nil, errors.New("unique input file bytes exceed limit")
		}
		plan = append(plan, file)
	}
	return plan, nil
}

func WriteInputFiles(encoder *Encoder, files []projectsv1.PreparedFile, open func(projectsv1.PreparedFile) (io.ReadCloser, error)) error {
	plan, err := PlanInputFiles(files)
	if err != nil {
		return err
	}
	if len(plan) > 0 && open == nil {
		return errors.New("input file opener required")
	}
	for _, file := range plan {
		header, _ := json.Marshal(file)
		if err = encoder.WriteBytes(InputFileHeader, header); err != nil {
			return err
		}
		body, openErr := open(file)
		if openErr != nil {
			return openErr
		}
		if body == nil {
			return errors.New("nil input file body")
		}
		hash, _ := hex.DecodeString(file.SHA256)
		var digest [32]byte
		copy(digest[:], hash)
		err = encoder.WriteFrom(InputFile, file.SizeBytes, digest, body)
		if err == nil {
			var extra [1]byte
			n, readErr := io.ReadFull(body, extra[:])
			if n != 0 || readErr != io.EOF {
				err = errors.New("input file size did not match descriptor")
			}
		}
		err = errors.Join(err, body.Close())
		if err != nil {
			return err
		}
	}
	return nil
}

func ReadInputFiles(decoder *Decoder, files []projectsv1.PreparedFile, consume func(projectsv1.PreparedFile, io.Reader) error) error {
	plan, err := PlanInputFiles(files)
	if err != nil {
		return err
	}
	if len(plan) > 0 && consume == nil {
		return errors.New("input file consumer required")
	}
	for _, file := range plan {
		data, err := decoder.ReadBytes(InputFileHeader)
		if err != nil {
			return err
		}
		var received projectsv1.PreparedFile
		if err = DecodeJSON(data, &received); err != nil {
			return err
		}
		if received != file {
			return errors.New("input file header binding mismatch")
		}
		hash, _ := hex.DecodeString(file.SHA256)
		var digest [32]byte
		copy(digest[:], hash)
		if err = decoder.Read(InputFile, func(header Header, reader io.Reader) error {
			if header.Size != file.SizeBytes || header.SHA256 != digest {
				return errors.New("input file frame identity mismatch")
			}
			return consume(file, reader)
		}); err != nil {
			return err
		}
	}
	return nil
}
