package keyboards

import (
	"MeowSentinelBot/internal/model"
	"fmt"
	"testing"
)

func TestStackViewKeyboard_Local1BasedButtons(t *testing.T) {
	// 5 entries with non-sequential, large DB IDs
	entries := []model.Entry{
		{ID: 101, Title: "Item A"},
		{ID: 202, Title: "Item B"},
		{ID: 303, Title: "Item C"},
		{ID: 404, Title: "Item D"},
		{ID: 505, Title: "Item E"},
	}

	kb := StackViewKeyboard(1, entries)
	if kb == nil {
		t.Fatal("expected non-nil keyboard")
	}

	// We expect:
	// Row 0: Action buttons (Add Media, Settings)
	// Row 1: Direct pickers (up to 4): #1 (ID 101), #2 (ID 202), #3 (ID 303), #4 (ID 404)
	// Row 2: Direct picker (remaining 1): #5 (ID 505)
	// Row 3: Navigation (All Stacks)

	if len(kb.InlineKeyboard) < 4 {
		t.Fatalf("expected at least 4 rows, got %d", len(kb.InlineKeyboard))
	}

	row1 := kb.InlineKeyboard[1]
	if len(row1) != 4 {
		t.Fatalf("expected 4 buttons in row 1, got %d", len(row1))
	}

	for i := 0; i < 4; i++ {
		expectedText := fmt.Sprintf("#%d", i+1)
		expectedData := fmt.Sprintf("entry:view:%d", entries[i].ID)

		if row1[i].Text != expectedText {
			t.Errorf("button %d text = %q, want %q", i, row1[i].Text, expectedText)
		}
		if row1[i].CallbackData != expectedData {
			t.Errorf("button %d callbackData = %q, want %q", i, row1[i].CallbackData, expectedData)
		}
	}

	row2 := kb.InlineKeyboard[2]
	if len(row2) != 1 {
		t.Fatalf("expected 1 button in row 2, got %d", len(row2))
	}
	if row2[0].Text != "#5" {
		t.Errorf("button 5 text = %q, want '#5'", row2[0].Text)
	}
	if row2[0].CallbackData != "entry:view:505" {
		t.Errorf("button 5 callbackData = %q, want 'entry:view:505'", row2[0].CallbackData)
	}
}
