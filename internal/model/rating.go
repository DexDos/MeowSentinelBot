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

func (s Score) Label() string {
	switch s {
	case NotSeen:
		return "Not seen"
	case Nasty:
		return "Nasty (1/6)"
	case Bad:
		return "Bad (2/6)"
	case Ick:
		return "Ick (3/6)"
	case WorthMeowing:
		return "Worth Meowing (4/6)"
	case NiceOne:
		return "Nice One (5/6)"
	case MEOW:
		return "MEOW (6/6)"
	default:
		return "Unknown"
	}
}

func (s Score) Stars() string {
	switch s {
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
		return "—"
	}
}

func (s Score) Int() int {
	return int(s)
}

func ScoreFromInt(val int64) Score {
	if val < 0 {
		return NotSeen
	}
	if val > 6 {
		return MEOW
	}
	return Score(val)
}

type RatingFull struct {
	ID              int64
	EntryID         int64
	RaterTgID       int64
	RaterTgUsername string
	Score           Score
}
