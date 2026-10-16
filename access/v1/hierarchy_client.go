package accessv1

import (
	"bytes"
	"context"
	"encoding/json"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// HierarchyResponseLimit accommodates 100 maximum-width legacy project records:
// four 256-byte identifiers and a 128-byte name, with worst-case six-byte JSON
// escaping, framing and a bounded cursor. Authentication keeps its 64KiB cap.
const HierarchyResponseLimit int64 = 1 << 20

func (c *SessionClient) publicCall(ctx context.Context, auth ResolveRequest, method, path string, in, out any) error {
	if auth.Validate() != nil {
		return ErrUnauthenticated
	}
	if auth.SessionCookie != "" {
		if method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions && c.origin == "" {
			return ErrInvalid
		}
		for i := range auth.SessionCookie {
			b := auth.SessionCookie[i]
			if b < 0x21 || b > 0x7e || strings.ContainsRune("\"\\;,", rune(b)) {
				return ErrUnauthenticated
			}
		}
	}
	ctx, cancel := context.WithTimeout(ctx, c.sender.timeout)
	defer cancel()
	var data []byte
	var e error
	if in != nil {
		data, e = json.Marshal(in)
		if e != nil {
			return ErrInvalid
		}
	}
	req, e := http.NewRequestWithContext(ctx, method, c.sender.baseURL+path, bytes.NewReader(data))
	if e != nil {
		return ErrInvalid
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth.Bearer != "" {
		req.Header.Set("Authorization", "Bearer "+auth.Bearer)
	} else {
		name, e := c.selectedCookieName(auth)
		if e != nil {
			return e
		}
		req.Header.Set("Cookie", name+"="+auth.SessionCookie)
		if method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions {
			req.Header.Set("Origin", c.origin)
		}
	}
	return c.sender.sendBounded(req, out, HierarchyResponseLimit)
}
func hierarchySegment(id string) string {
	// ServeMux canonicalizes literal dot segments before routing. Escaping these
	// exact IDs preserves their text; encoded slashes and query bytes stay data.
	if id == "." {
		return "%2E"
	}
	if id == ".." {
		return "%2E%2E"
	}
	return url.PathEscape(id)
}
func validPath(ids ...string) bool {
	for _, id := range ids {
		if !identityv1.ValidIdentifier(id) {
			return false
		}
	}
	return true
}
func pageQuery(p PageRequest) (string, error) {
	if p.Validate() != nil {
		return "", ErrInvalid
	}
	v := url.Values{"limit": {strconv.Itoa(p.Limit)}}
	if p.Cursor != "" {
		v.Set("cursor", p.Cursor)
	}
	return "?" + v.Encode(), nil
}
func (c *SessionClient) CreateOrganization(ctx context.Context, a ResolveRequest, in NameRequest) (Organization, error) {
	var out Organization
	if in.Validate() != nil {
		return out, ErrInvalid
	}
	e := c.publicCall(ctx, a, "POST", "/v1/organizations", in, &out)
	return checked(out, e, out.Validate())
}
func (c *SessionClient) CreateWorkspace(ctx context.Context, a ResolveRequest, org string, in NameRequest) (Workspace, error) {
	var out Workspace
	if !validPath(org) || in.Validate() != nil {
		return out, ErrInvalid
	}
	e := c.publicCall(ctx, a, "POST", "/v1/organizations/"+hierarchySegment(org)+"/workspaces", in, &out)
	if e == nil && out.OrganizationID != org {
		e = ErrDependency
	}
	return checked(out, e, out.Validate())
}
func (c *SessionClient) CreateTeam(ctx context.Context, a ResolveRequest, org string, in NameRequest) (Team, error) {
	var out Team
	if !validPath(org) || in.Validate() != nil {
		return out, ErrInvalid
	}
	e := c.publicCall(ctx, a, "POST", "/v1/organizations/"+hierarchySegment(org)+"/teams", in, &out)
	if e == nil && out.OrganizationID != org {
		e = ErrDependency
	}
	return checked(out, e, out.Validate())
}
func (c *SessionClient) CreateProject(ctx context.Context, a ResolveRequest, workspace string, in ProjectRequest) (Project, error) {
	var out Project
	if !validPath(workspace) || in.Validate() != nil {
		return out, ErrInvalid
	}
	e := c.publicCall(ctx, a, "POST", "/v1/workspaces/"+hierarchySegment(workspace)+"/projects", in, &out)
	if e == nil && (out.WorkspaceID != workspace || out.Kind != in.Kind || out.OwnerID != in.OwnerID || out.TeamID != in.TeamID) {
		e = ErrDependency
	}
	return checked(out, e, out.Validate())
}
func (c *SessionClient) SetOrganizationMembership(ctx context.Context, a ResolveRequest, org string, in MembershipRequest) (Membership, error) {
	var out Membership
	if !validPath(org) || in.Validate() != nil {
		return out, ErrInvalid
	}
	e := c.publicCall(ctx, a, "PUT", "/v1/organizations/"+hierarchySegment(org)+"/memberships/"+hierarchySegment(in.HumanID), in, &out)
	if e == nil && (out.OrganizationID != org || out.HumanID != in.HumanID || out.Role != in.Role || out.TeamID != "") {
		e = ErrDependency
	}
	return checked(out, e, out.Validate())
}
func (c *SessionClient) AddTeamMembership(ctx context.Context, a ResolveRequest, team, human string) (Membership, error) {
	var out Membership
	if !validPath(team, human) {
		return out, ErrInvalid
	}
	e := c.publicCall(ctx, a, "POST", "/v1/teams/"+hierarchySegment(team)+"/memberships", MembershipRequest{HumanID: human, Role: "member"}, &out)
	if e == nil && (out.TeamID != team || out.HumanID != human || out.Role != "member") {
		e = ErrDependency
	}
	return checked(out, e, out.Validate())
}
func (c *SessionClient) RemoveOrganizationMembership(ctx context.Context, a ResolveRequest, org, human string) (Result, error) {
	return c.remove(ctx, a, "/v1/organizations/"+hierarchySegment(org)+"/memberships/"+hierarchySegment(human), org, human)
}
func (c *SessionClient) RemoveTeamMembership(ctx context.Context, a ResolveRequest, team, human string) (Result, error) {
	return c.remove(ctx, a, "/v1/teams/"+hierarchySegment(team)+"/memberships/"+hierarchySegment(human), team, human)
}
func (c *SessionClient) RevokeInvitation(ctx context.Context, a ResolveRequest, org, id string) (Result, error) {
	return c.remove(ctx, a, "/v1/organizations/"+hierarchySegment(org)+"/invitations/"+hierarchySegment(id), org, id)
}
func (c *SessionClient) remove(ctx context.Context, a ResolveRequest, path string, ids ...string) (Result, error) {
	var out Result
	if !validPath(ids...) {
		return out, ErrInvalid
	}
	e := c.publicCall(ctx, a, "DELETE", path, struct{}{}, &out)
	if e == nil && !out.Applied {
		e = ErrDependency
	}
	return checked(out, e, nil)
}
func (c *SessionClient) IssueInvitation(ctx context.Context, a ResolveRequest, org string, in InvitationRequest) (IssuedInvitation, error) {
	var out IssuedInvitation
	if !validPath(org) || in.Validate(time.Now()) != nil {
		return out, ErrInvalid
	}
	e := c.publicCall(ctx, a, "POST", "/v1/organizations/"+hierarchySegment(org)+"/invitations", in, &out)
	if e == nil && (out.Invitation.OrganizationID != org || out.Invitation.HumanID != in.HumanID || out.Invitation.Role != in.Role || out.Invitation.State != "pending" || (AcceptInvitationRequest{Secret: out.Secret}).Validate() != nil) {
		e = ErrDependency
	}
	return checked(out, e, out.Invitation.Validate())
}
func (c *SessionClient) AcceptInvitation(ctx context.Context, a ResolveRequest, in AcceptInvitationRequest) (Result, error) {
	var out Result
	if in.Validate() != nil {
		return out, ErrInvalid
	}
	e := c.publicCall(ctx, a, "POST", "/v1/invitations/accept", in, &out)
	if e == nil && !out.Applied {
		e = ErrDependency
	}
	return checked(out, e, nil)
}
func checked[T any](out T, e, validation error) (T, error) {
	if e == nil && validation != nil {
		e = ErrDependency
	}
	if e != nil {
		var zero T
		return zero, e
	}
	return out, nil
}
func (p Organization) Validate() error {
	if !validPath(p.OrganizationID) || !ValidName(p.Name) {
		return ErrInvalid
	}
	return nil
}
func (p Workspace) Validate() error {
	if !validPath(p.OrganizationID, p.WorkspaceID) || !ValidName(p.Name) {
		return ErrInvalid
	}
	return nil
}
func (p Team) Validate() error {
	if !validPath(p.OrganizationID, p.TeamID) || !ValidName(p.Name) {
		return ErrInvalid
	}
	return nil
}
func (p Project) Validate() error {
	if !validPath(p.OrganizationID, p.WorkspaceID, p.ProjectID) {
		return ErrInvalid
	}
	return (ProjectRequest{Name: p.Name, Kind: p.Kind, OwnerID: p.OwnerID, TeamID: p.TeamID}).Validate()
}
func (p Membership) Validate() error {
	if !validPath(p.OrganizationID) || (p.TeamID != "" && (!validPath(p.TeamID) || p.Role != "member")) {
		return ErrInvalid
	}
	return (MembershipRequest{HumanID: p.HumanID, Role: p.Role}).Validate()
}
func (p Invitation) Validate() error {
	if !validPath(p.OrganizationID, p.InvitationID) || (p.State != "pending" && p.State != "consumed" && p.State != "revoked") || p.ExpiresAt.IsZero() || p.ExpiresAt.Year() > 9999 {
		return ErrInvalid
	}
	return (MembershipRequest{HumanID: p.HumanID, Role: p.Role}).Validate()
}

type validatable interface{ Validate() error }

func publicPage[T validatable](ctx context.Context, c *SessionClient, a ResolveRequest, path string, p PageRequest, scope func(T) bool) (Page[T], error) {
	var out Page[T]
	query, e := pageQuery(p)
	if e != nil {
		return out, e
	}
	e = c.publicCall(ctx, a, "GET", path+query, nil, &out)
	if e == nil {
		if out.Items == nil || len(out.Items) > p.Limit || len(out.NextCursor) > 2048 {
			e = ErrDependency
		}
		for _, item := range out.Items {
			if item.Validate() != nil || !scope(item) {
				e = ErrDependency
			}
		}
	}
	return checked(out, e, nil)
}
func (c *SessionClient) Organizations(ctx context.Context, a ResolveRequest, p PageRequest) (Page[Organization], error) {
	return publicPage(ctx, c, a, "/v1/organizations", p, func(Organization) bool { return true })
}
func (c *SessionClient) Workspaces(ctx context.Context, a ResolveRequest, org string, p PageRequest) (Page[Workspace], error) {
	if !validPath(org) {
		return Page[Workspace]{}, ErrInvalid
	}
	return publicPage(ctx, c, a, "/v1/organizations/"+hierarchySegment(org)+"/workspaces", p, func(v Workspace) bool { return v.OrganizationID == org })
}
func (c *SessionClient) Teams(ctx context.Context, a ResolveRequest, org string, p PageRequest) (Page[Team], error) {
	if !validPath(org) {
		return Page[Team]{}, ErrInvalid
	}
	return publicPage(ctx, c, a, "/v1/organizations/"+hierarchySegment(org)+"/teams", p, func(v Team) bool { return v.OrganizationID == org })
}
func (c *SessionClient) Projects(ctx context.Context, a ResolveRequest, workspace string, p PageRequest) (Page[Project], error) {
	if !validPath(workspace) {
		return Page[Project]{}, ErrInvalid
	}
	return publicPage(ctx, c, a, "/v1/workspaces/"+hierarchySegment(workspace)+"/projects", p, func(v Project) bool { return v.WorkspaceID == workspace })
}
func (c *SessionClient) OrganizationMemberships(ctx context.Context, a ResolveRequest, org string, p PageRequest) (Page[Membership], error) {
	if !validPath(org) {
		return Page[Membership]{}, ErrInvalid
	}
	return publicPage(ctx, c, a, "/v1/organizations/"+hierarchySegment(org)+"/memberships", p, func(v Membership) bool { return v.OrganizationID == org && v.TeamID == "" })
}
func (c *SessionClient) TeamMemberships(ctx context.Context, a ResolveRequest, team string, p PageRequest) (Page[Membership], error) {
	if !validPath(team) {
		return Page[Membership]{}, ErrInvalid
	}
	return publicPage(ctx, c, a, "/v1/teams/"+hierarchySegment(team)+"/memberships", p, func(v Membership) bool { return v.TeamID == team })
}
func (c *SessionClient) Invitations(ctx context.Context, a ResolveRequest, org string, p PageRequest) (Page[Invitation], error) {
	if !validPath(org) {
		return Page[Invitation]{}, ErrInvalid
	}
	return publicPage(ctx, c, a, "/v1/organizations/"+hierarchySegment(org)+"/invitations", p, func(v Invitation) bool { return v.OrganizationID == org })
}
func publicGet[T validatable](ctx context.Context, c *SessionClient, a ResolveRequest, path string, scope func(T) bool) (T, error) {
	var out T
	e := c.publicCall(ctx, a, "GET", path, nil, &out)
	if e == nil && !scope(out) {
		e = ErrDependency
	}
	return checked(out, e, out.Validate())
}
func (c *SessionClient) Organization(ctx context.Context, a ResolveRequest, id string) (Organization, error) {
	if !validPath(id) {
		return Organization{}, ErrInvalid
	}
	return publicGet(ctx, c, a, "/v1/organizations/"+hierarchySegment(id), func(v Organization) bool { return v.OrganizationID == id })
}
func (c *SessionClient) Workspace(ctx context.Context, a ResolveRequest, org, id string) (Workspace, error) {
	if !validPath(org, id) {
		return Workspace{}, ErrInvalid
	}
	return publicGet(ctx, c, a, "/v1/organizations/"+hierarchySegment(org)+"/workspaces/"+hierarchySegment(id), func(v Workspace) bool { return v.OrganizationID == org && v.WorkspaceID == id })
}
func (c *SessionClient) Team(ctx context.Context, a ResolveRequest, org, id string) (Team, error) {
	if !validPath(org, id) {
		return Team{}, ErrInvalid
	}
	return publicGet(ctx, c, a, "/v1/organizations/"+hierarchySegment(org)+"/teams/"+hierarchySegment(id), func(v Team) bool { return v.OrganizationID == org && v.TeamID == id })
}
func (c *SessionClient) Project(ctx context.Context, a ResolveRequest, workspace, id string) (Project, error) {
	if !validPath(workspace, id) {
		return Project{}, ErrInvalid
	}
	return publicGet(ctx, c, a, "/v1/workspaces/"+hierarchySegment(workspace)+"/projects/"+hierarchySegment(id), func(v Project) bool { return v.WorkspaceID == workspace && v.ProjectID == id })
}
func (c *SessionClient) Invitation(ctx context.Context, a ResolveRequest, org, id string) (Invitation, error) {
	if !validPath(org, id) {
		return Invitation{}, ErrInvalid
	}
	return publicGet(ctx, c, a, "/v1/organizations/"+hierarchySegment(org)+"/invitations/"+hierarchySegment(id), func(v Invitation) bool { return v.OrganizationID == org && v.InvitationID == id })
}
