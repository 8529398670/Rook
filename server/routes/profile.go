package routes

import (
	fiber "github.com/gofiber/fiber/v3"

	models "rook/server/models"
	security "rook/server/security"
)

// A player's own look and sign-in PIN.

type avatarRequest struct {
	Icon      string `json:"icon"`
	Color     string `json:"color"`
	CSRFToken string `json:"csrf_token"`
}

type pinRequest struct {
	Pin       string `json:"pin"`
	CSRFToken string `json:"csrf_token"`
}

// GetProfileOptions lists the icons and colors a player may pick from. The
// server is the authority on the list because it validates the choice.
func ( handlers *Handlers ) GetProfileOptions( c fiber.Ctx ) ( err error ) {
	err = c.JSON( fiber.Map{ "icons": models.Icons , "colors": models.Colors } )
	return
}

// refreshTables shows a renamed or re-iconed player at their tables at once.
func ( handlers *Handlers ) refreshTables( user_id uint64 ) {
	if user , get_err := models.GetUser( handlers.Store , user_id ); get_err == nil {
		handlers.Hub.RefreshUser( user )
	}
}

func ( handlers *Handlers ) SetAvatar( c fiber.Ctx ) ( err error ) {
	var body avatarRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	user := security.UserFrom( c )
	if models.SetUserAvatar( handlers.Store , user.ID , body.Icon , body.Color ) != nil {
		err = badRequest( c , "bad_avatar" )
		return
	}
	handlers.refreshTables( user.ID )
	err = c.JSON( fiber.Map{ "ok": true , "icon": body.Icon , "color": body.Color } )
	return
}

// SetPin sets, or with an empty pin removes, the PIN asked for when someone
// picks this account from a sign-up link's "did you mean...?".
func ( handlers *Handlers ) SetPin( c fiber.Ctx ) ( err error ) {
	var body pinRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	user := security.UserFrom( c )
	if pin_err := models.SetUserPin( handlers.Store , user.ID , body.Pin ); pin_err != nil {
		err = badRequest( c , "bad_pin" )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true , "has_pin": body.Pin != "" } )
	return
}
