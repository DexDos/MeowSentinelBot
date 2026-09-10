package model

type Score int

const (
	Nasty Score = iota
	Bad
	Ick
	WorthMeowing
	NiceOne
	MEOW
)

type Rating struct {
	ScoreValue Score
}
