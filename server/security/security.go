// Package security is the only place that decides who a request belongs to
// and whether it is allowed to change anything. Route handlers read the
// answer; they never re-derive it. One implementation means one place to get
// right, and one place to look when auditing.
//
// There are two ways to prove who you are, and they meet here rather than
// anywhere downstream: a session cookie (a browser) or an API key in an
// Authorization header (a script, a cron job, another service). Everything
// past this package sees one Actor and one effective role, so a route does not
// know or care which door the request came through.
package security

import (
	errors "errors"
	strings "strings"
	time "time"

	fiber "github.com/gofiber/fiber/v3"

	apidoc "rook/server/apidoc"
	config "rook/server/config"
	db "rook/server/db"
	encryption "rook/server/encryption"
	logs "rook/server/logs"
	models "rook/server/models"
)

// Typed, unexported keys so nothing else in the process can overwrite what we
// stashed on the request by accident.
type contextKey string

const keyActor contextKey = "current_actor"

// Auth method names, as reported by /api/me. Worth exposing so a client can
// tell a browser session from a key without guessing.
const (
	MethodSession = "session"
	MethodAPIKey  = "api_key"
)

// Actor is who a request belongs to, plus how they proved it.
//
// Role is the *effective* role, not the account's: a user-scoped API key held
// by an admin authenticates as a user. Every permission check reads this
// field, so the narrowing cannot be forgotten at a call site.
type Actor struct {
	User    *models.User
	Session *models.Session
	APIKey  *models.APIKey
	Role    string
}

func ( actor *Actor ) IsAdmin() ( result bool ) {
	result = actor.Role == models.RoleAdmin
	return
}

// ViaAPIKey distinguishes the two doors. It decides the CSRF question below,
// and it is what an endpoint reads if it genuinely must be browser-only.
func ( actor *Actor ) ViaAPIKey() ( result bool ) {
	result = actor.APIKey != nil
	return
}

// KeyID is the id of the API key this request came with, or "" for a browser
// session. A login link or invite link a key makes records it, so revoking the
// key voids them (see models.VoidLinksMintedByKeyTx).
func ( actor *Actor ) KeyID() ( result string ) {
	if actor.APIKey != nil { result = actor.APIKey.ID }
	return
}

func ( actor *Actor ) Method() ( result string ) {
	result = MethodSession
	if actor.ViaAPIKey() {
		result = MethodAPIKey
	}
	return
}

type Guard struct {
	Store  *db.Store
	Config *config.Config
}

func New( store *db.Store , cfg *config.Config ) ( guard *Guard ) {
	guard = &Guard{ Store: store , Config: cfg }
	return
}

// cfg is the configuration the guard reads per request, under the name the
// template's code uses for it.
func ( guard *Guard ) cfg() ( cfg *config.Config ) {
	cfg = guard.Config
	return
}

// SetSessionCookie stores the session credential encrypted with the server
// key. The server already checks the credential against the database, so the
// encryption is not what makes the session unforgeable -- it means the cookie
// is opaque to anyone reading it out of a browser profile or a proxy log, and
// that a tampered cookie fails to decrypt instead of reaching the lookup.
func ( guard *Guard ) SetSessionCookie( c fiber.Ctx , credential string ) ( err error ) {
	sealed , err := encryption.ChaChaEncryptString( encryption.SubKey( guard.cfg().SecretKey , encryption.PurposeCookie ) , credential )
	if err != nil { return }
	c.Cookie( &fiber.Cookie{
		Name:     guard.cfg().SessionCookieName(),
		Value:    sealed,
		Path:     "/",
		MaxAge:   int( guard.Config.SessionTTL.Seconds() ),
		Secure:   guard.Config.SecureCookies,
		HTTPOnly: true,
		SameSite: "Strict",
	} )
	return
}

func ( guard *Guard ) ClearSessionCookie( c fiber.Ctx ) {
	// Overwrite before clearing: some browsers hold on to a cookie whose
	// attributes do not match exactly, and an empty value is harmless either way.
	c.Cookie( &fiber.Cookie{
		Name:     guard.cfg().SessionCookieName(),
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   guard.Config.SecureCookies,
		HTTPOnly: true,
		SameSite: "Strict",
	} )
	c.ClearCookie( guard.cfg().SessionCookieName() )
	// A cookie from before the __Host- prefix goes too.
	if guard.cfg().SecureCookies { c.ClearCookie( guard.cfg().CookieName ) }
}

// DecryptCookie turns the stored cookie back into a session credential.
// Exposed because logging out has to destroy the server-side record, not just
// drop the cookie -- otherwise a copy of the cookie taken earlier still works.
func ( guard *Guard ) DecryptCookie( raw string ) ( credential string , err error ) {
	credential , err = encryption.ChaChaDecryptBase64String( encryption.SubKey( guard.cfg().SecretKey , encryption.PurposeCookie ) , raw )
	return
}

// ---------------------------------------------------------------------------
// Signed-in browsers from before the upgrade. TEMPORARY: remove after
// 2027-02-15 (deploy + auth.session_ttl_seconds, 90 days), when every
// session such a cookie could name has expired anyway.
//
// Until the go-webserver security fixes S12 and S16, the session cookie was
// named server.cookie_name ("session") and sealed with secret.key itself. It
// is now __Host-<cookie_name> while secure_cookies is on, sealed with the
// PurposeCookie subkey. The session records did not change, so a browser
// holding the old cookie still names a live session: SessionCredential
// accepts it, and resolveSession swaps it for the new cookie on that same
// response, so nobody is signed out by the upgrade.
// ---------------------------------------------------------------------------

// ErrNoSessionCookie: the request carries no session cookie that opens.
var ErrNoSessionCookie = errors.New( "security: no session cookie" )

// SessionCredential is the session credential this request's cookie holds:
// the current cookie, or else one from before the upgrade (legacy true).
// Everything that reads the session cookie goes through here.
func ( guard *Guard ) SessionCredential( c fiber.Ctx ) ( credential string , legacy bool , err error ) {
	cfg := guard.cfg()
	if raw := c.Cookies( cfg.SessionCookieName() ); raw != "" {
		if credential , err = guard.DecryptCookie( raw ); err == nil { return }
	}
	raw := c.Cookies( cfg.CookieName )
	if raw == "" {
		credential , err = "" , ErrNoSessionCookie
		return
	}
	credential , err = encryption.ChaChaDecryptBase64String( cfg.SecretKey , raw )
	if err != nil {
		credential , err = "" , ErrNoSessionCookie
		return
	}
	legacy = true
	return
}

// upgradeLegacyCookie replaces a cookie from before the upgrade with the
// current one, for the rest of the session's life, and expires the old one.
func ( guard *Guard ) upgradeLegacyCookie( c fiber.Ctx , credential string , session *models.Session ) {
	cfg := guard.cfg()
	lifetime := time.Until( session.ExpiresAt )
	if lifetime <= 0 { return }
	sealed , err := encryption.ChaChaEncryptString( encryption.SubKey( cfg.SecretKey , encryption.PurposeCookie ) , credential )
	if err != nil { return }
	c.Cookie( &fiber.Cookie{
		Name:     cfg.SessionCookieName(),
		Value:    sealed,
		Path:     "/",
		MaxAge:   int( lifetime.Seconds() ),
		Secure:   cfg.SecureCookies,
		HTTPOnly: true,
		SameSite: "Strict",
	} )
	if cfg.CookieName != cfg.SessionCookieName() {
		c.Cookie( &fiber.Cookie{
			Name:     cfg.CookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			Secure:   cfg.SecureCookies,
			HTTPOnly: true,
			SameSite: "Strict",
		} )
	}
}

// LoadUser runs on every request. It resolves the caller at most once and
// stashes the result, so a handler and a middleware asking "who is this" do
// not cost two database reads. It never rejects anything -- that is
// RequireLogin's job -- because plenty of routes are fine for anonymous
// visitors.
//
// An Authorization: Bearer header decides it: the request is that key or no
// one, and the cookie is not read. Otherwise the cookie. The CSRF rule below
// stays sound: a header is never ambient -- a page on another site cannot add
// one without a CORS preflight this server does not answer -- and a request
// that skips the token check by carrying a key needed that key to begin with.
func ( guard *Guard ) LoadUser( c fiber.Ctx ) ( err error ) {
	var actor *Actor
	// A request that names a key in its Authorization header acts as that key
	// and nothing else, even from a browser that is signed in: a script using
	// a narrow key must not quietly run as the full account whose cookie
	// happens to be in the jar. A bad key is no one, not the cookie.
	if hasBearer( c ) {
		actor = guard.resolveAPIKey( c )
	} else {
		actor = guard.resolveSession( c )
	}
	if actor != nil {
		c.Locals( keyActor , actor )
	}
	err = c.Next()
	return
}

func ( guard *Guard ) resolveSession( c fiber.Ctx ) ( actor *Actor ) {
	credential , legacy , cookie_err := guard.SessionCredential( c )
	if cookie_err != nil { return }

	session , user := models.LoadSession( guard.Store , credential )
	if user == nil { return }
	if legacy { guard.upgradeLegacyCookie( c , credential , session ) }

	// "Last seen" on the devices list. Not for the event streams, which
	// re-check their session every few seconds whether anyone is there.
	if strings.HasSuffix( c.Path() , "/events" ) == false {
		models.TouchSession( guard.Store , session )
	}
	actor = &Actor{ User: user , Session: session , Role: user.Role }
	return
}

// hasBearer is true when the request carries any Authorization header. Any
// at all is an attempt at a key, and one that does not hold a good key is
// nobody (resolveAPIKey): it never falls back to the cookie. Splitting only on
// a space let "Bearer<TAB>key" fall back and run as the signed-in account
// (S16).
func hasBearer( c fiber.Ctx ) ( result bool ) {
	result = strings.TrimSpace( c.Get( fiber.HeaderAuthorization ) ) != ""
	return
}

// resolveAPIKey reads "Authorization: Bearer <id>.<secret>".
//
// One header and one scheme, deliberately: a second accepted spelling (an
// X-API-Key, a query parameter) is a second code path to audit, and a
// credential in a query string ends up in access logs and browser history.
//
// APIKeysEnabled is checked here rather than only on the routes that mint
// keys, so turning the feature off actually closes the door: the header stops
// being believed at all, and existing keys stop authenticating.
func ( guard *Guard ) resolveAPIKey( c fiber.Ctx ) ( actor *Actor ) {
	if guard.Config.APIKeysEnabled == false { return }

	// The scheme and the key, split on any whitespace, and nothing else.
	parts := strings.Fields( c.Get( fiber.HeaderAuthorization ) )
	if len( parts ) != 2 || strings.EqualFold( parts[ 0 ] , "bearer" ) == false { return }
	credential := parts[ 1 ]

	key , user := models.LoadAPIKey( guard.Store , credential )
	if user == nil { return }

	actor = &Actor{
		User:   user,
		APIKey: key,
		// The ceiling, applied per request. See models.NarrowRole: the stored
		// role is what was asked for, this is what is granted.
		Role: models.NarrowRole( user.Role , key.Role ),
	}
	return
}

// RequireLogin, RequireAdmin, and RequireSession are middleware rather than
// helpers called at the top of each handler, so a new protected route is
// protected by where it is registered -- forgetting the check is not something
// you can do silently.
// ViewerMayCall is what a viewer may send besides reads (portal contract v2,
// role viewer): signing out, ending its own sessions, and cancelling its own
// login link. Everything else that is not GET, HEAD or OPTIONS answers 403
// view_only (ViewOnly). In Rook a viewer is a spectator: it may watch any
// table it could see and read the chat, but not sit, bid or play.
// path may be a real path or a route's template; both match.
func ViewerMayCall( method string , path string ) ( ok bool ) {
	switch {
	case method == fiber.MethodGet || method == fiber.MethodHead || method == fiber.MethodOptions:
		ok = true
	case method != fiber.MethodPost:
	case path == "/api/logout" || path == "/api/account/login-link/cancel":
		ok = true
	case path == "/api/account/sessions/revoke-others":
		ok = true
	// A key rotating its own secret takes the old one away: a viewer key
	// keeps itself current like any other (portal contract v3).
	case path == "/api/keys/self/rotate":
		ok = true
	case strings.HasPrefix( path , "/api/account/sessions/" ) && strings.HasSuffix( path , "/revoke" ):
		ok = true
	}
	return
}

// signingIn is true for Rook's public sign-in routes: redeeming a login
// link, signing up from a table's seat, and the sign-up link pages. A
// viewer's browser posting to one is not acting as the viewer, it is becoming
// someone (possibly someone else), so the view-only rule does not apply.
// POST /api/join/{code} alone is not one: that is taking a seat. The public
// watch chat is let through too, since anyone may post there signed out.
func signingIn( path string ) ( result bool ) {
	switch {
	case path == "/api/login" || strings.HasPrefix( path , "/api/login/" ):
		result = true
	case strings.HasPrefix( path , "/api/signup/" ):
		result = true
	case strings.HasPrefix( path , "/api/join/" ) && strings.HasSuffix( path , "/signup" ):
		result = true
	case strings.HasPrefix( path , "/api/watch/" ) && strings.HasSuffix( path , "/chat" ):
		result = true
	}
	return
}

// ViewOnly is the viewer role's one guard, in front of every route: a
// viewer's session or key may read, and may send only what ViewerMayCall
// lets through. Anything else answers 403 {"error":"view_only","code":"view_only"}, whatever
// the route, so a route added later is covered without remembering to.
func ( guard *Guard ) ViewOnly( c fiber.Ctx ) ( err error ) {
	actor := ActorFrom( c )
	path := c.Path()
	if actor == nil || actor.Role != models.RoleViewer || signingIn( path ) || ViewerMayCall( c.Method() , path ) {
		err = c.Next()
		return
	}
	err = c.Status( fiber.StatusForbidden ).JSON( apidoc.ErrorBody( fiber.StatusForbidden , "view_only" , apidoc.CodeViewOnly ) )
	return
}

func ( guard *Guard ) RequireLogin( c fiber.Ctx ) ( err error ) {
	if ActorFrom( c ) == nil {
		err = c.Status( fiber.StatusUnauthorized ).JSON( apidoc.ErrorBody( fiber.StatusUnauthorized , "not authenticated" , apidoc.CodeNotAuthenticated ) )
		return
	}
	err = c.Next()
	return
}

func ( guard *Guard ) RequireAdmin( c fiber.Ctx ) ( err error ) {
	actor := ActorFrom( c )
	if actor == nil {
		err = c.Status( fiber.StatusUnauthorized ).JSON( apidoc.ErrorBody( fiber.StatusUnauthorized , "not authenticated" , apidoc.CodeNotAuthenticated ) )
		return
	}
	if actor.IsAdmin() == false {
		err = c.Status( fiber.StatusForbidden ).JSON( apidoc.ErrorBody( fiber.StatusForbidden , "forbidden" , apidoc.CodeNotAdmin ) )
		return
	}
	err = c.Next()
	return
}

// RequireSession refuses a request that authenticated with an API key, even a
// correctly scoped one. It guards minting and revoking API keys, a seat
// invite, sign-up links and deleting your own account (see routes.go).
//
// The reasoning is containment rather than privilege. A key is a bearer
// credential that lives in a config file or a CI secret; if a leaked key could
// create another credential -- a key directly, or a login link that becomes a
// session that mints a key -- revoking it would not end the compromise,
// because what it created while you were not looking survives. Requiring a
// browser session for all of these means every credential traces back to a
// person signing in, and revoking a key actually revokes it.
//
// Accounts and ways in are not on that list. An admin key may create and
// delete accounts, change roles and expiry, and hand out login links and
// invite links. A key makes its own account a login link only once that
// account switched it on for the key, from a session (models.APIKey.MaySignIn),
// and then one that works once, for at most 15 minutes.
//
// A key may make keys only one way, with lineage. The owner switches
// MayMintKeys on, from a session, on one of the owner's own admin-role keys;
// that key may then make keys for other accounts
// (POST /api/admin/users/{user_id}/keys), never the owner's, each capped at
// its account's role and recording the key that made it
// (models.APIKey.MintedByKey). A minted key never gets MayMintKeys or
// MaySignIn, so it cannot mint or turn into a session in turn. Every other
// door to minting stays browser-only.
//
// What keeps revoking a leaked key meaningful is that everything it made
// records it. Revoking or deleting the key voids its login links still
// usable, ends every session one of them signed in, switches off its invite
// links, and revokes every key it minted, down the whole chain
// (models.VoidLinksMintedByKeyTx). Disabling or demoting the key's account
// does the same for all its keys. What it does not undo is anything those
// did meanwhile, or the accounts made through an invite link it handed out:
// those show in the activity log and the user list.
func ( guard *Guard ) RequireSession( c fiber.Ctx ) ( err error ) {
	actor := ActorFrom( c )
	if actor == nil {
		err = c.Status( fiber.StatusUnauthorized ).JSON( apidoc.ErrorBody( fiber.StatusUnauthorized , "not authenticated" , apidoc.CodeNotAuthenticated ) )
		return
	}
	if actor.ViaAPIKey() {
		err = c.Status( fiber.StatusForbidden ).JSON( apidoc.ErrorBody( fiber.StatusForbidden , "this endpoint requires a signed-in session, not an API key" , apidoc.CodeSessionOnly ) )
		return
	}
	err = c.Next()
	return
}

// StillOwner is true while credential is a live session of the owner -- the
// account the server was set up with -- acting as an admin. The activity
// log's stream asks it every few seconds, so signing out or losing admin ends
// a Logs screen left open.
func ( guard *Guard ) StillOwner( credential string ) ( ok bool ) {
	_ , user := models.LoadSession( guard.Store , credential )
	if user == nil || user.IsAdmin() == false { return }
	owner_id , err := models.SuperAdminID( guard.Store )
	ok = err == nil && owner_id != 0 && user.ID == owner_id
	return
}

// LogActor is who the activity log says made a request: the account, how it
// signed in, and the API key if that was the way in. Anonymous when nobody.
func LogActor( actor *Actor ) ( result *logs.Actor ) {
	if actor == nil || actor.User == nil {
		result = logs.Anonymous()
		return
	}
	result = &logs.Actor{
		UserID: actor.User.ID,
		Name:   actor.User.DisplayName,
		Role:   actor.Role,
		Method: actor.Method(),
	}
	if actor.APIKey != nil {
		result.KeyID = actor.APIKey.ID
		result.KeyName = actor.APIKey.Name
	}
	return
}

func ActorFrom( c fiber.Ctx ) ( actor *Actor ) {
	actor , _ = c.Locals( keyActor ).( *Actor )
	return
}

// UserFrom, SessionFrom, and APIKeyFrom are the narrow reads a handler
// usually wants. UserFrom is the account; ActorFrom is the account plus how
// it authenticated and what it may do, which is what a permission decision
// needs.
func UserFrom( c fiber.Ctx ) ( user *models.User ) {
	if actor := ActorFrom( c ); actor != nil {
		user = actor.User
	}
	return
}

func SessionFrom( c fiber.Ctx ) ( session *models.Session ) {
	if actor := ActorFrom( c ); actor != nil {
		session = actor.Session
	}
	return
}

func APIKeyFrom( c fiber.Ctx ) ( key *models.APIKey ) {
	if actor := ActorFrom( c ); actor != nil {
		key = actor.APIKey
	}
	return
}

// CheckCSRF guards state-changing requests. SameSite=Strict already blocks
// the common cross-site POST, but it is a browser-side promise: older
// browsers, odd cross-scheme cases, and non-browser clients do not all honour
// it. A per-session token echoed back in the JSON body costs nothing and does
// not depend on the browser behaving.
//
// A request authenticated by an API key is exempt, and that is not a hole.
// CSRF exists because a cookie is *ambient* -- the browser attaches it to a
// request some other site caused. Nothing attaches an Authorization header on
// its own; a forged cross-site request from an attacker's page carries no key,
// so there is nothing to forge with. Requiring a token here would instead mean
// every script had to fetch one first, for no gain.
func ( guard *Guard ) CheckCSRF( c fiber.Ctx , submitted string ) ( ok bool ) {
	actor := ActorFrom( c )
	if actor == nil { return }
	if actor.ViaAPIKey() {
		ok = true
		return
	}
	if actor.Session == nil || submitted == "" { return }
	ok = encryption.ConstantTimeEqual( actor.Session.CSRFToken , submitted )
	return
}

// SameOrigin guards the handful of POSTs anyone may make without being signed
// in -- signing up through a join or sign-up link. There is no session yet,
// so there is no CSRF token to check; instead the request must come from a
// page on this site. Browsers say where a request came from in
// Sec-Fetch-Site (or, failing that, Origin), and neither can be set by a
// page on another site. A request with neither header is not a browser and
// carries no ambient credential to abuse, so it is allowed.
func SameOrigin( c fiber.Ctx ) ( ok bool ) {
	if site := c.Get( "Sec-Fetch-Site" ); site != "" {
		ok = site == "same-origin"
		return
	}
	origin := c.Get( fiber.HeaderOrigin )
	if origin == "" {
		ok = true
		return
	}
	ok = origin == c.Scheme()+"://"+c.Host()
	return
}

// RequireAPIKey is RequireSession's mirror: it refuses a browser session and
// lets only a key through. It guards /mcp, which is for agents and scripts.
// Taking only a header credential there means no ambient cookie can ride
// along on a request some other page caused, so the endpoint needs no CSRF
// token and the no-credentials rule for keys covers everything it can do.
func ( guard *Guard ) RequireAPIKey( c fiber.Ctx ) ( err error ) {
	actor := ActorFrom( c )
	if actor == nil {
		c.Set( fiber.HeaderWWWAuthenticate , "Bearer" )
		err = c.Status( fiber.StatusUnauthorized ).JSON( apidoc.ErrorBody( fiber.StatusUnauthorized , "not authenticated" , apidoc.CodeNotAuthenticated ) )
		return
	}
	if actor.ViaAPIKey() == false {
		err = c.Status( fiber.StatusForbidden ).JSON( apidoc.ErrorBody( fiber.StatusForbidden , "this endpoint requires an API key, not a signed-in session" , apidoc.CodeForbidden ) )
		return
	}
	err = c.Next()
	return
}

// RequireOwner lets through only the owner, signed in from a browser. It
// guards the activity log (routes/logs.go), which shows what every account
// did -- including the other admins, which is exactly why they cannot read
// it. Everyone else, any API key included, gets the 404 an unknown route
// gets: the Logs screen is not something to advertise.
//
// A session, not a key: the log holds every account's IP addresses and
// activity, and a key is a credential that lives in files. An app that wants
// the log readable by a script should add a narrow export, knowingly.
func ( guard *Guard ) RequireOwner( c fiber.Ctx ) ( err error ) {
	actor := ActorFrom( c )
	allowed := actor != nil && actor.ViaAPIKey() == false && actor.IsAdmin()
	if allowed {
		owner_id , owner_err := models.SuperAdminID( guard.Store )
		allowed = owner_err == nil && owner_id != 0 && actor.User.ID == owner_id
	}
	if allowed == false {
		err = c.Status( fiber.StatusNotFound ).JSON( apidoc.ErrorBody( fiber.StatusNotFound , "not_found" , apidoc.CodeNotFound ) )
		return
	}
	err = c.Next()
	return
}
