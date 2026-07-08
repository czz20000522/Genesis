package kernel

import (
	"errors"
	"sort"
	"strings"
)

var ErrSessionSearchInvalidRequest = errors.New("session search invalid request")

const (
	defaultSessionSearchLimit = 20
	maxSessionSearchLimit     = 100
)

type sessionSearchPreview struct {
	UserText      string
	AssistantText string
}

func (k *Kernel) SearchSessions(req SessionSearchRequest) (SessionSearchResponse, error) {
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return SessionSearchResponse{}, ErrSessionSearchInvalidRequest
	}
	limit := req.Limit
	if limit == 0 {
		limit = defaultSessionSearchLimit
	}
	if limit < 0 || limit > maxSessionSearchLimit {
		return SessionSearchResponse{}, ErrSessionSearchInvalidRequest
	}

	list, err := k.ListSessions()
	if err != nil {
		return SessionSearchResponse{}, err
	}
	events, err := k.loadEvents()
	if err != nil {
		return SessionSearchResponse{}, err
	}
	previews := sessionSearchPreviews(events)
	queryFolded := strings.ToLower(query)
	results := make([]SessionSearchResult, 0, minInt(limit, len(list.Items)))
	for _, item := range list.Items {
		preview := previews[item.SessionID]
		matchFields, snippet := sessionSearchMatch(item, preview, queryFolded)
		if len(matchFields) == 0 {
			continue
		}
		results = append(results, SessionSearchResult{
			SessionID:   item.SessionID,
			Title:       item.Title,
			UpdatedAt:   item.UpdatedAt,
			MatchFields: matchFields,
			Snippet:     boundedTimelinePreview(snippet),
		})
		if len(results) >= limit {
			break
		}
	}
	if results == nil {
		results = []SessionSearchResult{}
	}
	return SessionSearchResponse{Query: query, Items: results}, nil
}

func sessionSearchPreviews(events []StoredEvent) map[string]sessionSearchPreview {
	previews := map[string]sessionSearchPreview{}
	ordered := append([]StoredEvent(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
	})
	for _, event := range ordered {
		sessionID := strings.TrimSpace(event.SessionID)
		if sessionID == "" {
			continue
		}
		preview := previews[sessionID]
		switch event.Type {
		case "turn.submitted":
			if preview.UserText == "" {
				preview.UserText = firstTextInput(event.Data.InputItems)
			}
		case "model.final":
			if event.Data.Final != nil && strings.TrimSpace(event.Data.Final.Text) != "" {
				preview.AssistantText = event.Data.Final.Text
			}
		}
		previews[sessionID] = preview
	}
	return previews
}

func sessionSearchMatch(item SessionListItem, preview sessionSearchPreview, queryFolded string) ([]string, string) {
	var fields []string
	snippet := ""
	addMatch := func(field string, value string) {
		if !strings.Contains(strings.ToLower(value), queryFolded) {
			return
		}
		fields = append(fields, field)
		if snippet == "" {
			snippet = value
		}
	}
	addMatch("session_id", item.SessionID)
	addMatch("title", item.Title)
	addMatch("user_text", preview.UserText)
	addMatch("assistant_text", preview.AssistantText)
	if snippet == "" {
		snippet = firstNonEmpty(item.Title, preview.UserText, preview.AssistantText, item.SessionID)
	}
	return fields, snippet
}

func firstTextInput(items []InputItem) string {
	for _, item := range items {
		if item.Type == "text" && strings.TrimSpace(item.Text) != "" {
			return item.Text
		}
	}
	return ""
}
