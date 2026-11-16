package routes

import (
	errors "errors"
	strconv "strconv"

	fiber "github.com/gofiber/fiber/v3"

	apidoc "rook/server/apidoc"
	logs "rook/server/logs"
	models "rook/server/models"
	security "rook/server/security"
)

// Table pictures (models/backgrounds.go): upload one for your own themes,
// or as an admin for the default themes, list the ones you may use, delete
// one, and fetch its bytes for the table.

type backgroundPath struct {
	BackgroundID string `json:"background_id" path:"background_id"`
}

type deleteBackgroundIn struct {
	backgroundPath
}

type getBackgroundIn struct {
	backgroundPath
}

type listBackgroundsReply struct {
	Backgrounds []models.BackgroundRow `json:"backgrounds" jsonschema_description:"Your own pictures, then the shared ones."`
	Limit       int                 `json:"limit" jsonschema_description:"How many of your own you may keep."`
	SharedLimit int                 `json:"shared_limit" jsonschema_description:"How many shared ones there may be."`
	MaxSide     int                 `json:"max_side" jsonschema_description:"The long side, in pixels, a picture is shrunk to."`
}

type deleteBackgroundReply struct {
	Ok bool `json:"ok"`
}

// listBackgrounds is what the caller may pick from.
//
// @route("GET /api/account/backgrounds", summary="Your table pictures",
//     tag="Account", feature="appearance",
//     description="The pictures you may put on the game table in a theme: your own, then the ones an admin "
//         "shared for the default themes.",
// )
func ( handlers *Handlers ) listBackgrounds( c fiber.Ctx , in *apidoc.None ) ( reply *listBackgroundsReply , err error ) {
	user := security.UserFrom( c )
	backgrounds , err := handlers.Backgrounds.Usable( user.ID )
	if err != nil { return }
	rows := []models.BackgroundRow{}
	for _ , background := range backgrounds { rows = append( rows , background.Row( user.ID ) ) }
	reply = &listBackgroundsReply{ Backgrounds: rows , Limit: models.MaxBackgroundsPerUser ,
		SharedLimit: models.MaxBackgroundsShared , MaxSide: models.BackgroundMaxSide }
	return
}

// uploadBackground keeps a picture as the caller's own.
//
// @route("POST /api/account/backgrounds", summary="Upload a table picture",
//     tag="Account", feature="appearance", body_type="image/jpeg", reply_type="application/json", status=201,
//     errors=(400,),
//     description="The body is the picture itself: JPEG, PNG or WebP, under the request size limit. It is "
//         "stored as a JPEG at most 2560 pixels on its long side, with its metadata removed. Answers the "
//         "Background; use its id as a theme's table image.",
// )
func ( handlers *Handlers ) uploadBackground( c fiber.Ctx , in *apidoc.None ) ( err error ) {
	err = handlers.addBackground( c , false )
	return
}

// uploadSharedBackground keeps a picture any signed-in person may use: for
// the default themes.
//
// @route("POST /api/admin/backgrounds", summary="Upload a shared table picture",
//     tag="Admin", role=ADMIN, feature="appearance", body_type="image/jpeg", reply_type="application/json",
//     status=201, errors=(400,),
//     description="Like POST /api/account/backgrounds, but everyone signed in may see and use the picture: "
//         "for the default themes.",
// )
func ( handlers *Handlers ) uploadSharedBackground( c fiber.Ctx , in *apidoc.None ) ( err error ) {
	err = handlers.addBackground( c , true )
	return
}

func ( handlers *Handlers ) addBackground( c fiber.Ctx , shared bool ) ( err error ) {
	user := security.UserFrom( c )
	background , add_err := handlers.Backgrounds.Add( user.ID , shared , c.Body() )
	switch {
	case errors.Is( add_err , models.ErrBackgroundFormat ) , errors.Is( add_err , models.ErrBackgroundSize ) , errors.Is( add_err , models.ErrBackgroundLimit ):
		err = badRequest( c , add_err.Error() )
		return
	case add_err != nil:
		err = serverError( c , add_err )
		return
	}
	kind := logs.KindAuth
	if shared { kind = logs.KindAdmin }
	handlers.audit( c , kind , "uploaded a table picture" , fiber.Map{ "id": background.ID , "shared": shared ,
		"width": background.Width , "height": background.Height } )
	err = c.Status( fiber.StatusCreated ).JSON( background.Row( user.ID ) )
	return
}

// deleteBackground removes the caller's own picture, or for an admin a
// shared one.
//
// @route("POST /api/account/backgrounds/{background_id}/delete", summary="Delete a table picture",
//     tag="Account", feature="appearance", errors=(404,),
//     description="Delete one of your own pictures (an admin may delete shared ones too). Your themes that "
//         "used it, and for a shared one the default themes, go back to the look underneath.",
// )
func ( handlers *Handlers ) deleteBackground( c fiber.Ctx , in *deleteBackgroundIn ) ( reply *deleteBackgroundReply , err error ) {
	user := security.UserFrom( c )
	background , get_err := handlers.Backgrounds.Get( in.BackgroundID )
	mine := get_err == nil && background.Shared == false && background.OwnerID == user.ID
	shared := get_err == nil && background.Shared && user.Role == models.RoleAdmin
	if mine == false && shared == false {
		err = refuse( fiber.StatusNotFound , models.ErrBackgroundGone.Error() )
		return
	}
	if err = handlers.Backgrounds.Delete( background , user.ID ); err != nil { return }
	kind := logs.KindAuth
	if shared { kind = logs.KindAdmin }
	handlers.audit( c , kind , "deleted a table picture" , fiber.Map{ "id": background.ID , "shared": background.Shared } )
	reply = &deleteBackgroundReply{ Ok: true }
	return
}

// getBackground is a picture's bytes. Ids are random and never reused, so
// the browser may keep it for good.
//
// @route("GET /api/backgrounds/{background_id}", summary="A table picture",
//     tag="Account", feature="appearance", reply_type="image/jpeg", errors=(404,),
//     description="The picture's JPEG, for its owner or, for a shared one, anyone signed in.",
// )
func ( handlers *Handlers ) getBackground( c fiber.Ctx , in *getBackgroundIn ) ( err error ) {
	background , get_err := handlers.Backgrounds.Get( in.BackgroundID )
	if get_err != nil || background.CanUse( security.UserFrom( c ).ID ) == false {
		err = notFound( c , models.ErrBackgroundGone.Error() )
		return
	}
	etag := `"` + background.ID + `"`
	c.Set( fiber.HeaderCacheControl , "private, max-age=31536000, immutable" )
	c.Set( fiber.HeaderETag , etag )
	if c.Get( fiber.HeaderIfNoneMatch ) == etag {
		err = c.SendStatus( fiber.StatusNotModified )
		return
	}
	picture , read_err := handlers.Backgrounds.Bytes( background )
	if read_err != nil {
		err = serverError( c , read_err )
		return
	}
	c.Set( fiber.HeaderContentType , "image/jpeg" )
	c.Set( fiber.HeaderContentLength , strconv.Itoa( len( picture ) ) )
	err = c.Send( picture )
	return
}
