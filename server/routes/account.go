package routes

import (
	fiber "github.com/gofiber/fiber/v3"

	apidoc "rook/server/apidoc"

	logs "rook/server/logs"
	models "rook/server/models"
	security "rook/server/security"
	tables "rook/server/tables"
)

type renameRequest struct {
	DisplayName string `json:"display_name,omitempty"`
	CSRFToken   string `json:"csrf_token,omitempty"`
}

// GetMe is the frontend's first call on every page load: it answers "who am
// I, what may I do, and what CSRF token do I echo back". Handing the token
// out here means no page needs a hidden form field or a template variable.
//
// "role" is the *effective* role, not the account's: a request made with a
// user-scoped API key belonging to an admin reports "user", because that is
// what it may actually do. account_role reports the account itself, for the
// rare caller that needs to tell the two apart. Reporting the account's role
// as "role" would have a client believe it has permissions the server will
// refuse to act on.
//
// @route("GET /api/me", summary="Get me", tag="Account",
// )
func ( handlers *Handlers ) GetMe( c fiber.Ctx , in *apidoc.None ) ( err error ) {
	actor := security.ActorFrom( c )
	user := actor.User

	payload := fiber.Map{
		"authenticated":    true,
		"id":               user.ID,
		"display_name":     user.DisplayName,
		"role":             actor.Role,
		"account_role":     user.Role,
		"auth":             actor.Method(),
		"api_keys_enabled": handlers.Config.APIKeysEnabled,
		// Where the API keys card links: the API reference as JSON and as
		// tables for people (openapi.go, api_docs_md.go).
		"openapi_url":      handlers.openAPIURL(),
		"api_doc_url":      handlers.apiDocURL(),
		// The running build ("dev" when none was stamped). fresh.js compares
		// it with X-App-Version to notice a deploy; live.js reloads at once
		// on a dev build.
		"version":          handlers.Version,
		"can_invite":       user.MayInvite(),
		"can_hint":         user.CanHint,
		"icon":             user.Icon,
		"color":            user.Color,
		"has_pin":          handlers.Config.SigninPinEnabled && user.HasPin(),
		"signin_pin_enabled": handlers.Config.SigninPinEnabled,
		// owner: the account this server was set up with (see
		// models/superadmin.go), and only when acting with admin rights, so
		// a user-scoped key of the owner's is not the owner.
		"owner":            actor.IsAdmin() && models.IsSuperAdmin( handlers.Store , user.ID ),
		"can_undo":         user.CanUndo,
		"can_reshuffle":    user.CanReshuffle,
		"bot_level":        botLevelOf( user ),
		"auto_play_level":  string( tables.AutoPlayLevelOf( user ) ),
		"hint_level":       string( tables.HintLevelOf( user ) ),
		// Where the table's two display switches start on a device where
		// the player has not flipped them (see models.User).
		"player_scores":    user.PlayerScores != nil && *user.PlayerScores,
		"led_glow":         user.LedGlow != nil && *user.LedGlow,
		"win_glow":         user.WinGlow == nil || *user.WinGlow,
		"partner_bot_outbids": user.PartnerBotOutbids,
		// The Appearance screen's themes (see themes.go): your own list, else
		// the app's defaults, else null for the built-in ones.
		"theme_id":         user.ThemeID,
		// The app-wide pause after a trick, for the New table form's
		// "Default (x s)" choice.
		"trick_pause_default": handlers.Hub.Settings().TrickPause(),
	}
	themes , themes_custom , themes_builtins , themes_err := handlers.myThemes( user )
	if themes_err != nil {
		err = serverError( c , themes_err )
		return
	}
	payload[ "themes" ] = themes
	payload[ "themes_custom" ] = themes_custom
	payload[ "themes_builtins" ] = themes_builtins
	// Only a session has a CSRF token. A key-authenticated request has no
	// ambient credential to protect, so there is nothing to hand back -- see
	// Guard.CheckCSRF.
	if actor.Session != nil {
		payload[ "csrf_token" ] = actor.Session.CSRFToken
	}
	if actor.APIKey != nil {
		payload[ "api_key_id" ] = actor.APIKey.ID
		payload[ "api_key_name" ] = actor.APIKey.Name
	}
	err = c.JSON( payload )
	return
}

// @route("POST /api/account/rename", summary="Rename account", tag="Account",
// )
func ( handlers *Handlers ) RenameAccount( c fiber.Ctx , in *renameAccountIn ) ( err error ) {
	body := in.renameRequest
	if models.ValidDisplayName( body.DisplayName ) == false {
		err = badRequest( c , "display name must be 1-80 characters" )
		return
	}

	user := security.UserFrom( c )
	if models.RenameUser( handlers.Store , user.ID , body.DisplayName ) != nil {
		err = serverError( c )
		return
	}
	handlers.refreshTables( user.ID )
	err = c.JSON( fiber.Map{ "ok": true , "display_name": body.DisplayName } )
	return
}

// Logout deletes the session server-side as well as clearing the cookie, so
// a copy of the cookie captured earlier is dead too.
//
// It is held to the CSRF token like every other state change. SameSite=Strict
// already stops another site from sending the cookie here, but not a page on
// a sibling subdomain, and being signed out by someone else's page is the
// first step of tricking a person into signing in to the wrong account.
//
// Called with an API key it does nothing and says so by succeeding: there is
// no session to destroy. A key is ended by revoking it, not by logging out.
//
// @route("POST /api/logout", summary="Logout", tag="Account",
// )
func ( handlers *Handlers ) Logout( c fiber.Ctx , in *apidoc.None ) ( err error ) {
	handlers.audit( c , logs.KindAuth , "signed out" , nil )
	handlers.destroyCookieSession( c )
	handlers.Guard.ClearSessionCookie( c )
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}

// CreateMyLoginLink is ReissueLogin for yourself: a one-time link that signs
// another device in as you, so a player can move to their phone without
// asking an admin. Making one retires any link the account had not used.
//
// A browser session may always ask. An API key may only when its account
// turned that on for it, from a session (POST /api/keys/{key_id}/sign-in):
// the link becomes an ordinary browser session, which opens everything kept
// from keys (the API keys card, minting keys) to whoever holds the key. A
// key's link works once and for at most 15 minutes, whatever it asked for; it
// records the key, and revoking or deleting the key voids it while it is
// unused and signs out the browser it signed in.
//
// @route("POST /api/account/login-link", summary="Create my login link", tag="Account",
//     description="A one-time link that signs your own account in on a browser or another device. "
//         "An API key may ask only if your account let it, from a browser session "
//         "(POST /api/keys/{key_id}/sign-in); its link works for at most 15 minutes. "
//         "Answers with login_path.",
// )
func ( handlers *Handlers ) CreateMyLoginLink( c fiber.Ctx , in *createMyLoginLinkIn ) ( err error ) {
	actor := security.ActorFrom( c )
	ceiling := handlers.Config.LoginTokenTTL
	if actor.ViaAPIKey() {
		if actor.APIKey.MaySignIn == false {
			err = forbidden( c , "this key may not make sign-in links: turn that on for it on the API keys card" )
			return
		}
		ceiling = keyLinkCeiling( ceiling )
		// A key's link works once, whatever it asked for.
		in.MaxUses = nil
	}
	ttl , ok , err := loginLinkTTL( c , in.loginLinkRequest , ceiling )
	if ok == false { return }
	credential , token_err := models.IssueLoginTokenByKey( handlers.Store , actor.User.ID , ttl , actor.KeyID() )
	if token_err != nil {
		err = serverError( c )
		return
	}
	handlers.audit( c , logs.KindAuth , "own login link created" , fiber.Map{ "expires_in_seconds": int( ttl.Seconds() ) } )
	err = c.JSON( loginLinkReply( credential , ttl ) )
	return
}

// DeleteMyAccount is DeleteUser for yourself: the account, every way to
// sign in as it, and its seats at open tables, gone for good. The same two
// accounts are protected as from an admin: the owner, and the last admin
// standing. Session only, like CreateMyLoginLink -- an API key must not be
// able to delete the account it belongs to.
//
// @route("POST /api/account/delete", summary="Delete my account", tag="Account", access=SESSION_ONLY,
// )
func ( handlers *Handlers ) DeleteMyAccount( c fiber.Ctx , in *apidoc.None ) ( err error ) {
	user := security.UserFrom( c )
	if models.IsSuperAdmin( handlers.Store , user.ID ) {
		err = badRequest( c , "superadmin" )
		return
	}
	if user.IsAdmin() && user.Disabled() == false {
		admins , count_err := models.CountActiveAdmins( handlers.Store )
		if count_err != nil {
			err = serverError( c )
			return
		}
		if admins <= 1 {
			err = badRequest( c , "last_admin" )
			return
		}
	}
	if models.DeleteUser( handlers.Store , user.ID ) != nil {
		err = serverError( c )
		return
	}
	handlers.Hub.ForgetUser( user.ID )
	handlers.Guard.ClearSessionCookie( c )
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}
