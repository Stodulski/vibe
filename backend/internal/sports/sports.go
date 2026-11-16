// Package sports names the sports the API accepts in the Spanish the public pages
// and the frontend show them.
package sports

import "strings"

// labels mirrors the frontend's es_AR sport labels.
var labels = map[string]string{
	"padel":      "Pádel",
	"tennis":     "Tenis",
	"soccer":     "Fútbol",
	"basketball": "Básquet",
	"volleyball": "Vóley",
	"hockey":     "Hockey",
	"pickleball": "Pickleball",
}

// Label is the Spanish label of a sport. A sport with no label is shown as stored.
func Label(sport string) string {
	if label, ok := labels[sport]; ok {
		return label
	}
	return sport
}

// Lowercase is the label in running text, as in "Torneo de pádel".
func Lowercase(sport string) string {
	return strings.ToLower(Label(sport))
}
