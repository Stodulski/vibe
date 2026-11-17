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

// masculineSports are the sports whose modalities read Masculino and Femenino; the
// others read Caballeros and Damas.
var masculineSports = map[string]bool{"soccer": true, "basketball": true, "volleyball": true, "hockey": true}

// ModalityLabel is the Spanish label of a modality (men, women or mixed) in the words
// its sport uses: "Caballeros" or "Masculino" for men, "Damas" or "Femenino" for
// women, "Mixto" for mixed. A modality it does not know is shown as stored.
func ModalityLabel(sport, modality string) string {
	masculine := masculineSports[sport]
	switch modality {
	case "men":
		if masculine {
			return "Masculino"
		}
		return "Caballeros"
	case "women":
		if masculine {
			return "Femenino"
		}
		return "Damas"
	case "mixed":
		return "Mixto"
	}
	return modality
}
