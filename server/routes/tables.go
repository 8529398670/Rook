package routes

import (
	strconv "strconv"
	strings "strings"

	fiber "github.com/gofiber/fiber/v3"

	apidoc "rook/server/apidoc"

	botnames "rook/server/botnames"
	game "rook/server/game"
	models "rook/server/models"
	security "rook/server/security"
	tables "rook/server/tables"
)

// Table routes stay thin: decode, check CSRF, call one Hub method, report.
// Every rule and every refusal reason lives in server/tables or server/game.

type createTableRequest struct {
	Name      string             `json:"name,omitempty"`
	Options   createTableOptions `json:"options,omitempty"`
	CSRFToken string             `json:"csrf_token,omitempty"`
}

// createTableOptions is what a new table's creator may send: every field is
// optional, so the lobby can send play_to alone. The bird's rules are taken
// but ignored -- Hub.Create sets them from the app's settings -- and are here
// only so a client that still sends them is not refused. PlayTo is a pointer
// because 0 is a real choice (one hand) and leaving it out means the default.
type createTableOptions struct {
	PlayTo         *int          `json:"play_to,omitempty"`
	RookRank       game.RookRank `json:"rook_rank,omitempty"`
	RookAnytime    bool          `json:"rook_anytime,omitempty"`
	RookFirstTrick bool          `json:"rook_first_trick,omitempty"`
}

// game turns the request's options into a table's, the defaults filling in
// whatever was left out.
func ( sent createTableOptions ) game() ( options game.Options ) {
	options = game.DefaultOptions()
	if sent.PlayTo != nil { options.PlayTo = *sent.PlayTo }
	return
}

type seatRequest struct {
	Seat      int    `json:"seat,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

type answerRequest struct {
	UserID    uint64 `json:"user_id,omitempty"`
	Accept    bool   `json:"accept,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

type inviteRequest struct {
	Seat        int    `json:"seat,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	CSRFToken   string `json:"csrf_token,omitempty"`
}

type assignRequest struct {
	Seat      int    `json:"seat,omitempty"`
	UserID    uint64 `json:"user_id,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

// addBotRequest picks the seat and, optionally, how well the bot plays:
// easy, normal, hard or expert. Leaving it out means normal.
type addBotRequest struct {
	Seat      int    `json:"seat,omitempty"`
	Level     string `json:"level,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

type renameSeatRequest struct {
	Seat      int    `json:"seat,omitempty"`
	Name      string `json:"name,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

type chatRequest struct {
	Text      string `json:"text,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
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
	case string( tables.ErrNoSuchTable ) , string( tables.ErrNoSuchLink ) , string( tables.ErrNoSuchGame ):
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
	if c.Bind().JSON( body ) != nil {
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

// @route("GET /api/tables", summary="List tables", tag="Tables",
// )
func ( handlers *Handlers ) ListTables( c fiber.Ctx , in *apidoc.None ) ( err error ) {
	err = c.JSON( handlers.Hub.List( security.UserFrom( c ) ) )
	return
}

// @route("POST /api/tables", summary="Create table", tag="Tables",
// )
func ( handlers *Handlers ) CreateTable( c fiber.Ctx , in *createTableIn ) ( err error ) {
	body := createTableRequest{}
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id , create_err := handlers.Hub.Create( security.UserFrom( c ) , body.Name , body.Options.game() )
	if create_err != nil {
		err = handlers.tableError( c , create_err )
		return
	}
	c.Status( fiber.StatusCreated )
	err = handlers.tableResult( c , id , nil )
	return
}

// @route("GET /api/tables/{table_id}", summary="Get table", tag="Tables",
// )
func ( handlers *Handlers ) GetTable( c fiber.Ctx , in *getTableIn ) ( err error ) {
	err = handlers.tableResult( c , c.Params( "table_id" ) , nil )
	return
}

// @route("POST /api/tables/{table_id}/sit", summary="Sit at table", tag="Tables",
// )
func ( handlers *Handlers ) SitAtTable( c fiber.Ctx , in *sitAtTableIn ) ( err error ) {
	body := in.seatRequest
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.Sit( id , security.UserFrom( c ) , body.Seat ) )
	return
}

// @route("POST /api/tables/{table_id}/leave", summary="Leave table", tag="Tables",
// )
func ( handlers *Handlers ) LeaveTable( c fiber.Ctx , in *leaveTableIn ) ( err error ) {
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

// @route("POST /api/tables/{table_id}/bot", summary="Add bot", tag="Tables",
// )
func ( handlers *Handlers ) AddBot( c fiber.Ctx , in *addBotIn ) ( err error ) {
	body := in.addBotRequest
	id , user := c.Params( "table_id" ) , security.UserFrom( c )
	// Every bot gets a random name, emoji and color. Where the names come
	// from is the owner's choice in Game settings (see randomBotNames).
	look := tables.BotLook{ Names: handlers.randomBotNames() , Icons: models.BotIcons }
	err = handlers.tableResult( c , id , handlers.Hub.AddBotLooking( id , user , body.Seat , body.Level , look ) )
	return
}

// usingFileBotNames says the owner chose the names file in Game settings and
// it loaded.
func ( handlers *Handlers ) usingFileBotNames() ( ok bool ) {
	ok = handlers.Hub.Settings().BotNames == models.BotNamesFile && len( handlers.FileBotNames ) > 0
	return
}

// randomBotNames is the list a random-looking bot's name comes from: the
// most popular names in the names file when the owner chose it in Game
// settings (and it loaded), else bots.random_names in language.yaml,
// separated by commas.
func ( handlers *Handlers ) randomBotNames() ( names []string ) {
	if handlers.usingFileBotNames() {
		names = botnames.Top( handlers.FileBotNames , handlers.Hub.Settings().BotNamesTop )
		return
	}
	for _ , name := range strings.Split( handlers.Language.Get( "bots.random_names" ) , "," ) {
		if name = strings.TrimSpace( name ); name != "" { names = append( names , name ) }
	}
	return
}

// SetSeatBotLevel changes how well a computer player plays: the host's or an
// admin's call.
//
// @route("POST /api/tables/{table_id}/bot/level", summary="Set seat bot level", tag="Tables",
// )
func ( handlers *Handlers ) SetSeatBotLevel( c fiber.Ctx , in *setSeatBotLevelIn ) ( err error ) {
	body := in.addBotRequest
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.SetBotLevel( id , security.UserFrom( c ) , viewerFrom( c ).Admin , body.Seat , body.Level ) )
	return
}

// @route("POST /api/tables/{table_id}/bot/remove", summary="Remove bot", tag="Tables",
// )
func ( handlers *Handlers ) RemoveBot( c fiber.Ctx , in *removeBotIn ) ( err error ) {
	body := in.seatRequest
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.RemoveBot( id , security.UserFrom( c ) , body.Seat ) )
	return
}

// RemovePlayer takes someone else out of their seat: the host's or an
// admin's call.
//
// @route("POST /api/tables/{table_id}/seat/remove", summary="Remove player", tag="Tables",
// )
func ( handlers *Handlers ) RemovePlayer( c fiber.Ctx , in *removePlayerIn ) ( err error ) {
	body := in.seatRequest
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.RemovePlayer( id , security.UserFrom( c ) , viewerFrom( c ).Admin , body.Seat ) )
	return
}

// RenameSeat renames a seat's player at this table only: the host's or an
// admin's call.
//
// @route("POST /api/tables/{table_id}/seat/rename", summary="Rename seat", tag="Tables",
// )
func ( handlers *Handlers ) RenameSeat( c fiber.Ctx , in *renameSeatIn ) ( err error ) {
	body := in.renameSeatRequest
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.RenameSeat( id , security.UserFrom( c ) , viewerFrom( c ).Admin , body.Seat , body.Name ) )
	return
}

// ShuffleSeatBot gives a bot a new random name, emoji and color.
//
// @route("POST /api/tables/{table_id}/seat/shuffle", summary="Shuffle seat bot", tag="Tables",
// )
func ( handlers *Handlers ) ShuffleSeatBot( c fiber.Ctx , in *shuffleSeatBotIn ) ( err error ) {
	body := in.seatRequest
	id := c.Params( "table_id" )
	look := tables.BotLook{ Names: handlers.randomBotNames() , Icons: models.BotIcons }
	err = handlers.tableResult( c , id , handlers.Hub.ShuffleBot( id , security.UserFrom( c ) , viewerFrom( c ).Admin , body.Seat , look ) )
	return
}

// UnshuffleSeatBot puts back the look a bot had before its last shuffle.
//
// @route("POST /api/tables/{table_id}/seat/unshuffle", summary="Unshuffle seat bot", tag="Tables",
// )
func ( handlers *Handlers ) UnshuffleSeatBot( c fiber.Ctx , in *unshuffleSeatBotIn ) ( err error ) {
	body := in.seatRequest
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.UnshuffleBot( id , security.UserFrom( c ) , viewerFrom( c ).Admin , body.Seat ) )
	return
}

type botLookRequest struct {
	Seat      int    `json:"seat,omitempty"`
	Icon      string `json:"icon,omitempty"`
	Color     string `json:"color,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

// SetSeatBotLook picks a bot's emoji and color by hand.
//
// @route("POST /api/tables/{table_id}/seat/look", summary="Set seat bot look", tag="Tables",
// )
func ( handlers *Handlers ) SetSeatBotLook( c fiber.Ctx , in *setSeatBotLookIn ) ( err error ) {
	body := in.botLookRequest
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.SetBotLook( id , security.UserFrom( c ) , viewerFrom( c ).Admin , body.Seat , body.Icon , body.Color ) )
	return
}

// RequestTakeover asks the host to let a watcher replace a bot.
//
// @route("POST /api/tables/{table_id}/takeover", summary="Request takeover", tag="Tables",
// )
func ( handlers *Handlers ) RequestTakeover( c fiber.Ctx , in *requestTakeoverIn ) ( err error ) {
	body := in.seatRequest
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.RequestSeat( id , security.UserFrom( c ) , body.Seat ) )
	return
}

// @route("POST /api/tables/{table_id}/takeover/cancel", summary="Cancel takeover", tag="Tables",
// )
func ( handlers *Handlers ) CancelTakeover( c fiber.Ctx , in *cancelTakeoverIn ) ( err error ) {
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.CancelRequest( id , security.UserFrom( c ) ) )
	return
}

// AnswerTakeover is the host's (or an admin's) yes or no.
//
// @route("POST /api/tables/{table_id}/takeover/answer", summary="Answer takeover", tag="Tables",
// )
func ( handlers *Handlers ) AnswerTakeover( c fiber.Ctx , in *answerTakeoverIn ) ( err error ) {
	body := in.answerRequest
	id := c.Params( "table_id" )
	admin := viewerFrom( c ).Admin
	err = handlers.tableResult( c , id , handlers.Hub.AnswerRequest( id , security.UserFrom( c ) , admin , body.UserID , body.Accept ) )
	return
}

// RestartTable starts the game over at the same table: the host's or an
// admin's call.
//
// @route("POST /api/tables/{table_id}/restart", summary="Restart table", tag="Tables",
// )
func ( handlers *Handlers ) RestartTable( c fiber.Ctx , in *restartTableIn ) ( err error ) {
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.Restart( id , security.UserFrom( c ) , viewerFrom( c ).Admin ) )
	return
}

// DeleteTable removes the table for everyone: the host's or an admin's call.
//
// @route("POST /api/tables/{table_id}/delete", summary="Delete table", tag="Tables",
// )
func ( handlers *Handlers ) DeleteTable( c fiber.Ctx , in *deleteTableIn ) ( err error ) {
	if delete_err := handlers.Hub.Delete( c.Params( "table_id" ) , security.UserFrom( c ) , viewerFrom( c ).Admin ); delete_err != nil {
		err = handlers.tableError( c , delete_err )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}

// GetRecord is the game so far, hand by hand and trick by trick, with
// nothing hidden in it: what the game log tab draws.
//
// @route("GET /api/tables/{table_id}/record", summary="Get record", tag="Tables",
// )
func ( handlers *Handlers ) GetRecord( c fiber.Ctx , in *getRecordIn ) ( err error ) {
	record , record_err := handlers.Hub.Record( c.Params( "table_id" ) )
	if record_err != nil {
		err = handlers.tableError( c , record_err )
		return
	}
	err = c.JSON( record )
	return
}

// startRequest names the first dealer: a seat, or random_dealer. With
// neither, whoever pressed Deal deals.
type startRequest struct {
	Dealer       *int   `json:"dealer,omitempty"`
	RandomDealer bool   `json:"random_dealer,omitempty"`
	CSRFToken    string `json:"csrf_token,omitempty"`
}

// @route("POST /api/tables/{table_id}/start", summary="Start table", tag="Tables",
// )
func ( handlers *Handlers ) StartTable( c fiber.Ctx , in *startTableIn ) ( err error ) {
	body := in.startRequest
	dealer := tables.DealerHost
	if body.RandomDealer {
		dealer = tables.DealerRandom
	} else if body.Dealer != nil {
		dealer = *body.Dealer
		// A seat number only; the two named choices go by their own fields.
		if dealer < 0 { dealer = 4 }
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.Start( id , security.UserFrom( c ) , dealer ) )
	return
}

// @route("POST /api/tables/{table_id}/chat", summary="Chat at table", tag="Tables",
// )
func ( handlers *Handlers ) ChatAtTable( c fiber.Ctx , in *chatAtTableIn ) ( err error ) {
	body := in.chatRequest
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.Chat( id , security.UserFrom( c ) , body.Text ) )
	return
}

// EditChatAtTable and DeleteChatAtTable change one of the caller's own
// lines in a table's chat -- or anyone's, for the owner.
//
// @route("POST /api/tables/{table_id}/chat/{seq}/edit", summary="Edit chat at table", tag="Tables",
// )
func ( handlers *Handlers ) EditChatAtTable( c fiber.Ctx , in *editChatAtTableIn ) ( err error ) {
	body := in.chatRequest
	seq , ok := chatSeq( c )
	if ok == false {
		err = badRequest( c , "no_such_message" )
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.EditChat( id , security.UserFrom( c ) , handlers.isOwner( c ) , seq , body.Text ) )
	return
}

// @route("POST /api/tables/{table_id}/chat/{seq}/delete", summary="Delete chat at table", tag="Tables",
// )
func ( handlers *Handlers ) DeleteChatAtTable( c fiber.Ctx , in *deleteChatAtTableIn ) ( err error ) {
	seq , ok := chatSeq( c )
	if ok == false {
		err = badRequest( c , "no_such_message" )
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.DeleteChat( id , security.UserFrom( c ) , handlers.isOwner( c ) , seq ) )
	return
}

// InviteToSeat is the admin "create user" flow aimed at one empty chair:
// create the account, mint its one-time login link, and hold the seat for
// it. Admins can always do it; anyone else only if an admin switched on
// can_invite for them.
//
// The new account is always a plain user. Inviting is bringing a friend to
// the table, not handing out the keys to the server.
//
// @route("POST /api/tables/{table_id}/invite", summary="Invite to seat", tag="Tables", access=SESSION_ONLY,
// )
func ( handlers *Handlers ) InviteToSeat( c fiber.Ctx , in *inviteToSeatIn ) ( err error ) {
	body := in.inviteRequest
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
//
// @route("POST /api/tables/{table_id}/assign", summary="Assign seat", tag="Tables",
// )
func ( handlers *Handlers ) AssignSeat( c fiber.Ctx , in *assignSeatIn ) ( err error ) {
	body := in.assignRequest
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
