package routes

import (
	fiber "github.com/gofiber/fiber/v3"

	security "rook/server/security"
	tables "rook/server/tables"
)

// The four things a player does with cards. Each answers with the player's
// new view, or a refusal code such as must_follow_suit.

type bidRequest struct {
	// Amount 0 is a pass. A separate pass endpoint would be one more route
	// for the same decision.
	Amount    int    `json:"amount"`
	CSRFToken string `json:"csrf_token"`
}

type buryRequest struct {
	Cards     []string `json:"cards"`
	Trump     string   `json:"trump"`
	CSRFToken string   `json:"csrf_token"`
}

type playRequest struct {
	Card      string `json:"card"`
	CSRFToken string `json:"csrf_token"`
}

func ( handlers *Handlers ) Bid( c fiber.Ctx ) ( err error ) {
	var body bidRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id , user := c.Params( "table_id" ) , security.UserFrom( c )
	var action_err error
	if body.Amount == 0 {
		action_err = handlers.Hub.Pass( id , user )
	} else {
		action_err = handlers.Hub.Bid( id , user , body.Amount )
	}
	err = handlers.tableResult( c , id , action_err )
	return
}

func ( handlers *Handlers ) Bury( c fiber.Ctx ) ( err error ) {
	var body buryRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.Bury( id , security.UserFrom( c ) , body.Cards , body.Trump ) )
	return
}

func ( handlers *Handlers ) PlayCard( c fiber.Ctx ) ( err error ) {
	var body playRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.Play( id , security.UserFrom( c ) , body.Card ) )
	return
}

func ( handlers *Handlers ) ContinueHand( c fiber.Ctx ) ( err error ) {
	var body csrfOnlyRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.Continue( id , security.UserFrom( c ) ) )
	return
}

// GetHint is the hint button: the move a bot would make in your seat, for
// players an admin has given hints to. It suggests; the player decides --
// the page shows it, then highlights it, then plays it only if asked.
func ( handlers *Handlers ) GetHint( c fiber.Ctx ) ( err error ) {
	user := security.UserFrom( c )
	if user.CanHint == false {
		err = forbidden( c , "hints_off" )
		return
	}
	action , hint_err := handlers.Hub.Hint( c.Params( "table_id" ) , user )
	if hint_err != nil {
		err = handlers.tableError( c , hint_err )
		return
	}
	err = c.JSON( hintView( action ) )
	return
}

func hintView( action tables.Action ) ( result fiber.Map ) {
	result = fiber.Map{ "kind": action.Kind }
	switch action.Kind {
	case tables.ActionBid:
		result[ "amount" ] = action.Amount
	case tables.ActionBury:
		result[ "cards" ] = action.Cards
		result[ "trump" ] = action.Trump
	case tables.ActionPlay:
		result[ "card" ] = action.Card
	}
	return
}
