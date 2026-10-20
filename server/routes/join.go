package routes

import (
	fiber "github.com/gofiber/fiber/v3"

	models "rook/server/models"
	security "rook/server/security"
	tables "rook/server/tables"
)

// Join links: every table has one. Opening it signed in takes a free seat.
// Opening it signed out shows the table's name and either a "your name" box
// -- when an admin has allowed sign-up through join links -- or a note to ask
// for a login link.

type joinSignupRequest struct {
	DisplayName string `json:"display_name"`
}

// GetJoin is public: it says what the link leads to and what the visitor
// can do about it, and nothing about the cards.
func ( handlers *Handlers ) GetJoin( c fiber.Ctx ) ( err error ) {
	id , code_err := handlers.Hub.ByCode( c.Params( "code" ) , false )
	if code_err != nil {
		err = handlers.tableError( c , code_err )
		return
	}
	view , view_err := handlers.Hub.View( id , tables.Viewer{} , tables.ViewOptions{} )
	if view_err != nil {
		err = handlers.tableError( c , view_err )
		return
	}
	open_seats , bot_seats := 0 , 0
	for _ , seat := range view.Seats {
		switch seat.Kind {
		case tables.KindEmpty:
			open_seats++
		case tables.KindBot:
			bot_seats++
		}
	}
	err = c.JSON( fiber.Map{
		"name":           view.Name,
		"status":         view.Status,
		"open_seats":     open_seats,
		"bot_seats":      bot_seats,
		"signup_allowed": handlers.Hub.Settings().AllowJoinSignup,
		"signed_in":      security.ActorFrom( c ) != nil,
	} )
	return
}

// JoinTable is for someone signed in: sit at the first free seat, if any.
func ( handlers *Handlers ) JoinTable( c fiber.Ctx ) ( err error ) {
	var body csrfOnlyRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	user := security.UserFrom( c )
	id , join_err := handlers.Hub.Join( c.Params( "code" ) , user )
	if join_err != nil {
		err = handlers.tableError( c , join_err )
		return
	}
	err = c.JSON( fiber.Map{ "table_id": id , "requested": handlers.requested( id , user ) } )
	return
}

// JoinSignup creates an account from a join link, signs the browser in, and
// seats them. Only while the admin's "sign up through join links" setting is
// on; the new account is always a plain player.
func ( handlers *Handlers ) JoinSignup( c fiber.Ctx ) ( err error ) {
	if security.SameOrigin( c ) == false {
		err = forbidden( c , "cross-site request" )
		return
	}
	if handlers.Hub.Settings().AllowJoinSignup == false {
		err = forbidden( c , "signup_closed" )
		return
	}
	var body joinSignupRequest
	if c.Bind().JSON( &body ) != nil || models.ValidDisplayName( body.DisplayName ) == false {
		err = badRequest( c , "bad_name" )
		return
	}
	code := c.Params( "code" )
	if _ , code_err := handlers.Hub.ByCode( code , false ); code_err != nil {
		err = handlers.tableError( c , code_err )
		return
	}
	user , create_err := models.CreateUser( handlers.Store , body.DisplayName , models.RoleUser )
	if create_err != nil {
		err = serverError( c )
		return
	}
	if handlers.startSession( c , user.ID ) != nil {
		err = serverError( c )
		return
	}
	id , join_err := handlers.Hub.Join( code , user )
	if join_err != nil {
		err = handlers.tableError( c , join_err )
		return
	}
	err = c.Status( fiber.StatusCreated ).JSON( fiber.Map{ "table_id": id , "requested": handlers.requested( id , user ) } )
	return
}

// requested says whether joining left the person asking the host for a
// bot's seat rather than sitting down, so the join page can say so.
func ( handlers *Handlers ) requested( id string , user *models.User ) ( result bool ) {
	view , view_err := handlers.Hub.View( id , tables.Viewer{ User: user } , tables.ViewOptions{} )
	result = view_err == nil && view.You < 0 && view.MyRequest >= 0
	return
}
