package auditv1

import (
	"errors"
	accessv1 "github.com/leafrun/leafrun-contracts/access/v1"
	identityv1 "github.com/leafrun/leafrun-contracts/identity/v1"
	"regexp"
	"time"
)

const MaxHistoryLimit = 100
const MaxCursorBytes = 4096

var historyFactIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type HistoryQuery struct {
	Scope      accessv1.PermissionScope `json:"scope"`
	Limit      int                      `json:"limit"`
	Cursor     string                   `json:"cursor,omitempty"`
	Producer   Producer                 `json:"producer,omitempty"`
	Action     ActionName               `json:"action,omitempty"`
	ResourceID string                   `json:"resourceId,omitempty"`
}

func (q HistoryQuery) Validate() error {
	if q.Scope.Validate() != nil || q.Scope.ProjectID == "" || q.Limit < 1 || q.Limit > MaxHistoryLimit || len(q.Cursor) > MaxCursorBytes {
		return errors.New("invalid history query")
	}
	if q.Producer != "" {
		if _, err := Subject(q.Producer); err != nil {
			return errors.New("invalid history query")
		}
	}
	if q.Action != "" && (actionProducer(q.Action) == "" || q.Producer != "" && actionProducer(q.Action) != q.Producer) {
		return errors.New("invalid history query")
	}
	if q.ResourceID != "" && !identityv1.ValidIdentifier(q.ResourceID) {
		return errors.New("invalid history query")
	}
	return nil
}

type HistoryItem struct {
	EventID    string                   `json:"eventId"`
	Scope      accessv1.PermissionScope `json:"scope"`
	OccurredAt time.Time                `json:"occurredAt"`
	Action     Action                   `json:"action"`
}
type HistoryPage struct {
	Items      []HistoryItem `json:"items"`
	NextCursor string        `json:"nextCursor,omitempty"`
}

func (p HistoryPage) Validate(q HistoryQuery) error {
	if q.Validate() != nil || p.Items == nil || len(p.Items) > q.Limit || len(p.NextCursor) > MaxCursorBytes {
		return errors.New("invalid history page")
	}
	seen := map[string]bool{}
	for _, i := range p.Items {
		subject, err := Subject(i.Action.Producer)
		if err != nil || i.Action.Validate(subject) != nil || i.Scope != q.Scope || !historyFactIDPattern.MatchString(i.EventID) || !historyFactIDPattern.MatchString(i.Action.OperationID) || seen[i.EventID] || i.OccurredAt.IsZero() || (q.Producer != "" && i.Action.Producer != q.Producer) || (q.Action != "" && i.Action.Action != q.Action) || (q.ResourceID != "" && i.Action.Resource.ID != q.ResourceID) {
			return errors.New("invalid history page")
		}
		seen[i.EventID] = true
	}
	return nil
}
