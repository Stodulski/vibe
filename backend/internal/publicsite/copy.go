package publicsite

import "fmt"

// The strings below are user-facing copy, shown in search results and social
// previews. They are Spanish because that is the only locale the product
// currently ships.
//
// They are gathered here rather than inlined at their use sites so that moving
// them behind the backend i18n layer, when it exists, is one edit in one file
// rather than a hunt through the renderer. Nothing else in this package should
// contain a user-visible string.

// pageTitle is the <title> and og:title for a complex's public page.
func pageTitle(complexName string) string {
	return complexName + " - Reserva tu cancha"
}

// pageDescription is the meta description and og:description. The city is what
// tells one venue's snippet from another's; without it every complex would
// carry the same sentence. Both arguments must already be HTML-escaped.
func pageDescription(complexName, city string) string {
	if city == "" {
		return fmt.Sprintf("Reserva canchas en %s. Horarios y reservas online.", complexName)
	}
	return fmt.Sprintf("Reserva canchas en %s, %s. Horarios y reservas online.", complexName, city)
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
	servicesHeading = "Servicios"
	phoneLabel      = "Teléfono:"
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
