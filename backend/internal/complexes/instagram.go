package complexes

import (
	"regexp"
	"strings"
)

// instagramMessage is the field error for a value that does not reduce to a
// handle. The column's CHECK (complexes_instagram_handle) applies the same
// pattern, so the two agree.
const instagramMessage = "must be an Instagram username or profile link"

var instagramHandleRX = regexp.MustCompile(`^[a-z0-9._]{1,30}$`)

// instagramHosts are the hosts a pasted profile link may carry. Anything else
// is not an Instagram link, and its path is not taken as a handle.
var instagramHosts = []string{"instagram.com/", "www.instagram.com/", "m.instagram.com/"}

// normalizeInstagram reduces what an owner types or pastes to the bare handle
// the column stores: "@Club.Padel", "instagram.com/club.padel" and
// "https://www.instagram.com/club.padel/?hl=es" all become "club.padel".
// Instagram usernames are case-insensitive and shown in lowercase, so the
// handle is lowercased. A blank value is "" with ok true: the caller turns it
// into NULL. ok is false when what is left is not a valid handle.
func normalizeInstagram(raw string) (handle string, ok bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", true
	}
	s = strings.ToLower(s)

	rest, hadScheme := strings.CutPrefix(s, "https://")
	if !hadScheme {
		rest, hadScheme = strings.CutPrefix(s, "http://")
	}
	isLink := false
	for _, host := range instagramHosts {
		if path, found := strings.CutPrefix(rest, host); found {
			s, isLink = path, true
			break
		}
	}
	if hadScheme && !isLink {
		return "", false
	}
	if isLink {
		s, _, _ = strings.Cut(s, "?")
		s, _, _ = strings.Cut(s, "#")
		s = strings.TrimSuffix(s, "/")
	}

	s = strings.TrimPrefix(s, "@")
	if !instagramHandleRX.MatchString(s) {
		return "", false
	}
	return s, true
}
