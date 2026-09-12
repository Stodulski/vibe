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
	return fmt.Sprintf("Reserva canchas en %s. Rápido y seguro.", complexName)
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

// defaultImage is the fallback social preview image when a complex has no logo.
// Absolute, matching the frontend's own og:image: a social unfurler fetches
// this URL on its own and has no page to resolve a relative one against.
const defaultImage = "https://app.vibe.com.ar/logo.png"
