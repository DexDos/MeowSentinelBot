package presenters

import (
	"MeowSentinelBot/internal/model"
	"fmt"
	"html"
	"strings"
	"time"
)

func RenderMainMenu(stacks []model.StackInfo) string {
	sb := strings.Builder{}
	sb.WriteString("🐱 <b>Welcome to MeowSentinel!</b>\n")
	sb.WriteString("<i>Your personal and collaborative media tracker.</i>\n\n")

	if len(stacks) == 0 {
		sb.WriteString("📚 <b>Your Stacks:</b>\n")
		sb.WriteString("<i>You don't have any stacks yet! Create one or redeem an invite code below.</i>")
	} else {
		sb.WriteString("📚 <b>Your Stacks:</b>\n")
		for _, s := range stacks {
			prefix := "👥 Shared"
			if s.Role == model.RoleOwner {
				prefix = "👑 Owner"
			}
			escapedTitle := html.EscapeString(s.Title)
			sb.WriteString(fmt.Sprintf("• <b>%s</b> (%s) — %d items\n", escapedTitle, prefix, s.EntriesCount))
		}
	}
	return sb.String()
}

func RenderStackView(stack *model.StackInfo, entries []model.Entry) string {
	sb := strings.Builder{}
	escapedTitle := html.EscapeString(stack.Title)
	roleLabel := "👑 Owner"
	if stack.Role != model.RoleOwner {
		roleLabel = "👥 Shared Member"
	}

	sb.WriteString(fmt.Sprintf("📂 <b>Stack: %s</b>\n", escapedTitle))
	sb.WriteString(fmt.Sprintf("<i>Role: %s</i>\n\n", roleLabel))

	if len(entries) == 0 {
		sb.WriteString("<i>This stack is currently empty!</i>\n")
		sb.WriteString("Click <b>✨ Add Media</b> to add your first movie or series.")
		return sb.String()
	}

	// Group by media type
	grouped := make(map[string][]model.Entry)
	var typeOrder []string
	for _, e := range entries {
		t := e.Type
		if t == "" {
			t = "Media"
		}
		if _, ok := grouped[t]; !ok {
			typeOrder = append(typeOrder, t)
		}
		grouped[t] = append(grouped[t], e)
	}

	for _, t := range typeOrder {
		sb.WriteString(fmt.Sprintf("<blockquote><b>:: %s</b></blockquote>\n", html.EscapeString(t)))
		for _, e := range grouped[t] {
			sb.WriteString(e.FormattedLine())
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("<i>Last updated: %s</i>", stack.UpdatedAt.Format("15:04 02.01.2006")))
	return sb.String()
}

func RenderStackSettings(stack *model.StackInfo) string {
	escapedTitle := html.EscapeString(stack.Title)
	roleLabel := "👑 Owner"
	if stack.Role != model.RoleOwner {
		roleLabel = "👥 Shared Member"
	}

	return fmt.Sprintf(
		"⚙️ <b>Stack Settings: \"%s\"</b>\n\n"+
			"• <b>Role:</b> %s\n"+
			"• <b>Total Entries:</b> %d\n"+
			"• <b>Created:</b> %s\n\n"+
			"<i>Choose an action below:</i>",
		escapedTitle,
		roleLabel,
		stack.EntriesCount,
		stack.CreatedAt.Format("15:04 02.01.2006"),
	)
}

func RenderInviteShare(inv *model.Invite, botUsername string) string {
	deepLink := fmt.Sprintf("https://t.me/%s?start=%s", botUsername, inv.Code)
	timeLeft := time.Until(inv.ExpiresAt).Round(time.Minute)

	return fmt.Sprintf(
		"🎟 <b>Temporary Invite Code:</b>\n\n"+
			"<code>%s</code>\n\n"+
			"⏳ <i>Valid for: %s</i>\n\n"+
			"Send this code to a friend! They can enter it via <b>🎟 Redeem Code</b> in the bot, or click this link:\n"+
			"%s",
		inv.Code,
		timeLeft,
		deepLink,
	)
}

func RenderEntryDetail(entry *model.Entry, userScore model.Score) string {
	escapedTitle := html.EscapeString(entry.Title)
	escapedType := html.EscapeString(entry.Type)

	sb := strings.Builder{}
	sb.WriteString(fmt.Sprintf("🎬 <b>#%d %s</b>\n", entry.ID, escapedTitle))
	sb.WriteString(fmt.Sprintf("🏷 <i>Type:</i> %s\n", escapedType))
	sb.WriteString(fmt.Sprintf("⭐️ <i>Average Rating:</i> %s (%s)\n", entry.Rating.Stars(), entry.Rating.Label()))

	if userScore > 0 {
		sb.WriteString(fmt.Sprintf("👤 <i>Your Rating:</i> %s\n", userScore.Stars()))
	}

	sb.WriteString("\n<blockquote>")
	sb.WriteString(fmt.Sprintf("<b>Watching Progress:</b>\nSeason: <b>%d</b> | Episode: <b>%d</b>", entry.Season, entry.Episode))
	sb.WriteString("</blockquote>")

	return sb.String()
}

func RenderPrompt(title, hint string) string {
	return fmt.Sprintf("✍️ <b>%s</b>\n\n<i>%s</i>", html.EscapeString(title), html.EscapeString(hint))
}

func RenderError(msg string) string {
	return fmt.Sprintf("❌ <b>Oops!</b>\n\n<i>%s</i>", html.EscapeString(msg))
}
