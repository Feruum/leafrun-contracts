package accessv1

import (
	"context"
)

func (c *SessionClient) CreateBot(ctx context.Context, auth ResolveRequest, workspace, project string, in CreateBotRequest) (Bot, error) {
	if !validPath(workspace, project) || in.Validate() != nil {
		return Bot{}, ErrInvalid
	}
	var out Bot
	e := c.publicCall(ctx, auth, "POST", "/v1/workspaces/"+hierarchySegment(workspace)+"/projects/"+hierarchySegment(project)+"/bots", in, &out)
	if e != nil {
		return Bot{}, e
	}
	if !validPath(out.BotID) || out.Scope.Validate() != nil || out.Scope.WorkspaceID != workspace || out.Scope.ProjectID != project || out.Name != in.Name || (NameRequest{Name: out.Name}).Validate() != nil {
		return Bot{}, ErrDependency
	}
	return out, nil
}
func (c *SessionClient) IssueBotKey(ctx context.Context, auth ResolveRequest, bot string, in BotKeyRequest) (BotKeyResult, error) {
	if !validPath(bot) || in.ValidateShape() != nil {
		return BotKeyResult{}, ErrInvalid
	}
	out, e := c.botKeyCall(ctx, auth, "POST", "/v1/bots/"+hierarchySegment(bot)+"/keys", bot, in)
	if e == nil && (out.Key.Version != 1 || out.Key.Status != "active") {
		return BotKeyResult{}, ErrDependency
	}
	return out, e
}
func (c *SessionClient) RotateBotKey(ctx context.Context, auth ResolveRequest, bot, key string, in RotateBotKeyRequest) (BotKeyResult, error) {
	if !validPath(bot, key) || in.ValidateShape() != nil {
		return BotKeyResult{}, ErrInvalid
	}
	out, e := c.botKeyCall(ctx, auth, "POST", "/v1/bots/"+hierarchySegment(bot)+"/keys/"+hierarchySegment(key)+"/rotate", bot, in)
	if e == nil && (out.Key.KeyID != key || out.Key.Version != in.ExpectedVersion+1 || out.Key.Status != "active") {
		return BotKeyResult{}, ErrDependency
	}
	return out, e
}
func (c *SessionClient) RevokeBotKey(ctx context.Context, auth ResolveRequest, bot, key string, in RevokeBotKeyRequest) (BotKeyResult, error) {
	if !validPath(bot, key) || in.Validate() != nil {
		return BotKeyResult{}, ErrInvalid
	}
	out, e := c.botKeyCall(ctx, auth, "DELETE", "/v1/bots/"+hierarchySegment(bot)+"/keys/"+hierarchySegment(key), bot, in)
	if e == nil && (out.Key.KeyID != key || out.Key.Version != in.ExpectedVersion+1 || out.Key.Status != "revoked" || out.Disclosure != "unavailable") {
		return BotKeyResult{}, ErrDependency
	}
	return out, e
}
func (c *SessionClient) botKeyCall(ctx context.Context, auth ResolveRequest, method, path, bot string, in any) (BotKeyResult, error) {
	var out BotKeyResult
	e := c.publicCall(ctx, auth, method, path, in, &out)
	if e != nil {
		return BotKeyResult{}, e
	}
	if out.Key.BotID != bot || out.Validate() != nil {
		return BotKeyResult{}, ErrDependency
	}
	return out, nil
}
