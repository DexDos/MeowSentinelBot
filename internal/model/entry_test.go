package model

import (
	"strings"
	"testing"
)

func TestEntry_FormattedLine_LocalIndex(t *testing.T) {
	entry := Entry{
		ID:      777,
		StackID: 1,
		Title:   "Attack on Titan",
		Type:    "Anime",
		Season:  4,
		Episode: 28,
		Rating:  MEOW,
	}

	line := entry.FormattedLine(1)
	if !strings.HasPrefix(line, "<b>#1</b>") {
		t.Errorf("expected line to start with <b>#1</b>, got: %s", line)
	}
	if strings.Contains(line, "#777") {
		t.Errorf("expected line NOT to contain DB ID #777, got: %s", line)
	}

	line2 := entry.FormattedLine(4)
	if !strings.HasPrefix(line2, "<b>#4</b>") {
		t.Errorf("expected line to start with <b>#4</b>, got: %s", line2)
	}
}
