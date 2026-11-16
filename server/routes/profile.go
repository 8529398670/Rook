package routes

import (
	fiber "github.com/gofiber/fiber/v3"

	apidoc "rook/server/apidoc"

	game "rook/server/game"
	models "rook/server/models"
	security "rook/server/security"
)

// A player's own look, sign-in PIN, and the level of the bots they add.

type avatarRequest struct {
	Icon      string `json:"icon,omitempty"`
	Color     string `json:"color,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

type pinRequest struct {
	Pin       string `json:"pin,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

// GetProfileOptions lists the icons and colors a player may pick from. The
// server is the authority on the list because it validates the choice.
//
// @route("GET /api/profile/options", summary="Get profile options", tag="Account",
// )
func ( handlers *Handlers ) GetProfileOptions( c fiber.Ctx , in *apidoc.None ) ( err error ) {
	err = c.JSON( fiber.Map{ "icons": models.Icons , "colors": models.Colors } )
	return
}

// refreshTables shows a renamed or re-iconed player at their tables at once.
func ( handlers *Handlers ) refreshTables( user_id uint64 ) {
	if user , get_err := models.GetUser( handlers.Store , user_id ); get_err == nil {
		handlers.Hub.RefreshUser( user )
	}
}

// @route("POST /api/account/avatar", summary="Set avatar", tag="Account",
// )
func ( handlers *Handlers ) SetAvatar( c fiber.Ctx , in *setAvatarIn ) ( err error ) {
	body := in.avatarRequest
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
	Level     string `json:"level,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

// botLevelOf is the level the Add bot picker starts on for user.
func botLevelOf( user *models.User ) ( level string ) {
	parsed , _ := game.ParseLevel( user.BotLevel )
	level = string( parsed )
	return
}

// SetBotLevel saves the player's default level for the bots they add.
//
// @route("POST /api/account/bot-level", summary="Set bot level", tag="Account",
// )
func ( handlers *Handlers ) SetBotLevel( c fiber.Ctx , in *setBotLevelIn ) ( err error ) {
	body := in.botLevelRequest
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
//
// @route("POST /api/account/auto-play-level", summary="Set auto play level", tag="Account",
// )
func ( handlers *Handlers ) SetAutoPlayLevel( c fiber.Ctx , in *setAutoPlayLevelIn ) ( err error ) {
	body := in.botLevelRequest
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

// SetHintLevel saves the level the hint button suggests at for the player.
//
// @route("POST /api/account/hint-level", summary="Set hint level", tag="Account",
// )
func ( handlers *Handlers ) SetHintLevel( c fiber.Ctx , in *setHintLevelIn ) ( err error ) {
	body := in.botLevelRequest
	if _ , ok := game.ParseLevel( body.Level ); ok == false || body.Level == "" {
		err = badRequest( c , string( game.ErrBadLevel ) )
		return
	}
	user := security.UserFrom( c )
	if models.SetUserHintLevel( handlers.Store , user.ID , body.Level ) != nil {
		err = serverError( c )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true , "hint_level": body.Level } )
	return
}

type partnerBotOutbidsRequest struct {
	On        bool   `json:"on,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

// SetPartnerBotOutbids saves whether the player's bot partner may keep
// raising them once both opponents have passed. Their open tables follow
// the change at once.
//
// @route("POST /api/account/partner-bot-outbids", summary="Set partner bot outbids", tag="Account",
// )
func ( handlers *Handlers ) SetPartnerBotOutbids( c fiber.Ctx , in *setPartnerBotOutbidsIn ) ( err error ) {
	body := in.partnerBotOutbidsRequest
	user := security.UserFrom( c )
	if models.SetUserPartnerBotOutbids( handlers.Store , user.ID , body.On ) != nil {
		err = serverError( c )
		return
	}
	handlers.refreshTables( user.ID )
	err = c.JSON( fiber.Map{ "ok": true , "partner_bot_outbids": body.On } )
	return
}

// SetPin sets, or with an empty pin removes, the PIN asked for when someone
// picks this account from a sign-up link's "did you mean...?".
//
// @route("POST /api/account/pin", summary="Set pin", tag="Account",
// )
func ( handlers *Handlers ) SetPin( c fiber.Ctx , in *setPinIn ) ( err error ) {
	body := in.pinRequest
	user := security.UserFrom( c )
	if pin_err := models.SetUserPin( handlers.Store , user.ID , body.Pin ); pin_err != nil {
		err = badRequest( c , "bad_pin" )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true , "has_pin": body.Pin != "" } )
	return
}
