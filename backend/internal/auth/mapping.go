package auth

import (
	authstore "github.com/stodulski/vibe-server/internal/auth/store"
	"github.com/stodulski/vibe-server/internal/openapi/gen"
)

// toGenUser maps a store user onto the generated wire type (HTTP-08): a
// handler must never serialize authstore.User directly, because that would
// leak PasswordHash's json:"-" bypass, FailedLoginAttempts and the lockout
// timestamps the moment anyone forgets to keep those tags in sync by hand.
// Every field on the wire today is reproduced here by name. Email needs no
// conversion: the OpenAPI document declares it a plain string (format: email
// was dropped, since it made oapi-codegen emit openapi_types.Email, which
// fails to marshal any stored value that is not a valid net/mail address).
func toGenUser(u *authstore.User) gen.User {
	return gen.User{
		Id:            u.ID,
		Email:         u.Email,
		FirstName:     u.FirstName,
		LastName:      u.LastName,
		Phone:         u.Phone,
		Role:          gen.UserRole(u.Role),
		IsActive:      u.IsActive,
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
}
