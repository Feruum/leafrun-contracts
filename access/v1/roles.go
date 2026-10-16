package accessv1

import identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"

// BindingRequest assigns exactly one raw role or independent capability to an
// eligible same-organization human or team. Scope comes from the verified route.
type BindingRequest struct {
	SubjectKind string `json:"subjectKind"`
	SubjectID   string `json:"subjectId"`
	Relation    string `json:"relation"`
}

func RawRelation(r string) (string, bool) {
	switch r {
	case "reader", "commenter", "editor", "owner", "generate", "download":
		return r, true
	case "source.read":
		return "source_read", true
	case "resource.read":
		return "resource_read", true
	default:
		return "", false
	}
}
func (b BindingRequest) Validate() error {
	if b.SubjectKind != "human" && b.SubjectKind != "team" && b.SubjectKind != "bot" {
		return ErrInvalid
	}
	if !identityv1.ValidIdentifier(b.SubjectID) {
		return ErrInvalid
	}
	if _, ok := RawRelation(b.Relation); !ok {
		return ErrInvalid
	}
	return nil
}

type Binding struct {
	Scope PermissionScope `json:"scope"`
	BindingRequest
}
