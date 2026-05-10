package matrix

import "strings"

type DedupeCache struct {
	seen map[string]struct{}
}

func NewDedupeCache() *DedupeCache {
	return &DedupeCache{seen: map[string]struct{}{}}
}

func (c *DedupeCache) MarkDuplicate(event InboundEvent) bool {
	key := DedupeKey(event)
	if _, ok := c.seen[key]; ok {
		return true
	}
	c.seen[key] = struct{}{}
	return false
}

func DedupeKey(event InboundEvent) string {
	return strings.Join([]string{
		strings.TrimSpace(event.TenantID),
		strings.TrimSpace(event.ConnectorID),
		strings.TrimSpace(event.HomeserverID),
		strings.TrimSpace(event.ConversationID),
		strings.TrimSpace(event.MatrixEventID),
	}, "\x00")
}
