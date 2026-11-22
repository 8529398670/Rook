package routes

import (
	errors "errors"
	strconv "strconv"

	fiber "github.com/gofiber/fiber/v3"

	apidoc "rook/server/apidoc"

	botnames "rook/server/botnames"
	db "rook/server/db"
	models "rook/server/models"
	security "rook/server/security"
)

// Admin powers over games: step back and forth through a table's moves, put
// it on hold, decide who may see every hand, and the app-wide game settings.
// All of these but the trick pause (open to anyone seated) are registered under RequireAdmin. Sign-up links, which
// anyone allowed to invite may make, are in signup_links.go.

type resetRequest struct {
	To        int    `json:"to,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

type observersRequest struct {
	SeeAll    bool   `json:"see_all,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

type settingsRequest struct {
	models.AppSettings
	CSRFToken string `json:"csrf_token,omitempty"`
}

type trickPauseRequest struct {
	MS        int    `json:"ms,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

type setCanUndoRequest struct {
	CanUndo   bool   `json:"can_undo,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

type setCanReshuffleRequest struct {
	CanReshuffle bool   `json:"can_reshuffle,omitempty"`
	CSRFToken    string `json:"csrf_token,omitempty"`
}

type setCanHintRequest struct {
	CanHint   bool   `json:"can_hint,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

// @route("GET /api/tables/{table_id}/history", summary="Get history", tag="Tables", use=[handlers.RequireRewind],
// )
func ( handlers *Handlers ) GetHistory( c fiber.Ctx , in *getHistoryIn ) ( err error ) {
	history , history_err := handlers.Hub.History( c.Params( "table_id" ) )
	if history_err != nil {
		err = handlers.tableError( c , history_err )
		return
	}
	err = c.JSON( history )
	return
}

// historyStep runs one of undo / redo / resume, which take no arguments.
func ( handlers *Handlers ) historyStep( c fiber.Ctx , step func( id string ) error ) ( err error ) {
	var body csrfOnlyRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , step( id ) )
	return
}

// @route("POST /api/tables/{table_id}/undo", summary="Undo", tag="Tables", use=[handlers.RequireRewind],
// )
func ( handlers *Handlers ) Undo( c fiber.Ctx , in *undoIn ) ( err error ) {
	err = handlers.historyStep( c , handlers.Hub.Undo )
	return
}

// @route("POST /api/tables/{table_id}/redo", summary="Redo", tag="Tables", use=[handlers.RequireRewind],
// )
func ( handlers *Handlers ) Redo( c fiber.Ctx , in *redoIn ) ( err error ) {
	err = handlers.historyStep( c , handlers.Hub.Redo )
	return
}

// @route("POST /api/tables/{table_id}/resume", summary="Resume", tag="Tables", use=[handlers.RequireRewind],
// )
func ( handlers *Handlers ) Resume( c fiber.Ctx , in *resumeIn ) ( err error ) {
	err = handlers.historyStep( c , handlers.Hub.Resume )
	return
}

// @route("POST /api/tables/{table_id}/reset", summary="Reset to", tag="Tables", use=[handlers.RequireRewind],
// )
func ( handlers *Handlers ) ResetTo( c fiber.Ctx , in *resetToIn ) ( err error ) {
	body := in.resetRequest
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.ResetTo( id , body.To ) )
	return
}

// @route("POST /api/tables/{table_id}/observers", summary="Set table observers", tag="Tables", role=ADMIN,
// )
func ( handlers *Handlers ) SetTableObservers( c fiber.Ctx , in *setTableObserversIn ) ( err error ) {
	body := in.observersRequest
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.SetObserversSeeAll( id , body.SeeAll ) )
	return
}

// SetTableTrickPause is the table's creator or host (or an admin) choosing
// this table's pause after a finished trick; 0 goes back to the app-wide
// default.
//
// @route("POST /api/tables/{table_id}/trick-pause", summary="Set table trick pause", tag="Tables",
// )
func ( handlers *Handlers ) SetTableTrickPause( c fiber.Ctx , in *setTableTrickPauseIn ) ( err error ) {
	body := in.trickPauseRequest
	id := c.Params( "table_id" )
	actor := security.ActorFrom( c )
	var user *models.User
	if actor != nil && actor.IsAdmin() == false { user = actor.User }
	err = handlers.tableResult( c , id , handlers.Hub.SetTrickPause( id , user , body.MS ) )
	return
}

// settingsResponse is the app-wide settings, plus what the Game settings
// screen needs to offer the names file: where it is, and how many names it
// had (0 when it is missing or unreadable).
type settingsResponse struct {
	models.AppSettings
	BotNamesFile      string `json:"bot_names_file"`
	BotNamesAvailable int    `json:"bot_names_available"`
	BotNamesDefault   int    `json:"bot_names_default_top"`
}

func ( handlers *Handlers ) settingsResult( c fiber.Ctx ) ( err error ) {
	err = c.JSON( settingsResponse{
		AppSettings:       handlers.Hub.Settings(),
		BotNamesFile:      handlers.Config.BotNamesFile,
		BotNamesAvailable: len( handlers.FileBotNames ),
		BotNamesDefault:   botnames.DefaultTop,
	} )
	return
}

// @route("GET /api/admin/settings", summary="Get settings", tag="Admin", role=ADMIN, use=[handlers.RequireOwner],
// )
func ( handlers *Handlers ) GetSettings( c fiber.Ctx , in *apidoc.None ) ( err error ) {
	err = handlers.settingsResult( c )
	return
}

// @route("POST /api/admin/settings", summary="Save settings", tag="Admin", role=ADMIN, use=[handlers.RequireOwner],
// )
func ( handlers *Handlers ) SaveSettings( c fiber.Ctx , in *saveSettingsIn ) ( err error ) {
	body := in.settingsRequest
	if models.ValidTrickPause( body.TrickPauseMS ) == false {
		err = badRequest( c , "bad_trick_pause" )
		return
	}
	if models.ValidRookRank( body.RookRank ) == false || models.ValidBotNames( body.BotNames , body.BotNamesTop ) == false {
		err = badRequest( c , "bad_options" )
		return
	}
	if models.ValidNewUserLevel( body.NewUserHintLevel ) == false ||
		models.ValidNewUserLevel( body.NewUserBotLevel ) == false ||
		models.ValidNewUserLevel( body.NewUserAutoPlayLevel ) == false {
		err = badRequest( c , "bad_options" )
		return
	}
	if handlers.Hub.SetSettings( body.AppSettings ) != nil {
		err = serverError( c )
		return
	}
	err = handlers.settingsResult( c )
	return
}

// @route("POST /api/admin/users/{user_id}/can-hint", summary="Set user can hint", tag="Admin", role=ADMIN, use=[handlers.RequireAccountAdmin],
// )
func ( handlers *Handlers ) SetUserCanHint( c fiber.Ctx , in *setUserCanHintIn ) ( err error ) {
	body := in.setCanHintRequest
	user_id , parse_err := strconv.ParseUint( c.Params( "user_id" ) , 10 , 64 )
	if parse_err != nil {
		err = badRequest( c , "invalid user id" )
		return
	}
	if models.SetUserCanHint( handlers.Store , user_id , body.CanHint ) != nil {
		err = notFound( c , "no such user" )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}

// SetUserCanUndo switches a player's power to step through the moves of the
// games they sit in. Their open tables are refreshed so the controls appear
// or go at once.
//
// @route("POST /api/admin/users/{user_id}/can-undo", summary="Set user can undo", tag="Admin", role=ADMIN, use=[handlers.RequireAccountAdmin],
// )
func ( handlers *Handlers ) SetUserCanUndo( c fiber.Ctx , in *setUserCanUndoIn ) ( err error ) {
	body := in.setCanUndoRequest
	user_id , parse_err := strconv.ParseUint( c.Params( "user_id" ) , 10 , 64 )
	if parse_err != nil {
		err = badRequest( c , "invalid user id" )
		return
	}
	if models.SetUserCanUndo( handlers.Store , user_id , body.CanUndo ) != nil {
		err = notFound( c , "no such user" )
		return
	}
	handlers.refreshTables( user_id )
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}

// SetUserCanReshuffle switches a player's power to deal a hand again before
// they bid. Their open tables are refreshed so the button appears or goes.
//
// @route("POST /api/admin/users/{user_id}/can-reshuffle", summary="Set user can reshuffle", tag="Admin", role=ADMIN, use=[handlers.RequireAccountAdmin],
// )
func ( handlers *Handlers ) SetUserCanReshuffle( c fiber.Ctx , in *setUserCanReshuffleIn ) ( err error ) {
	body := in.setCanReshuffleRequest
	user_id , parse_err := strconv.ParseUint( c.Params( "user_id" ) , 10 , 64 )
	if parse_err != nil {
		err = badRequest( c , "invalid user id" )
		return
	}
	if models.SetUserCanReshuffle( handlers.Store , user_id , body.CanReshuffle ) != nil {
		err = notFound( c , "no such user" )
		return
	}
	handlers.refreshTables( user_id )
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}

// RequireOwner admits only the owner -- the account the server was set up
// with -- acting with admin rights. The app-wide game settings and the list
// of everyone's API keys are theirs alone, not every admin's.
func ( handlers *Handlers ) RequireOwner( c fiber.Ctx ) ( err error ) {
	if handlers.isOwner( c ) == false {
		err = forbidden( c , "forbidden" )
		return
	}
	err = c.Next()
	return
}

// RequireAccountAdmin guards the routes that run other people's accounts
// (/api/admin/users...). Any admin may use them, as in the other apps the
// project portal runs, so the portal can work through an admin account of
// its own rather than the owner's. The ladder is maySuperActOnTx: nobody
// acts on the owner's account, and only the owner acts on another admin's.
// A route without a {user_id} (list, create) needs only the admin role.
// What stays the owner's alone -- game settings, everyone's API keys, the
// logs -- keeps RequireOwner.
func ( handlers *Handlers ) RequireAccountAdmin( c fiber.Ctx ) ( err error ) {
	actor := security.ActorFrom( c )
	if actor == nil || actor.IsAdmin() == false {
		err = forbidden( c , "forbidden" )
		return
	}
	raw := c.Params( "user_id" )
	if raw == "" {
		err = c.Next()
		return
	}
	user_id , parse_err := strconv.ParseUint( raw , 10 , 64 )
	if parse_err != nil {
		err = badRequest( c , "invalid user id" )
		return
	}
	switch act_err := models.MaySuperActOn( handlers.Store , actor.User.ID , user_id ); {
	case act_err == nil:
		err = c.Next()
	case errors.Is( act_err , db.ErrNotFound ):
		err = notFound( c , "no such user" )
	case errors.Is( act_err , models.ErrOwnerProtected ):
		err = forbidden( c , "only the owner can change the owner's account" )
	case errors.Is( act_err , models.ErrOwnerRequired ):
		err = forbidden( c , "only the owner can do this to an admin account" )
	default:
		err = serverError( c , act_err )
	}
	return
}

// isOwner answers whether the caller is the owner acting with admin rights:
// RequireOwner's test, for a handler that lets the owner do more rather
// than refusing everyone else.
func ( handlers *Handlers ) isOwner( c fiber.Ctx ) ( result bool ) {
	actor := security.ActorFrom( c )
	result = actor != nil && actor.IsAdmin() && models.IsSuperAdmin( handlers.Store , actor.User.ID )
	return
}

// RequireRewind guards a table's move history: undo, redo, reset, resume.
// Admins may use it at any table; a player an admin gave can_undo to, only
// at a table they are sitting at.
func ( handlers *Handlers ) RequireRewind( c fiber.Ctx ) ( err error ) {
	actor := security.ActorFrom( c )
	allowed := actor != nil && ( actor.IsAdmin() ||
		( actor.User.CanUndo && handlers.Hub.IsSeated( c.Params( "table_id" ) , actor.User.ID ) ) )
	if allowed == false {
		err = forbidden( c , "forbidden" )
		return
	}
	err = c.Next()
	return
}
