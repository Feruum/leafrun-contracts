package auditv1

import (
	accessv1 "github.com/leafrun/leafrun-contracts/access/v1"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	"reflect"
	"strings"
	"time"
)

type schemaBuilder struct{ schemas map[string]any }

func (b *schemaBuilder) schema(t reflect.Type) any {
	if t == reflect.TypeFor[time.Time]() {
		return map[string]any{"type": "string", "format": "date-time"}
	}
	switch t.Kind() {
	case reflect.String:
		s := map[string]any{}
		for key, value := range b.schemas["Principal"].(map[string]any)["properties"].(map[string]any)["subject"].(map[string]any) {
			s[key] = value
		}
		if t == reflect.TypeFor[Producer]() {
			s["enum"] = []string{"access", "projects", "generation", "storage"}
		}
		if t == reflect.TypeFor[Outcome]() {
			s["enum"] = []string{"succeeded", "denied", "failed", "admitted", "completed", "cancelled", "rejected"}
		}
		if t == reflect.TypeFor[ResourceKind]() {
			s["enum"] = []string{"identity", "authority", "organization", "workspace", "project", "membership", "invitation", "role", "bot", "key", "grant", "guest", "consent", "release", "job", "artifact", "storage_object"}
		}
		if t == reflect.TypeFor[ActionName]() {
			s["enum"] = []ActionName{ActionIdentityResolve, ActionIdentityCreate, ActionIdentityUpdate, ActionIdentityRevoke, ActionHierarchyRegister, ActionOrganizationCreate, ActionOrganizationUpdate, ActionOrganizationDelete, ActionWorkspaceCreate, ActionWorkspaceUpdate, ActionWorkspaceDelete, ActionProjectCreate, ActionProjectUpdate, ActionProjectDelete, ActionMembershipAdd, ActionMembershipUpdate, ActionMembershipRemove, ActionInvitationCreate, ActionInvitationAccept, ActionInvitationRevoke, ActionRoleAssign, ActionRoleRevoke, ActionBotCreate, ActionBotUpdate, ActionBotRevoke, ActionKeyCreate, ActionKeyRotate, ActionKeyRevoke, ActionGrantCreate, ActionGrantUpdate, ActionGrantRevoke, ActionGuestCreate, ActionGuestExchange, ActionGuestRevoke, ActionConsentGrant, ActionConsentRevoke, ActionReleasePublish, ActionReleaseRetire, ActionGenerationAdmission, ActionGenerationTerminal, ActionDownloadAuthorized}
		}
		return s
	case reflect.Int:
		return map[string]any{"type": "integer"}
	case reflect.Uint64:
		return map[string]any{"type": "integer", "format": "int64", "minimum": 1, "maximum": identityv1.MaxAuthorityVersion}
	case reflect.Slice:
		return map[string]any{"type": "array", "items": b.schema(t.Elem())}
	case reflect.Struct:
		name := t.Name()
		if _, exists := b.schemas[name]; !exists {
			b.schemas[name] = map[string]any{}
			props := map[string]any{}
			required := []string{}
			for i := 0; i < t.NumField(); i++ {
				f := t.Field(i)
				tag := strings.Split(f.Tag.Get("json"), ",")
				props[tag[0]] = b.schema(f.Type)
				if len(tag) == 1 {
					required = append(required, tag[0])
				}
			}
			b.schemas[name] = map[string]any{"type": "object", "additionalProperties": false, "properties": props, "required": required}
		}
		return map[string]any{"$ref": "#/components/schemas/" + name}
	}
	panic("unsupported Audit DTO")
}
func NewOpenAPI() map[string]any {
	access := accessv1.NewOpenAPI()
	shared := access["components"].(map[string]any)["schemas"].(map[string]any)
	b := schemaBuilder{schemas: map[string]any{"Principal": shared["Principal"], "AuthorityRef": shared["AuthorityRef"], "PermissionScope": shared["PermissionScope"]}}
	response := b.schema(reflect.TypeFor[HistoryPage]())
	b.schema(reflect.TypeFor[HistoryQuery]())
	b.schemas["Action"].(map[string]any)["properties"].(map[string]any)["schemaVersion"] = map[string]any{"type": "integer", "const": 1}
	b.schemas["Action"].(map[string]any)["properties"].(map[string]any)["operationId"] = map[string]any{"type": "string", "format": "uuid", "pattern": historyFactIDPattern.String()}
	b.schemas["HistoryItem"].(map[string]any)["properties"].(map[string]any)["eventId"] = map[string]any{"type": "string", "format": "uuid", "pattern": historyFactIDPattern.String()}
	b.schemas["HistoryItem"].(map[string]any)["properties"].(map[string]any)["scope"] = map[string]any{"allOf": []any{map[string]any{"$ref": "#/components/schemas/PermissionScope"}, map[string]any{"required": []string{"organizationId", "workspaceId", "projectId"}}}}
	b.schemas["HistoryQuery"].(map[string]any)["properties"].(map[string]any)["scope"] = b.schemas["HistoryItem"].(map[string]any)["properties"].(map[string]any)["scope"]
	b.schemas["HistoryQuery"].(map[string]any)["properties"].(map[string]any)["limit"] = map[string]any{"type": "integer", "minimum": 1, "maximum": MaxHistoryLimit}
	b.schemas["HistoryQuery"].(map[string]any)["properties"].(map[string]any)["cursor"] = map[string]any{"type": "string", "minLength": 1, "maxLength": MaxCursorBytes}
	b.schemas["HistoryPage"].(map[string]any)["properties"].(map[string]any)["items"].(map[string]any)["maxItems"] = MaxHistoryLimit
	b.schemas["HistoryPage"].(map[string]any)["properties"].(map[string]any)["nextCursor"] = map[string]any{"type": "string", "minLength": 1, "maxLength": MaxCursorBytes, "description": "Opaque encrypted cursor bound to reader, complete scope, filters, limit, descending insertion order and finite expiry. Current authorization is always rechecked."}
	parameters := []any{}
	for _, name := range []string{"organizationId", "workspaceId", "projectId", "producer", "action", "resourceId", "cursor", "limit"} {
		schema := any(map[string]any{"type": "string", "minLength": 1, "maxLength": 256})
		if name == "limit" {
			schema = map[string]any{"type": "integer", "minimum": 1, "maximum": MaxHistoryLimit, "default": 25}
		}
		if name == "cursor" {
			schema = map[string]any{"type": "string", "minLength": 1, "maxLength": MaxCursorBytes}
		}
		if name == "producer" {
			schema = b.schema(reflect.TypeFor[Producer]())
		}
		if name == "action" {
			schema = b.schema(reflect.TypeFor[ActionName]())
		}
		parameters = append(parameters, map[string]any{"in": "query", "name": name, "required": name == "organizationId" || name == "workspaceId" || name == "projectId", "schema": schema})
	}
	responses := map[string]any{"200": map[string]any{"description": "Exact complete tenant history in descending insertion order; identity-only actions are excluded.", "content": map[string]any{"application/json": map[string]any{"schema": response}}}}
	for _, code := range []string{"400", "401", "403", "503"} {
		responses[code] = map[string]any{"description": "Invalid query/cursor, invalid authentication, current permission denied or dependency uncertainty. Fixed safe errors."}
	}
	return map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Leafrun Audit", "version": "v1"}, "paths": map[string]any{"/v1/audit/actions": map[string]any{"get": map[string]any{"operationId": "listAuditActions", "description": "Authenticate via Access and require current audit.read on the exact complete scope on every page. Duplicate/unknown query fields rejected. Public callers cannot create audit records.", "parameters": parameters, "security": []any{map[string]any{"userBearer": []string{}}, map[string]any{"sessionCookie": []string{}}, map[string]any{"guestCookie": []string{}}}, "responses": responses}}}, "components": map[string]any{"schemas": b.schemas, "securitySchemes": map[string]any{"userBearer": map[string]any{"type": "http", "scheme": "bearer"}, "sessionCookie": map[string]any{"type": "apiKey", "in": "cookie", "name": "ory_kratos_session"}, "guestCookie": map[string]any{"type": "apiKey", "in": "cookie", "name": "leafrun_guest_session"}}}}
}
