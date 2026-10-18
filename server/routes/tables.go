package routes

import (
	strconv "strconv"

	fiber "github.com/gofiber/fiber/v3"

	game "rook/server/game"
	models "rook/server/models"
	security "rook/server/security"
	tables "rook/server/tables"
)

// Table routes stay thin: decode, check CSRF, call one Hub method, report.
// Every rule and every refusal reason lives in server/tables or server/game.

type createTableRequest struct {
	Name      string       `json:"name"`
	Options   game.Options `json:"options"`
	CSRFToken string       `json:"csrf_token"`
}

type seatRequest struct {
	Seat      int    `json:"seat"`
	CSRFToken string `json:"csrf_token"`
}

type answerRequest struct {
	UserID    uint64 `json:"user_id"`
	Accept    bool   `json:"accept"`
	CSRFToken string `json:"csrf_token"`
}

type inviteRequest struct {
	Seat        int    `json:"seat"`
	DisplayName string `json:"display_name"`
	CSRFToken   string `json:"csrf_token"`
}

type assignRequest struct {
	Seat      int    `json:"seat"`
	UserID    uint64 `json:"user_id"`
	CSRFToken string `json:"csrf_token"`
}

type renameSeatRequest struct {
	Seat      int    `json:"seat"`
	Name      string `json:"name"`
	CSRFToken string `json:"csrf_token"`
}

type chatRequest struct {
	Text      string `json:"text"`
	CSRFToken string `json:"csrf_token"`
}

// viewerFrom is who is asking, by their effective role (an admin's
// user-scoped key does not see like an admin). Anonymous on watch links.
func viewerFrom( c fiber.Ctx ) ( viewer tables.Viewer ) {
	if actor := security.ActorFrom( c ); actor != nil {
		viewer = tables.Viewer{ User: actor.User , Admin: actor.IsAdmin() }
	}
	return
}

// viewOptions reads ?seat=N&all=1: whose seat a watcher looks from, and
// whether they want every hand (granted only if they may).
func viewOptions( c fiber.Ctx ) ( options tables.ViewOptions ) {
	seat , parse_err := strconv.Atoi( c.Query( "seat" ) )
	if parse_err != nil { seat = -1 }
	options = tables.ViewOptions{ Perspective: seat , AllHands: c.Query( "all" ) == "1" }
	return
}

// tableResult answers a table action: the caller's fresh view on success, so
// the screen updates without waiting for the stream, or the refusal code.
func ( handlers *Handlers ) tableResult( c fiber.Ctx , id string , action_err error ) ( err error ) {
	if action_err != nil {
		err = handlers.tableError( c , action_err )
		return
	}
	view , view_err := handlers.Hub.View( id , viewerFrom( c ) , viewOptions( c ) )
	if view_err != nil {
		err = handlers.tableError( c , view_err )
		return
	}
	err = c.JSON( view )
	return
}

// tableError reports a refusal as {"error": "<code>"} -- a code, not a
// sentence, which the frontend looks up under errors.<code> in
// language.yaml. Anything that is not a refusal is a real failure.
func ( handlers *Handlers ) tableError( c fiber.Ctx , action_err error ) ( err error ) {
	code , ok := tables.IsUserError( action_err )
	if ok == false {
		err = serverError( c )
		return
	}
	status := fiber.StatusBadRequest
	switch code {
	case string( tables.ErrNoSuchTable ) , string( tables.ErrNoSuchLink ):
		status = fiber.StatusNotFound
	case string( tables.ErrNotHost ) , string( tables.ErrCannotInvite ):
		status = fiber.StatusForbidden
	case string( tables.ErrTooManyTables ):
		status = fiber.StatusServiceUnavailable
	}
	err = c.Status( status ).JSON( fiber.Map{ "error": code } )
	return
}

// decode binds the body and checks its CSRF token in one step, since every
// state-changing table route needs both.
func ( handlers *Handlers ) decode( c fiber.Ctx , body any , csrf func() string ) ( ok bool , err error ) {
	if c.Bind().Body( body ) != nil {
		err = badRequest( c , "malformed request body" )
		return
	}
	if handlers.Guard.CheckCSRF( c , csrf() ) == false {
		err = forbidden( c , "invalid csrf token" )
		return
	}
	ok = true
	return
}

func ( handlers *Handlers ) ListTables( c fiber.Ctx ) ( err error ) {
	err = c.JSON( handlers.Hub.List( security.UserFrom( c ) ) )
	return
}

func ( handlers *Handlers ) CreateTable( c fiber.Ctx ) ( err error ) {
	body := createTableRequest{ Options: game.DefaultOptions() }
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id , create_err := handlers.Hub.Create( security.UserFrom( c ) , body.Name , body.Options )
	if create_err != nil {
		err = handlers.tableError( c , create_err )
		return
	}
	c.Status( fiber.StatusCreated )
	err = handlers.tableResult( c , id , nil )
	return
}

func ( handlers *Handlers ) GetTable( c fiber.Ctx ) ( err error ) {
	err = handlers.tableResult( c , c.Params( "table_id" ) , nil )
	return
}

func ( handlers *Handlers ) SitAtTable( c fiber.Ctx ) ( err error ) {
	var body seatRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.Sit( id , security.UserFrom( c ) , body.Seat ) )
	return
}

func ( handlers *Handlers ) LeaveTable( c fiber.Ctx ) ( err error ) {
	var body csrfOnlyRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	if leave_err := handlers.Hub.Leave( id , security.UserFrom( c ) ); leave_err != nil {
		err = handlers.tableError( c , leave_err )
		return
	}
	// The table may be gone now (the last person left), so there is no view
	// to send back.
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}

func ( handlers *Handlers ) AddBot( c fiber.Ctx ) ( err error ) {
	var body seatRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.AddBot( id , security.UserFrom( c ) , body.Seat ) )
	return
}

func ( handlers *Handlers ) RemoveBot( c fiber.Ctx ) ( err error ) {
	var body seatRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.RemoveBot( id , security.UserFrom( c ) , body.Seat ) )
	return
}

// RemovePlayer takes someone else out of their seat: the host's or an
// admin's call.
func ( handlers *Handlers ) RemovePlayer( c fiber.Ctx ) ( err error ) {
	var body seatRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.RemovePlayer( id , security.UserFrom( c ) , viewerFrom( c ).Admin , body.Seat ) )
	return
}

// RenameSeat renames a seat's player at this table only: the host's or an
// admin's call.
func ( handlers *Handlers ) RenameSeat( c fiber.Ctx ) ( err error ) {
	var body renameSeatRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.RenameSeat( id , security.UserFrom( c ) , viewerFrom( c ).Admin , body.Seat , body.Name ) )
	return
}

// RequestTakeover asks the host to let a watcher replace a bot.
func ( handlers *Handlers ) RequestTakeover( c fiber.Ctx ) ( err error ) {
	var body seatRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.RequestSeat( id , security.UserFrom( c ) , body.Seat ) )
	return
}

func ( handlers *Handlers ) CancelTakeover( c fiber.Ctx ) ( err error ) {
	var body csrfOnlyRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.CancelRequest( id , security.UserFrom( c ) ) )
	return
}

// AnswerTakeover is the host's (or an admin's) yes or no.
func ( handlers *Handlers ) AnswerTakeover( c fiber.Ctx ) ( err error ) {
	var body answerRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	admin := viewerFrom( c ).Admin
	err = handlers.tableResult( c , id , handlers.Hub.AnswerRequest( id , security.UserFrom( c ) , admin , body.UserID , body.Accept ) )
	return
}

// RestartTable starts the game over at the same table: the host's or an
// admin's call.
func ( handlers *Handlers ) RestartTable( c fiber.Ctx ) ( err error ) {
	var body csrfOnlyRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.Restart( id , security.UserFrom( c ) , viewerFrom( c ).Admin ) )
	return
}

// DeleteTable removes the table for everyone: the host's or an admin's call.
func ( handlers *Handlers ) DeleteTable( c fiber.Ctx ) ( err error ) {
	var body csrfOnlyRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	if delete_err := handlers.Hub.Delete( c.Params( "table_id" ) , security.UserFrom( c ) , viewerFrom( c ).Admin ); delete_err != nil {
		err = handlers.tableError( c , delete_err )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}

// GetRecord is the game so far, hand by hand and trick by trick, with
// nothing hidden in it: what the game log tab draws.
func ( handlers *Handlers ) GetRecord( c fiber.Ctx ) ( err error ) {
	record , record_err := handlers.Hub.Record( c.Params( "table_id" ) )
	if record_err != nil {
		err = handlers.tableError( c , record_err )
		return
	}
	err = c.JSON( record )
	return
}

func ( handlers *Handlers ) StartTable( c fiber.Ctx ) ( err error ) {
	var body csrfOnlyRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.Start( id , security.UserFrom( c ) ) )
	return
}

func ( handlers *Handlers ) ChatAtTable( c fiber.Ctx ) ( err error ) {
	var body chatRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.Chat( id , security.UserFrom( c ) , body.Text ) )
	return
}

// InviteToSeat is the admin "create user" flow aimed at one empty chair:
// create the account, mint its one-time login link, and hold the seat for
// it. Admins can always do it; anyone else only if an admin switched on
// can_invite for them.
//
// The new account is always a plain user. Inviting is bringing a friend to
// the table, not handing out the keys to the server.
func ( handlers *Handlers ) InviteToSeat( c fiber.Ctx ) ( err error ) {
	var body inviteRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	inviter := security.UserFrom( c )
	if inviter.MayInvite() == false {
		err = handlers.tableError( c , tables.ErrCannotInvite )
		return
	}
	if models.ValidDisplayName( body.DisplayName ) == false {
		err = badRequest( c , "display name must be 1-80 characters" )
		return
	}
	id := c.Params( "table_id" )
	if reserve_err := handlers.Hub.CanReserve( id , body.Seat ); reserve_err != nil {
		err = handlers.tableError( c , reserve_err )
		return
	}

	invitee , create_err := models.CreateUser( handlers.Store , body.DisplayName , models.RoleUser )
	if create_err != nil {
		err = serverError( c )
		return
	}
	credential , token_err := models.IssueLoginToken( handlers.Store , invitee.ID , handlers.Config.LoginTokenTTL )
	if token_err != nil {
		err = serverError( c )
		return
	}
	if reserve_err := handlers.Hub.Reserve( id , body.Seat , invitee ); reserve_err != nil {
		// Someone took the seat in the instant between the check and now.
		// The account and link still work; they just arrive without a seat.
		err = handlers.tableError( c , reserve_err )
		return
	}

	err = c.Status( fiber.StatusCreated ).JSON( fiber.Map{
		"user_id":            invitee.ID,
		"login_path":         "/login/" + credential,
		"expires_in_seconds": int( handlers.Config.LoginTokenTTL.Seconds() ),
	} )
	return
}

// AssignSeat holds an empty chair for someone who already has an account,
// the way an invite does for a new one. No account is created, so the
// table's host may do it as well as anyone allowed to invite.
func ( handlers *Handlers ) AssignSeat( c fiber.Ctx ) ( err error ) {
	var body assignRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	caller := security.UserFrom( c )
	if caller.MayInvite() == false && viewerFrom( c ).Admin == false && handlers.Hub.IsHost( id , caller.ID ) == false {
		err = handlers.tableError( c , tables.ErrCannotInvite )
		return
	}
	invitee , get_err := models.GetUser( handlers.Store , body.UserID )
	if get_err != nil || invitee == nil || invitee.Disabled() {
		err = badRequest( c , "no_such_user" )
		return
	}
	err = handlers.tableResult( c , id , handlers.Hub.Reserve( id , body.Seat , invitee ) )
	return
}
