package computeruse

import (
	"context"
	"strings"
	"testing"
)

func TestMemoryDriverNavigateAndTargetMismatch(t *testing.T) {
	t.Parallel()

	driver := NewMemoryDriver()
	session, err := driver.StartSession(context.Background(), Session{
		ComputerUseSessionID: "cusess_1",
		RunID:                "run_1",
	}, CreateSessionInput{DriverKind: "browser"})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	session, action, captures, err := driver.ExecuteAction(context.Background(), session, Action{
		ComputerUseActionID:  "cuact_nav",
		ComputerUseSessionID: session.ComputerUseSessionID,
		ActionKind:           ActionKindNavigate,
		Input:                map[string]any{"url": "https://example.test/page"},
	})
	if err != nil {
		t.Fatalf("ExecuteAction(navigate) returned error: %v", err)
	}
	if action.Status != ActionStatusCompleted || session.CurrentPage == nil || session.CurrentPage.URL != "https://example.test/page" {
		t.Fatalf("expected navigate to update current page, got session=%+v action=%+v", session, action)
	}
	if len(captures) != 0 {
		t.Fatalf("expected navigate to avoid implicit captures, got %+v", captures)
	}

	_, mismatchAction, mismatchCaptures, err := driver.ExecuteAction(context.Background(), session, Action{
		ComputerUseActionID:  "cuact_input",
		ComputerUseSessionID: session.ComputerUseSessionID,
		ActionKind:           ActionKindInput,
		TargetMatchContext: &TargetMatchContext{
			MatchStrategy:    "dom_selector",
			ExpectedSelector: "#missing-button",
		},
	})
	if err != nil {
		t.Fatalf("ExecuteAction(target mismatch) returned unexpected error: %v", err)
	}
	if mismatchAction.Status != ActionStatusFailed || mismatchAction.FailureClass != string(FailureClassTargetMismatch) {
		t.Fatalf("expected target mismatch failure, got %+v", mismatchAction)
	}
	if len(mismatchCaptures) == 0 {
		t.Fatalf("expected mismatch to preserve evidence capture request, got %+v", mismatchCaptures)
	}
}

func TestMemoryDriverSupportsHistorySelectionAndDownload(t *testing.T) {
	t.Parallel()

	driver := NewMemoryDriver()
	session, err := driver.StartSession(context.Background(), Session{
		ComputerUseSessionID: "cusess_1",
		RunID:                "run_1",
	}, CreateSessionInput{DriverKind: "browser", InitialURL: "https://example.test/start"})
	if err != nil {
		t.Fatalf("StartSession returned error: %v", err)
	}

	session.Actions = append(session.Actions, Action{
		ComputerUseActionID:  "cuact_nav_1",
		ComputerUseSessionID: session.ComputerUseSessionID,
		ActionKind:           ActionKindNavigate,
		Status:               ActionStatusCompleted,
		PageBefore:           &PageSummary{URL: "https://example.test/start", Title: "example.test"},
		PageAfter:            &PageSummary{URL: "https://example.test/first", Title: "example.test"},
	})
	session.CurrentPage = &PageSummary{URL: "https://example.test/first", Title: "example.test"}
	session, backAction, _, err := driver.ExecuteAction(context.Background(), session, Action{
		ComputerUseActionID:  "cuact_back",
		ComputerUseSessionID: session.ComputerUseSessionID,
		ActionKind:           ActionKindBack,
	})
	if err != nil {
		t.Fatalf("ExecuteAction(back) returned error: %v", err)
	}
	if backAction.Status != ActionStatusCompleted || backAction.PageAfter == nil || backAction.PageAfter.URL != "https://example.test/start" {
		t.Fatalf("expected back to restore prior page, got action=%+v", backAction)
	}

	session.Actions = append(session.Actions, backAction)
	session, forwardAction, _, err := driver.ExecuteAction(context.Background(), session, Action{
		ComputerUseActionID:  "cuact_forward",
		ComputerUseSessionID: session.ComputerUseSessionID,
		ActionKind:           ActionKindForward,
	})
	if err != nil {
		t.Fatalf("ExecuteAction(forward) returned error: %v", err)
	}
	if forwardAction.Status != ActionStatusCompleted || forwardAction.PageAfter == nil || forwardAction.PageAfter.URL != "https://example.test/first" {
		t.Fatalf("expected forward to restore next page, got action=%+v", forwardAction)
	}

	session.Actions = append(session.Actions, forwardAction)
	session, selectAction, selectCaptures, err := driver.ExecuteAction(context.Background(), session, Action{
		ComputerUseActionID:  "cuact_select",
		ComputerUseSessionID: session.ComputerUseSessionID,
		ActionKind:           ActionKindSelect,
		TargetMatchContext:   &TargetMatchContext{ExpectedSelector: "#size"},
		Input:                map[string]any{"selectedValue": "large"},
	})
	if err != nil {
		t.Fatalf("ExecuteAction(select) returned error: %v", err)
	}
	if selectAction.PageAfter == nil || !strings.Contains(selectAction.PageAfter.Title, "#size=large") {
		t.Fatalf("expected select to project selected state, got %+v", selectAction)
	}
	if session.CurrentPage == nil || session.CurrentPage.Title != selectAction.PageAfter.Title {
		t.Fatalf("expected session current page to track selection, got session=%+v action=%+v", session, selectAction)
	}
	if len(selectCaptures) == 0 {
		t.Fatalf("expected select evidence capture, got %+v", selectCaptures)
	}

	_, downloadAction, downloadCaptures, err := driver.ExecuteAction(context.Background(), session, Action{
		ComputerUseActionID:  "cuact_download",
		ComputerUseSessionID: session.ComputerUseSessionID,
		ActionKind:           ActionKindDownload,
		TargetMatchContext:   &TargetMatchContext{ExpectedSelector: "#export"},
	})
	if err != nil {
		t.Fatalf("ExecuteAction(download) returned error: %v", err)
	}
	if downloadAction.Status != ActionStatusCompleted {
		t.Fatalf("expected completed download action, got %+v", downloadAction)
	}
	var downloadCaptureFound bool
	for _, capture := range downloadCaptures {
		if capture.Kind != ArtifactKindDownload {
			continue
		}
		downloadCaptureFound = true
		if !strings.Contains(string(capture.Content), "selector=#export") {
			t.Fatalf("expected download capture to reflect page/action context, got %q", string(capture.Content))
		}
	}
	if !downloadCaptureFound {
		t.Fatalf("expected dedicated download artifact capture, got %+v", downloadCaptures)
	}
}
