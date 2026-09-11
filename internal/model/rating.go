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
