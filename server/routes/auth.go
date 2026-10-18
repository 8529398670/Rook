package routes

import (
	strings "strings"

	fiber "github.com/gofiber/fiber/v3"

	models "rook/server/models"
)

// RedeemLoginLink is the whole login flow. Visiting a valid link is the
// login; there is no form, no password, and nothing to phish out of a user
// beyond the link itself, which dies on first use.
//
// The route is registered as /login/* rather than /login/:credential because
// the credential contains a "." and route parameters should not have to care
// about the shape of a secret.
func ( handlers *Handlers ) RedeemLoginLink( c fiber.Ctx ) ( err error ) {
	credential := strings.Trim( c.Params( "*" ) , "/" )
	if credential == "" {
		err = c.Redirect().Status( fiber.StatusSeeOther ).To( "/?login=invalid" )
		return
	}

	user_id , redeem_err := models.RedeemLoginToken( handlers.Store , credential )
	if redeem_err != nil {
		// Expired, already spent, never existed, or wrong secret all land
		// here with the same answer. Telling them apart would let someone
		// probe which links are real.
		err = c.Redirect().Status( fiber.StatusSeeOther ).To( "/?login=invalid" )
		return
	}

	if session_err := handlers.startSession( c , user_id ); session_err != nil {
		err = serverError( c )
		return
	}

	err = c.Redirect().Status( fiber.StatusSeeOther ).To( "/" )
	return
}

// startSession signs the browser in as user_id: a session record plus the
// cookie that carries it. Login links, join-link sign-ups and sign-up links
// all end here, so a session is made exactly one way.
func ( handlers *Handlers ) startSession( c fiber.Ctx , user_id uint64 ) ( err error ) {
	credential , _ , err := models.CreateSession( handlers.Store , user_id , handlers.Config.SessionTTL )
	if err != nil { return }
	err = handlers.Guard.SetSessionCookie( c , credential )
	return
}
