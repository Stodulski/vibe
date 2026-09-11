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

// pageDescription is the meta description and og:description.
func pageDescription(complexName string) string {
	return fmt.Sprintf("Reserva canchas en %s. Rapido y seguro.", complexName)
}

// The placeholders below are the copy shipped in the frontend's index.html.
// Prerendering works by substituting them, so they must match that file
// exactly; changing the frontend's defaults without changing these silently
// disables prerendering.
const (
	placeholderTitleTag    = `<title>Vibe</title>`
	placeholderDescription = `content="Vibe - Gestion de complejos deportivos, reservas y canchas"`
	placeholderOGTitle     = `content="Vibe - Reserva tu cancha"`
	placeholderOGDesc      = `content="Reserva canchas de padel, tenis y futbol de forma rapida y segura."`
	placeholderImage       = `content="/logo.png"`
)

// defaultImage is the fallback social preview image when a complex has no logo.
const defaultImage = "/logo.png"
