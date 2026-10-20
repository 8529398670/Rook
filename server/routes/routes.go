// Package routes contains one file per group of endpoints, plus this file,
// which is the only place that knows the full URL map.
//
// The API here stays deliberately small: it covers logging in, managing who
// can log in, issuing the API keys that let non-browser callers in, and
// reading the UI's text. Everything else an app does belongs in a new file in
// this package (server side) and in static/js/ (browser side) -- see
// references/architecture.md for where the line sits and why.
package routes

import (
	fiber "github.com/gofiber/fiber/v3"

	config "rook/server/config"
	db "rook/server/db"
	language "rook/server/language"
	security "rook/server/security"
	static "rook/server/static"
	tables "rook/server/tables"
)

// Handlers carries the dependencies every route group needs. Passing this in
// rather than reaching for package-level globals is what makes the handlers
// testable and keeps the wiring visible in main.go.
type Handlers struct {
	Store    *db.Store
	Config   *config.Config
	Guard    *security.Guard
	Language *language.Language
	Static   *static.Server
	Hub      *tables.Hub
}

func New( store *db.Store , cfg *config.Config , guard *security.Guard , lang *language.Language , static_server *static.Server , hub *tables.Hub ) ( handlers *Handlers ) {
	handlers = &Handlers{
		Store:    store,
		Config:   cfg,
		Guard:    guard,
		Language: lang,
		Static:   static_server,
		Hub:      hub,
	}
	return
}

// Register wires every route. Read it top to bottom to see the whole surface
// of this server.
func ( handlers *Handlers ) Register( app *fiber.App ) {
	// Resolve the session once, before anything else looks at the request.
	app.Use( handlers.Guard.LoadUser )

	// Public: the login link's page and the call it makes, and the UI's text
	// (needed to render the signed-out page at all). Opening a login link
	// only shows a page; the link is spent by the POST that page sends when
	// the person confirms. See auth.go.
	app.Get( "/login/*" , handlers.ShowLoginPage )
	app.Post( "/api/login" , handlers.RedeemLoginLink )
	app.Get( "/api/language" , handlers.GetLanguage )
	app.Get( "/api/health" , handlers.GetHealth )

	// Public game links. These must be registered before the "/api" group
	// below, whose RequireLogin is mounted on the whole prefix.
	//   - watch links: anyone with the link may watch, signed in or not;
	//   - a join link's landing info, and its sign-up (only while an admin
	//     allows it);
	//   - multi-use sign-up links, where the credential is in the body.
	app.Get( "/api/watch/:code" , handlers.GetWatch )
	app.Get( "/api/watch/:code/events" , handlers.WatchEvents )
	app.Get( "/api/watch/:code/record" , handlers.GetWatchRecord )
	app.Post( "/api/watch/:code/chat" , handlers.WatchChat )
	app.Get( "/api/join/:code" , handlers.GetJoin )
	app.Post( "/api/join/:code/signup" , handlers.JoinSignup )
	app.Post( "/api/signup/info" , handlers.SignupInfo )
	app.Post( "/api/signup/match" , handlers.SignupMatch )
	app.Post( "/api/signup/claim" , handlers.SignupClaim )
	app.Post( "/api/signup/create" , handlers.SignupCreate )

	// Any signed-in user.
	account := app.Group( "/api" , handlers.Guard.RequireLogin )
	account.Get( "/me" , handlers.GetMe )
	account.Post( "/account/rename" , handlers.RenameAccount )
	account.Post( "/logout" , handlers.Logout )
	account.Get( "/team" , handlers.ListTeam )
	account.Get( "/profile/options" , handlers.GetProfileOptions )
	account.Post( "/account/avatar" , handlers.SetAvatar )
	account.Post( "/account/pin" , handlers.SetPin )
	account.Post( "/account/bot-level" , handlers.SetBotLevel )
	account.Post( "/account/auto-play-level" , handlers.SetAutoPlayLevel )
	account.Post( "/join/:code" , handlers.JoinTable )
	// The chat everyone shares, and who is online (see global.go).
	account.Get( "/global" , handlers.GetGlobal )
	account.Get( "/global/events" , handlers.GlobalEvents )
	account.Post( "/global/chat" , handlers.GlobalChat )
	account.Post( "/global/chat/:seq/edit" , handlers.EditGlobalChat )
	account.Post( "/global/chat/:seq/delete" , handlers.DeleteGlobalChat )
	// Guarded per route rather than by a group, for the prefix reason
	// spelled out at the API keys group below.
	account.Post( "/account/login-link" , handlers.Guard.RequireSession , handlers.CreateMyLoginLink )
	account.Post( "/account/delete" , handlers.Guard.RequireSession , handlers.DeleteMyAccount )

	// Sign-up links: anyone allowed to invite manages their own, admins
	// everyone's (see signup_links.go).
	account.Get( "/signup-links" , handlers.Guard.RequireSession , handlers.RequireInviter , handlers.ListSignupLinks )
	account.Post( "/signup-links" , handlers.Guard.RequireSession , handlers.RequireInviter , handlers.CreateSignupLink )
	account.Post( "/signup-links/:link_id/revoke" , handlers.Guard.RequireSession , handlers.RequireInviter , handlers.RevokeSignupLink )
	account.Post( "/signup-links/:link_id/delete" , handlers.Guard.RequireSession , handlers.RequireOwner , handlers.DeleteSignupLink )

	// API keys: any signed-in user manages their own.
	//
	// Note the group prefix. A group's middleware is mounted on a path
	// *prefix*, not on the routes written under it, so it applies to every
	// route registered after it whose path starts with that prefix. Mounting
	// these two on "/api" would put RequireSession in front of /api/admin/users
	// as well, and an admin-scoped API key would then be refused there. That is
	// also why the prefix is exactly "/api/keys": a future "/api/keysets" would
	// pick up this middleware by sharing the string, so keep new routes off
	// these prefixes unless they want these rules.
	keys := app.Group( "/api/keys" , handlers.RequireAPIKeys , handlers.Guard.RequireLogin , handlers.Guard.RequireSession )
	keys.Get( "" , handlers.ListMyAPIKeys )
	keys.Post( "" , handlers.CreateMyAPIKey )
	keys.Post( "/:key_id/revoke" , handlers.RevokeMyAPIKey )

	// Admin only, and of the admins the owner only: deciding who gets in.
	// RequireOwner is added per route rather than on the group, for the
	// prefix reason spelled out at the API keys group above. See
	// architecture.md on defaulting new features to "any signed-in user".
	//
	// The two routes that hand out a login link also require a browser
	// session. A login link becomes a session, and a session can mint API
	// keys -- so if an admin-scoped key could create accounts or reissue
	// links, a leaked key could turn itself into a fresh session and a fresh
	// key, and revoking the leaked one would not end the compromise. The
	// rule is: an API key may use what an account can do, but it never
	// creates credentials. (The seat invite under /api/tables follows it too.)
	admin := app.Group( "/api/admin" , handlers.Guard.RequireAdmin )
	admin.Get( "/users" , handlers.RequireOwner , handlers.ListUsers )
	admin.Post( "/users" , handlers.Guard.RequireSession , handlers.RequireOwner , handlers.CreateUser )
	admin.Post( "/users/:user_id/reissue-login" , handlers.Guard.RequireSession , handlers.RequireOwner , handlers.ReissueLogin )
	admin.Post( "/users/:user_id/disabled" , handlers.RequireOwner , handlers.SetUserDisabled )
	admin.Post( "/users/:user_id/revoke-access" , handlers.RequireOwner , handlers.RevokeAccess )
	admin.Post( "/users/:user_id/can-invite" , handlers.RequireOwner , handlers.SetUserCanInvite )
	admin.Post( "/users/:user_id/can-hint" , handlers.RequireOwner , handlers.SetUserCanHint )
	admin.Post( "/users/:user_id/can-undo" , handlers.RequireOwner , handlers.SetUserCanUndo )
	admin.Post( "/users/:user_id/role" , handlers.RequireOwner , handlers.AdminSetRole )
	admin.Post( "/users/:user_id/rename" , handlers.RequireOwner , handlers.AdminRenameUser )
	admin.Post( "/users/:user_id/avatar" , handlers.RequireOwner , handlers.AdminSetAvatar )
	admin.Post( "/users/:user_id/delete" , handlers.RequireOwner , handlers.DeleteUser )
	// The app-wide game settings: the owner's, not every admin's.
	admin.Get( "/settings" , handlers.RequireOwner , handlers.GetSettings )
	admin.Post( "/settings" , handlers.RequireOwner , handlers.SaveSettings )

	// Admin view of every key in the app -- "who has standing access" -- and
	// the ability to revoke or delete someone else's. Its own prefix for the same reason
	// as above: mounted on "/api/admin" it would gate the account routes too.
	// Owner only, like the game settings.
	admin_keys := app.Group( "/api/admin/keys" , handlers.RequireAPIKeys , handlers.Guard.RequireAdmin , handlers.Guard.RequireSession , handlers.RequireOwner )
	admin_keys.Get( "" , handlers.ListAllAPIKeys )
	admin_keys.Post( "/:key_id/revoke" , handlers.RevokeAnyAPIKey )
	admin_keys.Post( "/:key_id/delete" , handlers.DeleteAnyAPIKey )

	// The card game. Any signed-in player, by session or API key -- a key
	// at a table is how a script (or a test) takes a seat. Everything that
	// changes a table is a POST carrying the CSRF token, like the rest of
	// the API; the two /events routes are long-lived Server-Sent Event
	// streams (see events.go).
	game_tables := app.Group( "/api/tables" , handlers.Guard.RequireLogin )
	game_tables.Get( "" , handlers.ListTables )
	game_tables.Post( "" , handlers.CreateTable )
	game_tables.Get( "/:table_id" , handlers.GetTable )
	game_tables.Get( "/:table_id/events" , handlers.TableEvents )
	game_tables.Post( "/:table_id/sit" , handlers.SitAtTable )
	game_tables.Post( "/:table_id/leave" , handlers.LeaveTable )
	game_tables.Post( "/:table_id/bot" , handlers.AddBot )
	game_tables.Post( "/:table_id/bot/remove" , handlers.RemoveBot )
	game_tables.Post( "/:table_id/bot/level" , handlers.SetSeatBotLevel )
	// An invite creates an account and its login link, so like every route
	// that creates a credential it is browser-only (see the admin group).
	game_tables.Post( "/:table_id/invite" , handlers.Guard.RequireSession , handlers.InviteToSeat )
	game_tables.Post( "/:table_id/assign" , handlers.AssignSeat )
	game_tables.Post( "/:table_id/seat/remove" , handlers.RemovePlayer )
	game_tables.Post( "/:table_id/seat/rename" , handlers.RenameSeat )
	game_tables.Post( "/:table_id/start" , handlers.StartTable )
	game_tables.Post( "/:table_id/restart" , handlers.RestartTable )
	game_tables.Post( "/:table_id/delete" , handlers.DeleteTable )
	game_tables.Post( "/:table_id/bid" , handlers.Bid )
	game_tables.Post( "/:table_id/bury" , handlers.Bury )
	game_tables.Post( "/:table_id/play" , handlers.PlayCard )
	game_tables.Post( "/:table_id/continue" , handlers.ContinueHand )
	game_tables.Post( "/:table_id/auto-play" , handlers.SetAutoPlay )
	game_tables.Post( "/:table_id/chat" , handlers.ChatAtTable )
	game_tables.Post( "/:table_id/chat/:seq/edit" , handlers.EditChatAtTable )
	game_tables.Post( "/:table_id/chat/:seq/delete" , handlers.DeleteChatAtTable )
	game_tables.Get( "/:table_id/hint" , handlers.GetHint )
	game_tables.Get( "/:table_id/record" , handlers.GetRecord )
	game_tables.Post( "/:table_id/takeover" , handlers.RequestTakeover )
	game_tables.Post( "/:table_id/takeover/cancel" , handlers.CancelTakeover )
	game_tables.Post( "/:table_id/takeover/answer" , handlers.AnswerTakeover )

	// The move history: undo / redo / reset and resuming a held game. Admins
	// anywhere; players given can_undo at their own tables (RequireRewind).
	// Whether watchers see every hand stays admin-only. Guarded per route,
	// since the group only requires a login.
	game_tables.Get( "/:table_id/history" , handlers.RequireRewind , handlers.GetHistory )
	game_tables.Post( "/:table_id/undo" , handlers.RequireRewind , handlers.Undo )
	game_tables.Post( "/:table_id/redo" , handlers.RequireRewind , handlers.Redo )
	game_tables.Post( "/:table_id/reset" , handlers.RequireRewind , handlers.ResetTo )
	game_tables.Post( "/:table_id/resume" , handlers.RequireRewind , handlers.Resume )
	game_tables.Post( "/:table_id/observers" , handlers.Guard.RequireAdmin , handlers.SetTableObservers )
	game_tables.Post( "/:table_id/trick-pause" , handlers.Guard.RequireAdmin , handlers.SetTableTrickPause )

	// The game history: every finished or abandoned game, for anyone
	// signed in. Only the owner may delete one.
	games := app.Group( "/api/games" , handlers.Guard.RequireLogin )
	games.Get( "" , handlers.ListGames )
	games.Get( "/:game_id" , handlers.GetGame )
	games.Post( "/:game_id/delete" , handlers.RequireOwner , handlers.DeleteGame )

	lobby := app.Group( "/api/lobby" , handlers.Guard.RequireLogin )
	lobby.Get( "/events" , handlers.LobbyEvents )

	// Catch-all static serving. Must be registered last -- it matches every
	// remaining path, so anything added after this line never runs.
	app.Get( "/*" , handlers.Static.Handler )
}

// badRequest / notFound keep error bodies uniform, so the frontend has one
// shape to handle and no handler accidentally leaks internal detail.
func badRequest( c fiber.Ctx , message string ) ( err error ) {
	err = c.Status( fiber.StatusBadRequest ).JSON( fiber.Map{ "error": message } )
	return
}

func notFound( c fiber.Ctx , message string ) ( err error ) {
	err = c.Status( fiber.StatusNotFound ).JSON( fiber.Map{ "error": message } )
	return
}

func forbidden( c fiber.Ctx , message string ) ( err error ) {
	err = c.Status( fiber.StatusForbidden ).JSON( fiber.Map{ "error": message } )
	return
}

func serverError( c fiber.Ctx ) ( err error ) {
	err = c.Status( fiber.StatusInternalServerError ).JSON( fiber.Map{ "error": "internal error" } )
	return
}
