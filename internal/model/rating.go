package model

import "fmt"

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

type RatingFull struct {
	Rating
	RaterTgID       int64
	RaterTgUsername string
	EntryID         int64
	ID              int64
}

type Rating struct {
	ScoreValue Score
}

func (r Rating) SetScore(s int64) Rating {
	if s < 0 {
		return Rating{ScoreValue: Nasty}
	}
	if s > 6 {
		return Rating{ScoreValue: MEOW}
	}
	return Rating{ScoreValue: Score(s)}
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

// RawRating is a helper struct to safely extract Rating via PrepareRating() func
type RawRating struct {
	ScoreValue Score
}

func (r *RawRating) Scan(value any) error {
	if value == nil {
		r.ScoreValue = NotSeen
		return nil
	}

	switch v := value.(type) {
	case int64:
		r.ScoreValue = Score(v)
	case []byte:
		var score int
		if _, err := fmt.Sscanf(string(v), "%d", &score); err != nil {
			return fmt.Errorf("failed to scan Rating: %w", err)
		}
		r.ScoreValue = Score(score)
	default:
		return fmt.Errorf("unsupported type for Rating: %T", value)
	}
	return nil
}

func (r *RawRating) PrepareRating() Rating {
	return Rating{r.ScoreValue}
}
