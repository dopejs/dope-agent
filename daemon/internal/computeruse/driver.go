package computeruse

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"
)

type Driver interface {
	StartSession(context.Context, Session, CreateSessionInput) (Session, error)
	ExecuteAction(context.Context, Session, Action) (Session, Action, []ArtifactCaptureRequest, error)
	CloseSession(context.Context, Session) (Session, error)
}

type MemoryDriver struct{}

func NewMemoryDriver() *MemoryDriver {
	return &MemoryDriver{}
}

func (d *MemoryDriver) StartSession(_ context.Context, session Session, input CreateSessionInput) (Session, error) {
	session.Status = SessionStatusActive
	session.DriverKind = firstNonEmpty(strings.TrimSpace(input.DriverKind), "browser")
	if strings.TrimSpace(input.InitialURL) != "" {
		page := &PageSummary{URL: strings.TrimSpace(input.InitialURL), Title: titleFromURL(input.InitialURL)}
		session.CurrentPage = page
		session.TrustedPageScope = nextTrustedScope(session, "", page)
	}
	return session, nil
}

func (d *MemoryDriver) CloseSession(_ context.Context, session Session) (Session, error) {
	now := time.Now().UTC()
	session.Status = SessionStatusClosed
	session.ClosedAt = &now
	session.UpdatedAt = now
	return session, nil
}

func (d *MemoryDriver) ExecuteAction(_ context.Context, session Session, action Action) (Session, Action, []ArtifactCaptureRequest, error) {
	session.CurrentPage = navigationCurrentPage(session)
	now := time.Now().UTC()
	action.Status = ActionStatusRunning
	action.UpdatedAt = now
	action.PageBefore = clonePage(session.CurrentPage)

	var captures []ArtifactCaptureRequest
	switch action.ActionKind {
	case ActionKindNavigate:
		rawURL, _ := action.Input["url"].(string)
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" {
			return session, failAction(action, FailureClassNavigationFailure, "navigate action requires url"), nil, fmt.Errorf("navigate action requires url")
		}
		page := &PageSummary{URL: rawURL, Title: titleFromURL(rawURL)}
		action.PageAfter = page
		session.CurrentPage = clonePage(page)
		session.TrustedPageScope = nextTrustedScope(session, action.ComputerUseActionID, page)
	case ActionKindWait:
		action.PageAfter = clonePage(session.CurrentPage)
	case ActionKindScreenshot, ActionKindSnapshot:
		action.PageAfter = clonePage(session.CurrentPage)
		captures = append(captures, buildPageEvidenceCapture(session, action, action.ActionKind))
	case ActionKindClick, ActionKindInput, ActionKindSelect, ActionKindDownload:
		if mismatched(action.TargetMatchContext) {
			return session, failAction(action, FailureClassTargetMismatch, "approved target no longer matches current page"), []ArtifactCaptureRequest{buildPageEvidenceCapture(session, action, ActionKindSnapshot)}, nil
		}
		action.PageAfter = clonePage(session.CurrentPage)
		if action.ActionKind == ActionKindSelect {
			action.PageAfter = applySelectionState(action.PageAfter, action)
			session.CurrentPage = clonePage(action.PageAfter)
		}
		captures = append(captures, buildPageEvidenceCapture(session, action, ActionKindSnapshot))
		if action.ActionKind == ActionKindDownload {
			captures = append(captures, ArtifactCaptureRequest{
				RunID:                session.RunID,
				ComputerUseSessionID: session.ComputerUseSessionID,
				ComputerUseActionID:  action.ComputerUseActionID,
				Kind:                 ArtifactKindDownload,
				MIMEType:             "text/plain",
				FileName:             downloadFileName(action),
				Content:              []byte(downloadArtifactContent(action)),
			})
		}
	case ActionKindBack, ActionKindForward:
		backStack, forwardStack := navigationHistory(session)
		switch action.ActionKind {
		case ActionKindBack:
			if len(backStack) == 0 {
				return session, failAction(action, FailureClassNavigationFailure, "back action requires prior page history"), nil, fmt.Errorf("back action requires prior page history")
			}
			action.PageAfter = clonePage(backStack[len(backStack)-1])
		case ActionKindForward:
			if len(forwardStack) == 0 {
				return session, failAction(action, FailureClassNavigationFailure, "forward action requires forward page history"), nil, fmt.Errorf("forward action requires forward page history")
			}
			action.PageAfter = clonePage(forwardStack[len(forwardStack)-1])
		}
		session.CurrentPage = clonePage(action.PageAfter)
		session.TrustedPageScope = nextTrustedScope(session, action.ComputerUseActionID, action.PageAfter)
	case ActionKindCloseSession:
		closedSession, err := d.CloseSession(context.Background(), session)
		if err != nil {
			return session, failAction(action, FailureClassUnavailableConsumer, err.Error()), nil, err
		}
		session = closedSession
		action.PageAfter = clonePage(session.CurrentPage)
	default:
		return session, failAction(action, FailureClassUnsupportedAction, "action kind is not supported in phase 26"), nil, fmt.Errorf("unsupported action kind %q", action.ActionKind)
	}

	completedAt := time.Now().UTC()
	action.Status = ActionStatusCompleted
	action.UpdatedAt = completedAt
	action.CompletedAt = &completedAt
	session.LastActionID = action.ComputerUseActionID
	if action.ActionKind != ActionKindCloseSession {
		session.Status = SessionStatusActive
	}
	session.UpdatedAt = completedAt
	return session, action, captures, nil
}

func buildPageEvidenceCapture(session Session, action Action, kind ActionKind) ArtifactCaptureRequest {
	artifactKind := ArtifactKindPageSnapshot
	mimeType := "application/json"
	fileName := "page-snapshot.json"
	if kind == ActionKindScreenshot {
		artifactKind = ArtifactKindScreenshot
		mimeType = "text/plain"
		fileName = "screenshot.txt"
	}
	content := []byte(fmt.Sprintf("{\"sessionId\":%q,\"actionId\":%q,\"actionKind\":%q,\"url\":%q,\"title\":%q,\"value\":%q,\"selectedValue\":%q}\n",
		session.ComputerUseSessionID,
		action.ComputerUseActionID,
		action.ActionKind,
		firstPageField(action.PageAfter, action.PageBefore, "url"),
		firstPageField(action.PageAfter, action.PageBefore, "title"),
		inputString(action.Input, "value"),
		inputString(action.Input, "selectedValue"),
	))
	if artifactKind == ArtifactKindScreenshot {
		content = []byte(fmt.Sprintf("screenshot placeholder for %s (%s)\n", firstPageField(action.PageAfter, action.PageBefore, "url"), action.ActionKind))
	}
	return ArtifactCaptureRequest{
		RunID:                session.RunID,
		ComputerUseSessionID: session.ComputerUseSessionID,
		ComputerUseActionID:  action.ComputerUseActionID,
		Kind:                 artifactKind,
		MIMEType:             mimeType,
		FileName:             fileName,
		Content:              content,
	}
}

func navigationCurrentPage(session Session) *PageSummary {
	current, _, _ := navigationState(session)
	return current
}

func navigationHistory(session Session) ([]*PageSummary, []*PageSummary) {
	_, back, forward := navigationState(session)
	return back, forward
}

func navigationState(session Session) (*PageSummary, []*PageSummary, []*PageSummary) {
	var (
		current *PageSummary
		back    []*PageSummary
		forward []*PageSummary
	)
	for _, prior := range session.Actions {
		if prior.PageBefore != nil {
			current = clonePage(prior.PageBefore)
			break
		}
	}
	if current == nil {
		current = clonePage(session.CurrentPage)
	}
	for _, prior := range session.Actions {
		if prior.Status != ActionStatusCompleted {
			continue
		}
		switch prior.ActionKind {
		case ActionKindNavigate:
			if prior.PageAfter == nil {
				continue
			}
			if current != nil && !samePage(current, prior.PageAfter) {
				back = append(back, clonePage(current))
			}
			current = clonePage(prior.PageAfter)
			forward = nil
		case ActionKindBack:
			if len(back) == 0 {
				continue
			}
			if current != nil {
				forward = append(forward, clonePage(current))
			}
			current = clonePage(back[len(back)-1])
			back = back[:len(back)-1]
		case ActionKindForward:
			if len(forward) == 0 {
				continue
			}
			if current != nil {
				back = append(back, clonePage(current))
			}
			current = clonePage(forward[len(forward)-1])
			forward = forward[:len(forward)-1]
		case ActionKindSelect:
			if prior.PageAfter != nil {
				current = clonePage(prior.PageAfter)
			}
		}
	}
	return current, back, forward
}

func samePage(left, right *PageSummary) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.URL == right.URL && left.Title == right.Title
}

func applySelectionState(page *PageSummary, action Action) *PageSummary {
	next := clonePage(page)
	if next == nil {
		next = &PageSummary{}
	}
	label := strings.TrimSpace(inputString(action.Input, "selectedValue"))
	if label == "" {
		label = "selected"
	}
	if selector := strings.TrimSpace(selectorFromTarget(action.TargetMatchContext)); selector != "" {
		next.Title = strings.TrimSpace(fmt.Sprintf("%s [%s=%s]", firstNonEmpty(next.Title, next.URL, "page"), selector, label))
		return next
	}
	next.Title = strings.TrimSpace(fmt.Sprintf("%s [selected=%s]", firstNonEmpty(next.Title, next.URL, "page"), label))
	return next
}

func selectorFromTarget(target *TargetMatchContext) string {
	if target == nil {
		return ""
	}
	return strings.TrimSpace(target.ExpectedSelector)
}

func inputString(input map[string]any, key string) string {
	if input == nil {
		return ""
	}
	value, _ := input[key].(string)
	return strings.TrimSpace(value)
}

func downloadFileName(action Action) string {
	base := strings.TrimSpace(firstPageField(action.PageAfter, action.PageBefore, "title"))
	if base == "" {
		base = "computer-use-download"
	}
	base = strings.ToLower(strings.ReplaceAll(base, " ", "-"))
	base = strings.Trim(base, "-")
	if base == "" {
		base = "computer-use-download"
	}
	return path.Base(base) + ".txt"
}

func downloadArtifactContent(action Action) string {
	return fmt.Sprintf(
		"download artifact\nurl=%s\ntitle=%s\naction=%s\nselector=%s\n",
		firstPageField(action.PageAfter, action.PageBefore, "url"),
		firstPageField(action.PageAfter, action.PageBefore, "title"),
		action.ActionKind,
		selectorFromTarget(action.TargetMatchContext),
	)
}

func nextTrustedScope(session Session, actionID string, page *PageSummary) *TrustedPageScope {
	revision := 1
	if session.TrustedPageScope != nil {
		revision = session.TrustedPageScope.ScopeRevision + 1
	}
	now := time.Now().UTC()
	return &TrustedPageScope{
		ScopeID:              fmt.Sprintf("cuscope_%d", now.UnixNano()),
		ComputerUseSessionID: session.ComputerUseSessionID,
		Origin:               originFromURL(page.URL),
		PageURL:              page.URL,
		Title:                page.Title,
		ScopeRevision:        revision,
		DerivedFromActionID:  actionID,
		CreatedAt:            now,
	}
}

func mismatched(target *TargetMatchContext) bool {
	if target == nil {
		return false
	}
	expectedSelector := strings.ToLower(strings.TrimSpace(target.ExpectedSelector))
	expectedText := strings.ToLower(strings.TrimSpace(target.ExpectedText))
	expectedURL := strings.ToLower(strings.TrimSpace(target.ExpectedPageURL))
	return strings.Contains(expectedSelector, "missing") || strings.Contains(expectedText, "missing") || strings.Contains(expectedURL, "missing")
}

func failAction(action Action, class FailureClass, reason string) Action {
	now := time.Now().UTC()
	action.Status = ActionStatusFailed
	action.FailureClass = string(class)
	action.FailureReason = reason
	action.UpdatedAt = now
	action.CompletedAt = &now
	if action.TargetMatchContext != nil && class == FailureClassTargetMismatch {
		action.TargetMatchContext.MatchResult = MatchResultMismatched
		action.TargetMatchContext.EvaluatedAt = now
	}
	return action
}

func titleFromURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}
	if parsed.Host != "" {
		return parsed.Host
	}
	if parsed.Path != "" {
		return parsed.Path
	}
	return strings.TrimSpace(raw)
}

func originFromURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func clonePage(page *PageSummary) *PageSummary {
	if page == nil {
		return nil
	}
	cloned := *page
	return &cloned
}

func firstPageField(after, before *PageSummary, field string) string {
	switch field {
	case "url":
		if after != nil && after.URL != "" {
			return after.URL
		}
		if before != nil {
			return before.URL
		}
	case "title":
		if after != nil && after.Title != "" {
			return after.Title
		}
		if before != nil {
			return before.Title
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
