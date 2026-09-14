package model

type Score int

const (
	NotSeen Score = iota
	Nasty
	Bad
	Ick
	WorthMeowing
	NiceOne
	MEOW
)

type Rating struct {
	ScoreValue Score
}

func (r Rating) String() string {
	switch r.ScoreValue {
	case NotSeen:
		return "—"
	case Nasty:
		return "⭐️"
	case Bad:
		return "⭐️⭐️"
	case Ick:
		return "⭐️⭐️⭐️"
	case WorthMeowing:
		return "⭐️⭐️⭐️⭐️"
	case NiceOne:
		return "⭐️⭐️⭐️⭐️⭐️"
	case MEOW:
		return "🌟✨🌟✨🌟"
	default:
		return "?"
	}
}
