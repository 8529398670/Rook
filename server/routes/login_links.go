package routes

import (
	time "time"

	fiber "github.com/gofiber/fiber/v3"

	models "rook/server/models"
)

// loginLinkRequest is what a caller may say about a login link it asks for.
// Both fields are optional. A link here always works once, so max_uses is
// only there for callers written against apps whose links can work more than
// once (the project portal sends { "max_uses": 1, "expires_in_minutes": 15 }).
type loginLinkRequest struct {
	MaxUses          *int `json:"max_uses,omitempty"`
	ExpiresInMinutes *int `json:"expires_in_minutes,omitempty"`
}

// loginLinkMaxMinutes bounds expires_in_minutes as a number; the link's real
// ceiling is auth.login_token_ttl_seconds, or KeyLoginLinkTTL for a key's own.
const loginLinkMaxMinutes = 525600

// loginLinkTTL reads a loginLinkRequest into how long the link works, at most
// ceiling. Nothing asked means ceiling. ok is false, with the reply already
// sent, for a request asking for a link that works more than once or for no
// time at all.
func loginLinkTTL( c fiber.Ctx , in loginLinkRequest , ceiling time.Duration ) ( ttl time.Duration , ok bool , err error ) {
	if in.MaxUses != nil && *in.MaxUses != 1 {
		err = badRequest( c , "max_uses must be 1: a login link here works once" )
		return
	}
	ttl = ceiling
	if in.ExpiresInMinutes != nil {
		minutes := *in.ExpiresInMinutes
		if minutes < 1 || minutes > loginLinkMaxMinutes {
			err = badRequest( c , "expires_in_minutes must be 1-525600" )
			return
		}
		if asked := time.Duration( minutes ) * time.Minute; asked < ttl { ttl = asked }
	}
	ok = true
	return
}

// loginLinkReply is every route's answer with a login link in it. The link is
// a path, not a full URL: the browser knows the origin it is talking to,
// whereas the server behind a reverse proxy often does not.
func loginLinkReply( credential string , ttl time.Duration ) ( reply fiber.Map ) {
	reply = fiber.Map{
		"login_path":         "/login/" + credential,
		"expires_in_seconds": int( ttl.Seconds() ),
		"expires_at":         time.Now().UTC().Add( ttl ),
		"max_uses":           1,
	}
	return
}

// keyLinkCeiling is the longest a key's link for its own account may work:
// KeyLoginLinkTTL, or less when the server's own login links are shorter.
func keyLinkCeiling( configured time.Duration ) ( ceiling time.Duration ) {
	ceiling = models.KeyLoginLinkTTL
	if configured < ceiling { ceiling = configured }
	return
}
