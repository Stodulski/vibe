package whatsapp

import "testing"

// An Argentine mobile stored without its 9 is perfectly valid E.164 —
// internal/validator accepts it — and unroutable on WhatsApp. Nothing between
// the client typing it and Meta receiving it put the 9 back, so those clients
// got a booking, an email, and silence.
func TestRecipientID(t *testing.T) {
	for _, c := range []struct {
		name  string
		phone string
		want  string
	}{
		{"argentine mobile already carrying its 9", "+5491155551234", "+5491155551234"},
		{"argentine mobile missing its 9", "+541155551234", "+5491155551234"},
		{"spaces and dashes as a person types them", "+54 9 11 5555-1234", "+5491155551234"},
		{"argentine mobile missing its 9, spaced", "+54 11 5555 1234", "+5491155551234"},
		{"four-digit area code", "+542944123456", "+5492944123456"},
		{"brazil keeps its own nine", "+5511999999999", "+5511999999999"},
		{"united states untouched", "+14155552671", "+14155552671"},
		{"spain untouched", "+34911223344", "+34911223344"},
		{"argentine landline length is not a mobile", "+5454321", "+5454321"},
		{"empty stays empty", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := RecipientID(c.phone); got != c.want {
				t.Errorf("RecipientID(%q) = %q; want %q", c.phone, got, c.want)
			}
		})
	}
}

// Meta refuses the whole message when a text parameter carries a newline, a
// tab, or more than four consecutive spaces. Three of the values this package
// sends are typed by a complex owner, and nothing in the complex or court
// handlers forbids either.
func TestSanitizeParam(t *testing.T) {
	for _, c := range []struct {
		name string
		in   string
		want string
	}{
		{"newline", "Vibe\nPalermo", "Vibe Palermo"},
		{"carriage return and newline", "Vibe\r\nPalermo", "Vibe Palermo"},
		{"tab", "Vibe\tPalermo", "Vibe Palermo"},
		{"four consecutive spaces", "Vibe    Palermo", "Vibe Palermo"},
		{"many consecutive spaces", "Vibe          Palermo", "Vibe Palermo"},
		{"leading and trailing whitespace", "  Vibe Palermo\n", "Vibe Palermo"},
		{"ordinary text is untouched", "Cancha 1 - techada", "Cancha 1 - techada"},
		{"a maps query is untouched", "-34.603722,-58.381592", "-34.603722,-58.381592"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := sanitizeParam(c.in); got != c.want {
				t.Errorf("sanitizeParam(%q) = %q; want %q", c.in, got, c.want)
			}
		})
	}
}
