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
	Amount    int    `json:"amount,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

type buryRequest struct {
	Cards     []string `json:"cards,omitempty"`
	Trump     string   `json:"trump,omitempty"`
	CSRFToken string   `json:"csrf_token,omitempty"`
}

type playRequest struct {
	Card      string `json:"card,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

// @route("POST /api/tables/{table_id}/bid", summary="Bid", tag="Tables",
// )
func ( handlers *Handlers ) Bid( c fiber.Ctx , in *bidIn ) ( err error ) {
	body := in.bidRequest
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

// @route("POST /api/tables/{table_id}/bury", summary="Bury", tag="Tables",
// )
func ( handlers *Handlers ) Bury( c fiber.Ctx , in *buryIn ) ( err error ) {
	body := in.buryRequest
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.Bury( id , security.UserFrom( c ) , body.Cards , body.Trump ) )
	return
}

// @route("POST /api/tables/{table_id}/play", summary="Play card", tag="Tables",
// )
func ( handlers *Handlers ) PlayCard( c fiber.Ctx , in *playCardIn ) ( err error ) {
	body := in.playRequest
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.Play( id , security.UserFrom( c ) , body.Card ) )
	return
}

// @route("POST /api/tables/{table_id}/continue", summary="Continue hand", tag="Tables",
// )
func ( handlers *Handlers ) ContinueHand( c fiber.Ctx , in *continueHandIn ) ( err error ) {
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.Continue( id , security.UserFrom( c ) ) )
	return
}

// Reshuffle deals the hand again, for an admin or a player given
// can_reshuffle, before they have bid or passed in it.
//
// @route("POST /api/tables/{table_id}/reshuffle", summary="Reshuffle", tag="Tables",
// )
func ( handlers *Handlers ) Reshuffle( c fiber.Ctx , in *reshuffleIn ) ( err error ) {
	actor := security.ActorFrom( c )
	if actor == nil || ( actor.IsAdmin() == false && actor.User.CanReshuffle == false ) {
		err = forbidden( c , "forbidden" )
		return
	}
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.Reshuffle( id , actor.User ) )
	return
}

type autoPlayRequest struct {
	On        bool   `json:"on,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

// SetAutoPlay switches auto play on or off for the caller's seat for the rest
// of this hand. Open to every player, unlike hints: it only plays cards once
// the hand's real decisions are made.
//
// @route("POST /api/tables/{table_id}/auto-play", summary="Set auto play", tag="Tables",
// )
func ( handlers *Handlers ) SetAutoPlay( c fiber.Ctx , in *setAutoPlayIn ) ( err error ) {
	body := in.autoPlayRequest
	id := c.Params( "table_id" )
	err = handlers.tableResult( c , id , handlers.Hub.SetAutoPlay( id , security.UserFrom( c ) , body.On ) )
	return
}

// GetHint is the hint button: the move a bot would make in your seat, for
// players an admin has given hints to. It suggests; the player decides --
// the page shows it, then highlights it, then plays it only if asked.
//
// @route("GET /api/tables/{table_id}/hint", summary="Get hint", tag="Tables",
// )
func ( handlers *Handlers ) GetHint( c fiber.Ctx , in *getHintIn ) ( err error ) {
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
