package presenters

import (
	"MeowSentinelBot/internal/model"
	"strings"
	"testing"
	"time"
)

func TestRenderStackView_Local1BasedIndexing(t *testing.T) {
	stack := &model.StackInfo{
		ID:           1,
		Title:        "Movie Night",
		Role:         model.RoleOwner,
		EntriesCount: 3,
		UpdatedAt:    time.Now(),
	}

	// Deliberately non-sequential, large global DB IDs
	entries := []model.Entry{
		{
			ID:      42,
			StackID: 1,
			Title:   "Spirited Away",
			Type:    "Anime",
			Season:  1,
			Episode: 1,
			Rating:  model.NiceOne,
		},
		{
			ID:      999,
			StackID: 1,
			Title:   "Princess Mononoke",
			Type:    "Anime",
			Season:  1,
			Episode: 1,
			Rating:  model.MEOW,
		},
		{
			ID:      1337,
			StackID: 1,
			Title:   "Breaking Bad",
			Type:    "TV Series",
			Season:  5,
			Episode: 16,
			Rating:  model.MEOW,
		},
	}

	rendered := RenderStackView(stack, entries)

	// Must contain local 1-based indices #1, #2, #3
	if !strings.Contains(rendered, "<b>#1</b>") {
		t.Errorf("expected rendered stack view to contain local index <b>#1</b>, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "<b>#2</b>") {
		t.Errorf("expected rendered stack view to contain local index <b>#2</b>, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "<b>#3</b>") {
		t.Errorf("expected rendered stack view to contain local index <b>#3</b>, got:\n%s", rendered)
	}

	// Must NOT expose global DB IDs (42, 999, 1337)
	if strings.Contains(rendered, "<b>#42</b>") || strings.Contains(rendered, "<b>#999</b>") || strings.Contains(rendered, "<b>#1337</b>") {
		t.Errorf("rendered stack view leaked global DB IDs! Output:\n%s", rendered)
	}
}

func TestRenderEntryDetail_LocalIndex(t *testing.T) {
	entry := &model.Entry{
		ID:      54321, // global DB ID
		StackID: 1,
		Title:   "Severance",
		Type:    "TV Series",
		Season:  1,
		Episode: 9,
		Rating:  model.NiceOne,
	}

	// Test with local index 2
	renderedWithIndex := RenderEntryDetail(entry, model.NiceOne, 2)
	if !strings.Contains(renderedWithIndex, "🎬 <b>#2 Severance</b>") {
		t.Errorf("expected local index #2 in entry detail, got:\n%s", renderedWithIndex)
	}
	if strings.Contains(renderedWithIndex, "54321") {
		t.Errorf("entry detail leaked global DB ID 54321, got:\n%s", renderedWithIndex)
	}

	// Test without local index (0)
	renderedWithoutIndex := RenderEntryDetail(entry, model.NiceOne, 0)
	if !strings.Contains(renderedWithoutIndex, "🎬 <b>Severance</b>") {
		t.Errorf("expected title without index when localIndex=0, got:\n%s", renderedWithoutIndex)
	}
	if strings.Contains(renderedWithoutIndex, "54321") {
		t.Errorf("entry detail leaked global DB ID 54321 when localIndex=0, got:\n%s", renderedWithoutIndex)
	}
}

func TestPresenters_HTMLMarkupInjection(t *testing.T) {
	maliciousTitle := "<script>alert('xss')</script><b>Bold & Dangerous</b>"
	maliciousType := "<svg/onload=alert(1)>"

	stack := &model.StackInfo{
		ID:           1,
		Title:        maliciousTitle,
		Role:         model.RoleOwner,
		EntriesCount: 1,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	entry := model.Entry{
		ID:      1,
		StackID: 1,
		Title:   maliciousTitle,
		Type:    maliciousType,
		Season:  1,
		Episode: 1,
		Rating:  model.MEOW,
	}

	// 1. RenderMainMenu
	mainMenu := RenderMainMenu([]model.StackInfo{*stack})
	if strings.Contains(mainMenu, "<script>") || strings.Contains(mainMenu, "<b>Bold") {
		t.Errorf("RenderMainMenu failed to escape HTML injection: %s", mainMenu)
	}
	if !strings.Contains(mainMenu, "&lt;script&gt;") {
		t.Errorf("RenderMainMenu missing escaped entity: %s", mainMenu)
	}

	// 2. RenderStackView
	stackView := RenderStackView(stack, []model.Entry{entry})
	if strings.Contains(stackView, "<script>") || strings.Contains(stackView, "<svg/onload") {
		t.Errorf("RenderStackView failed to escape HTML injection: %s", stackView)
	}
	if !strings.Contains(stackView, "&lt;script&gt;") || !strings.Contains(stackView, "&lt;svg/onload=alert(1)&gt;") {
		t.Errorf("RenderStackView missing escaped entities: %s", stackView)
	}

	// 3. RenderEntryDetail
	detail := RenderEntryDetail(&entry, model.MEOW, 1)
	if strings.Contains(detail, "<script>") || strings.Contains(detail, "<svg/onload") {
		t.Errorf("RenderEntryDetail failed to escape HTML injection: %s", detail)
	}
	if !strings.Contains(detail, "&lt;script&gt;") || !strings.Contains(detail, "&lt;svg/onload=alert(1)&gt;") {
		t.Errorf("RenderEntryDetail missing escaped entities: %s", detail)
	}

	// 4. RenderStackSettings
	settings := RenderStackSettings(stack)
	if strings.Contains(settings, "<script>") {
		t.Errorf("RenderStackSettings failed to escape HTML injection: %s", settings)
	}

	// 5. RenderPrompt & RenderError
	prompt := RenderPrompt(maliciousTitle, maliciousTitle)
	if strings.Contains(prompt, "<script>") {
		t.Errorf("RenderPrompt failed to escape HTML injection: %s", prompt)
	}

	errView := RenderError(maliciousTitle)
	if strings.Contains(errView, "<script>") {
		t.Errorf("RenderError failed to escape HTML injection: %s", errView)
	}
}
