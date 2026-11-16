package tournaments

import (
	"strconv"

	"github.com/stodulski/vibe-server/internal/events"
	"github.com/stodulski/vibe-server/internal/sports"
)

// The modalities a category can carry, the genders a player declares and the level
// bounds are the events domain's: a category and an open-play event share them.
const (
	ModalityMen   = events.ModalityMen
	ModalityWomen = events.ModalityWomen
	ModalityMixed = events.ModalityMixed
	GenderMen     = events.GenderMen
	GenderWomen   = events.GenderWomen
	MinLevel      = events.MinLevel
	MaxLevel      = events.MaxLevel
)

// The range of a category's suma (the two players' levels added together). A
// category is a level or a suma, never both.
const (
	MinSuma = 2
	MaxSuma = 18
	// MaxPrizesLength is the longest prizes text a category carries.
	MaxPrizesLength = 200
)

// modalityLabels and levelLabels are the Spanish labels a category's name is
// composed from. The levels are ordinals, from 1ra to 9na.
var (
	modalityLabels = map[string]string{ModalityMen: "Caballeros", ModalityWomen: "Damas", ModalityMixed: "Mixto"}
	levelLabels    = [MaxLevel]string{"1ra", "2da", "3ra", "4ta", "5ta", "6ta", "7ma", "8va", "9na"}
)

// ComposeCategoryName is the name a category is stored and shown under: its
// modality and level ("Caballeros 8va"), or its modality and suma ("Mixto Suma 15").
func ComposeCategoryName(modality string, level, suma int) string {
	label := modalityLabels[modality]
	if suma != 0 {
		return label + " Suma " + strconv.Itoa(suma)
	}
	if level < MinLevel || level > MaxLevel {
		return label
	}
	return label + " " + levelLabels[level-1]
}

// defaultTournamentName is the name a tournament is stored under when its owner
// gives none, so emails and the open-play rows it creates stay readable.
func defaultTournamentName(sport string) string {
	return "Torneo de " + sports.Lowercase(sport)
}

// optionalText is nil for a text left blank, so it is stored as NULL.
func optionalText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
