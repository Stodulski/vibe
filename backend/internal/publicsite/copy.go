package publicsite

import (
	"fmt"
	"strings"
)

// The strings below are user-facing copy, shown in search results and social
// previews. They are Spanish because that is the only locale the product
// currently ships.
//
// They are gathered here rather than inlined at their use sites so that moving
// them behind the backend i18n layer, when it exists, is one edit in one file
// rather than a hunt through the renderer. Nothing else in this package should
// contain a user-visible string.

// maxDescriptionServices caps how many amenities the description names, so the
// snippet stays inside the width Google shows.
const maxDescriptionServices = 3

// maxDescriptionSports caps how many sports the description names, for the same
// reason as maxDescriptionServices.
const maxDescriptionSports = 3

// pageTitle is the <title> and og:title for a complex's public page. The city
// places the venue for someone searching their area. Both arguments must already
// be HTML-escaped.
func pageTitle(complexName, city string) string {
	if city == "" {
		return complexName + " - Reservá tu cancha"
	}
	return complexName + " en " + city + " - Reservá tu cancha"
}

// pageDescription is the meta description and og:description: how to book, where,
// what the venue plays and what it offers. It says "online" only when the complex
// takes online payments, and "por WhatsApp" otherwise, which is how the public
// page itself describes the two cases. sports are the Spanish labels of the
// active courts' sports, deduplicated by the caller; they lead the sentence as
// "canchas de pádel y tenis" and are lowercased here, because mid-sentence they
// are common nouns. services are the labels of the complex's amenities. The name
// and city must already be HTML-escaped; the sport and service labels are fixed.
func pageDescription(complexName, city string, online bool, sports, services []string) string {
	place := complexName
	if city != "" {
		place += ", " + city
	}
	channel := " por WhatsApp"
	if online {
		channel = " online"
	}
	subject := "Reservá tu cancha en "
	if len(sports) > 0 {
		if len(sports) > maxDescriptionSports {
			sports = sports[:maxDescriptionSports]
		}
		lowered := make([]string, len(sports))
		for i, sport := range sports {
			lowered[i] = strings.ToLower(sport)
		}
		subject = "Reservá canchas de " + joinSpanish(lowered) + " en "
	}
	detail := "Horarios y ubicación."
	if len(services) > 0 {
		if len(services) > maxDescriptionServices {
			services = services[:maxDescriptionServices]
		}
		detail = strings.Join(services, ", ") + "."
	}
	return fmt.Sprintf("%s%s%s. %s", subject, place, channel, detail)
}

// joinSpanish joins items the way Spanish writes a list: "a", "a y b", "a, b y c".
func joinSpanish(items []string) string {
	if len(items) <= 1 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " y " + items[len(items)-1]
}

// The placeholders below are the copy shipped in the frontend's index.html.
// Prerendering works by substituting them, so they must match that file
// byte for byte; a placeholder that does not match is not an error anywhere —
// the substitution simply finds nothing and the tag keeps Vibe's generic copy,
// so the page is served with a 200 and looks fine while carrying none of this
// complex's description or image.
//
// Three of the five were exactly that: the accents were missing from
// "Gestión", "pádel", "fútbol" and "rápida", and the image was the relative
// "/logo.png" where index.html ships an absolute URL. They are reproduced here
// from frontend/index.html, and publicsite_test.go asserts them against a
// literal copy of that file's tags rather than against these constants, so the
// next drift fails a test instead of quietly turning prerendering off.
const (
	placeholderTitleTag    = `<title>Vibe</title>`
	placeholderDescription = `content="Vibe - Gestión de complejos deportivos, reservas y canchas"`
	placeholderOGTitle     = `content="Vibe - Reserva tu cancha"`
	placeholderOGDesc      = `content="Reserva canchas de pádel, tenis y fútbol de forma rápida y segura."`
	placeholderImage       = `content="https://app.vibe.com.ar/logo.png"`
)

// defaultImage is the fallback social preview image when a complex has neither
// a cover photo nor a logo. Absolute, matching the frontend's own og:image: a
// social unfurler fetches this URL on its own and has no page to resolve a
// relative one against.
const defaultImage = "https://app.vibe.com.ar/logo.png"

// The body of the prerendered page repeats what the public page shows, with the
// same words, so the day names, the closed label, the headings and the amenity
// labels below mirror the frontend's es_AR copy.
const (
	closedLabel     = "Cerrado"
	hoursHeading    = "Horarios"
	courtsHeading   = "Canchas"
	servicesHeading = "Servicios"
)

// dayLabels names the stored days, in the Spanish the public page uses.
var dayLabels = map[string]string{
	"monday": "Lunes", "tuesday": "Martes", "wednesday": "Miércoles", "thursday": "Jueves",
	"friday": "Viernes", "saturday": "Sábado", "sunday": "Domingo",
}

// amenityLabels maps the stored amenity keys (the complexes_amenities_known
// vocabulary) to the labels the public page shows.
var amenityLabels = map[string]string{
	"parking": "Estacionamiento", "changing_rooms": "Vestuarios", "showers": "Duchas",
	"bar": "Bar", "racket_rental": "Alquiler de paletas", "pro_shop": "Venta de pelotas",
	"wifi": "Wi-Fi", "lockers": "Lockers", "lessons": "Clases", "tournaments": "Torneos",
	"accessible": "Accesible", "match_recording": "Grabación",
}

// sportLabels and courtTypeLabels name a court's stored sport and court type in
// the Spanish the public page and the owner's court form use (frontend
// es_AR courts.ts: sportTypes and courtTypes). Change them there and here
// together, because the same words appear on both sides.
var sportLabels = map[string]string{
	"padel": "Pádel", "tennis": "Tenis", "soccer": "Fútbol", "basketball": "Básquet",
	"volleyball": "Vóley", "hockey": "Hockey", "pickleball": "Pickleball",
}

var courtTypeLabels = map[string]string{
	"indoor": "Techada", "outdoor": "Descubierta", "semi_covered": "Semi cubierta",
}
