// Package auditv1 defines the closed v1 participant/service action payload.
// IDs are syntactically bounded, not semantically proven secret-free. Trusted
// producer provenance and server-owned ID selection remain server responsibilities.
package auditv1

import (
	"encoding/json"
	"errors"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	jsonvalue "github.com/leafrun/leafrun-contracts/jsonvalue/v1"
)

const MaxActionBytes = 8*identityv1.MaxIdentifierBytes + 4096

type Producer string

const (
	ProducerAccess     Producer = "access"
	ProducerProjects   Producer = "projects"
	ProducerGeneration Producer = "generation"
	ProducerStorage    Producer = "storage"
)

type ActionName string

const (
	ActionIdentityResolve     ActionName = "identity.resolve"
	ActionIdentityCreate      ActionName = "identity.create"
	ActionIdentityUpdate      ActionName = "identity.update"
	ActionIdentityRevoke      ActionName = "identity.revoke"
	ActionHierarchyRegister   ActionName = "hierarchy.register"
	ActionOrganizationCreate  ActionName = "organization.create"
	ActionOrganizationUpdate  ActionName = "organization.update"
	ActionOrganizationDelete  ActionName = "organization.delete"
	ActionWorkspaceCreate     ActionName = "workspace.create"
	ActionWorkspaceUpdate     ActionName = "workspace.update"
	ActionWorkspaceDelete     ActionName = "workspace.delete"
	ActionProjectCreate       ActionName = "project.create"
	ActionProjectUpdate       ActionName = "project.update"
	ActionProjectDelete       ActionName = "project.delete"
	ActionMembershipAdd       ActionName = "membership.add"
	ActionMembershipUpdate    ActionName = "membership.update"
	ActionMembershipRemove    ActionName = "membership.remove"
	ActionInvitationCreate    ActionName = "invitation.create"
	ActionInvitationAccept    ActionName = "invitation.accept"
	ActionInvitationRevoke    ActionName = "invitation.revoke"
	ActionRoleAssign          ActionName = "role.assign"
	ActionRoleRevoke          ActionName = "role.revoke"
	ActionBotCreate           ActionName = "bot.create"
	ActionBotUpdate           ActionName = "bot.update"
	ActionBotRevoke           ActionName = "bot.revoke"
	ActionKeyCreate           ActionName = "key.create"
	ActionKeyRotate           ActionName = "key.rotate"
	ActionKeyRevoke           ActionName = "key.revoke"
	ActionGrantCreate         ActionName = "grant.create"
	ActionGrantUpdate         ActionName = "grant.update"
	ActionGrantRevoke         ActionName = "grant.revoke"
	ActionGuestCreate         ActionName = "guest.create"
	ActionGuestExchange       ActionName = "guest.exchange"
	ActionGuestRevoke         ActionName = "guest.revoke"
	ActionConsentGrant        ActionName = "consent.grant"
	ActionConsentRevoke       ActionName = "consent.revoke"
	ActionReleasePublish      ActionName = "release.publish"
	ActionReleaseRetire       ActionName = "release.retire"
	ActionGenerationAdmission ActionName = "generation.admission"
	ActionGenerationTerminal  ActionName = "generation.terminal"
	// Authorization precedes opening a stream; it never asserts transfer completion.
	ActionDownloadAuthorized ActionName = "download.authorized"
)

type ResourceKind string

const (
	ResourceIdentity      ResourceKind = "identity"
	ResourceAuthority     ResourceKind = "authority"
	ResourceOrganization  ResourceKind = "organization"
	ResourceWorkspace     ResourceKind = "workspace"
	ResourceProject       ResourceKind = "project"
	ResourceMembership    ResourceKind = "membership"
	ResourceInvitation    ResourceKind = "invitation"
	ResourceRole          ResourceKind = "role"
	ResourceBot           ResourceKind = "bot"
	ResourceKey           ResourceKind = "key"
	ResourceGrant         ResourceKind = "grant"
	ResourceGuest         ResourceKind = "guest"
	ResourceConsent       ResourceKind = "consent"
	ResourceRelease       ResourceKind = "release"
	ResourceJob           ResourceKind = "job"
	ResourceArtifact      ResourceKind = "artifact"
	ResourceStorageObject ResourceKind = "storage_object"
)

type Outcome string

const (
	OutcomeSucceeded Outcome = "succeeded"
	OutcomeDenied    Outcome = "denied"
	OutcomeFailed    Outcome = "failed"
	OutcomeAdmitted  Outcome = "admitted"
	OutcomeCompleted Outcome = "completed"
	OutcomeCancelled Outcome = "cancelled"
	OutcomeRejected  Outcome = "rejected"
)

type ResourceRef struct {
	Kind ResourceKind `json:"kind"`
	ID   string       `json:"id"`
}
type Action struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Producer      Producer                `json:"producer"`
	Actor         identityv1.Principal    `json:"actor"`
	Authority     identityv1.AuthorityRef `json:"authority"`
	OperationID   string                  `json:"operationId"`
	Action        ActionName              `json:"action"`
	Resource      ResourceRef             `json:"resource"`
	Outcome       Outcome                 `json:"outcome"`
	Revision      string                  `json:"revision,omitempty"`
}

// Subject recognizes exactly the four owner domains. Audit operator actions use
// a separate authenticated internal ledger rather than a fifth event producer.
func Subject(producer Producer) (string, error) {
	switch producer {
	case ProducerAccess, ProducerProjects, ProducerGeneration, ProducerStorage:
		return "leafrun.audit." + string(producer) + ".action.v1", nil
	}
	return "", errors.New("invalid audit producer")
}
func actionProducer(action ActionName) Producer {
	switch action {
	case ActionIdentityResolve, ActionIdentityCreate, ActionIdentityUpdate, ActionIdentityRevoke, ActionHierarchyRegister,
		ActionOrganizationCreate, ActionOrganizationUpdate, ActionOrganizationDelete,
		ActionWorkspaceCreate, ActionWorkspaceUpdate, ActionWorkspaceDelete, ActionProjectCreate, ActionProjectUpdate, ActionProjectDelete,
		ActionMembershipAdd, ActionMembershipUpdate, ActionMembershipRemove, ActionInvitationCreate, ActionInvitationAccept, ActionInvitationRevoke,
		ActionRoleAssign, ActionRoleRevoke, ActionBotCreate, ActionBotUpdate, ActionBotRevoke, ActionKeyCreate, ActionKeyRotate, ActionKeyRevoke,
		ActionGrantCreate, ActionGrantUpdate, ActionGrantRevoke, ActionGuestCreate, ActionGuestExchange, ActionGuestRevoke, ActionConsentGrant, ActionConsentRevoke:
		return ProducerAccess
	case ActionReleasePublish, ActionReleaseRetire:
		return ProducerProjects
	case ActionGenerationAdmission, ActionGenerationTerminal:
		return ProducerGeneration
	case ActionDownloadAuthorized:
		return ProducerStorage
	}
	return ""
}
func (a Action) Validate(subject string) error {
	expected, err := Subject(a.Producer)
	if err != nil || subject != expected {
		return errors.New("invalid audit subject binding")
	}
	if a.SchemaVersion != 1 {
		return errors.New("invalid audit schema version")
	}
	if actionProducer(a.Action) != a.Producer {
		return errors.New("invalid audit action producer")
	}
	if a.Actor.Validate() != nil || a.Authority.Validate() != nil {
		return errors.New("invalid audit actor authority")
	}
	if !identityv1.ValidIdentifier(a.OperationID) || !identityv1.ValidIdentifier(a.Resource.ID) || (a.Revision != "" && !identityv1.ValidIdentifier(a.Revision)) {
		return errors.New("invalid audit ID")
	}
	switch a.Resource.Kind {
	case ResourceIdentity, ResourceAuthority, ResourceOrganization, ResourceWorkspace, ResourceProject, ResourceMembership, ResourceInvitation, ResourceRole, ResourceBot, ResourceKey, ResourceGrant, ResourceGuest, ResourceConsent, ResourceRelease, ResourceJob, ResourceArtifact, ResourceStorageObject:
	default:
		return errors.New("invalid audit resource kind")
	}
	switch a.Outcome {
	case OutcomeSucceeded, OutcomeDenied, OutcomeFailed, OutcomeAdmitted, OutcomeCompleted, OutcomeCancelled, OutcomeRejected:
	default:
		return errors.New("invalid audit outcome")
	}
	return nil
}
func EncodeAction(a Action) ([]byte, error) {
	subject, err := Subject(a.Producer)
	if err != nil {
		return nil, err
	}
	if err = a.Validate(subject); err != nil {
		return nil, err
	}
	data, err := json.Marshal(a)
	if err != nil || len(data) > MaxActionBytes {
		return nil, errors.New("invalid audit encoding")
	}
	return data, nil
}
func DecodeAction(subject string, data []byte) (Action, error) {
	var a Action
	// Never expose decoder errors: even an unknown field name can contain a secret.
	if err := jsonvalue.Unmarshal(data, &a, MaxActionBytes); err != nil {
		return Action{}, errors.New("invalid audit JSON")
	}
	if err := a.Validate(subject); err != nil {
		return Action{}, err
	}
	return a, nil
}
