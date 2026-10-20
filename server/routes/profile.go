package routes

import (
	fiber "github.com/gofiber/fiber/v3"

	game "rook/server/game"
	models "rook/server/models"
	security "rook/server/security"
)

// A player's own look, sign-in PIN, and the level of the bots they add.

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

type botLevelRequest struct {
	Level     string `json:"level"`
	CSRFToken string `json:"csrf_token"`
}

// botLevelOf is the level the Add bot picker starts on for user.
func botLevelOf( user *models.User ) ( level string ) {
	parsed , _ := game.ParseLevel( user.BotLevel )
	level = string( parsed )
	return
}

// SetBotLevel saves the player's default level for the bots they add.
func ( handlers *Handlers ) SetBotLevel( c fiber.Ctx ) ( err error ) {
	var body botLevelRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	if _ , ok := game.ParseLevel( body.Level ); ok == false || body.Level == "" {
		err = badRequest( c , string( game.ErrBadLevel ) )
		return
	}
	user := security.UserFrom( c )
	if models.SetUserBotLevel( handlers.Store , user.ID , body.Level ) != nil {
		err = serverError( c )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true , "bot_level": body.Level } )
	return
}

// SetAutoPlayLevel saves the level auto play plays the player's cards at. A
// table where their auto play is already on switches over at their next move.
func ( handlers *Handlers ) SetAutoPlayLevel( c fiber.Ctx ) ( err error ) {
	var body botLevelRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
	if _ , ok := game.ParseLevel( body.Level ); ok == false || body.Level == "" {
		err = badRequest( c , string( game.ErrBadLevel ) )
		return
	}
	user := security.UserFrom( c )
	if models.SetUserAutoPlayLevel( handlers.Store , user.ID , body.Level ) != nil {
		err = serverError( c )
		return
	}
	handlers.refreshTables( user.ID )
	err = c.JSON( fiber.Map{ "ok": true , "auto_play_level": body.Level } )
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
