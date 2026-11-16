package routes

import (
	bufio "bufio"
	strconv "strconv"

	fiber "github.com/gofiber/fiber/v3"

	apidoc "rook/server/apidoc"

	security "rook/server/security"
)

// The global chat and the "online now" list: one snapshot, one stream, one
// way to talk. Any signed-in player. The stream is opened by every signed-in
// page and kept for as long as it is up, which is what puts someone on the
// online list (see tables/global.go).

type globalChatRequest struct {
	Text      string `json:"text,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

// @route("GET /api/global", summary="Get global", tag="Global chat",
// )
func ( handlers *Handlers ) GetGlobal( c fiber.Ctx , in *apidoc.None ) ( err error ) {
	err = c.JSON( handlers.Hub.Global() )
	return
}

// @route("GET /api/global/events", summary="Global events", tag="Global chat",
// )
func ( handlers *Handlers ) GlobalEvents( c fiber.Ctx , in *apidoc.None ) ( err error ) {
	user_id := security.UserFrom( c ).ID
	hub := handlers.Hub
	lifetime := handlers.Config.EventStreamLifetime
	subscriber := hub.SubscribeGlobal( user_id )

	prepareStream( c )
	err = c.SendStreamWriter( func( writer *bufio.Writer ) {
		defer hub.UnsubscribeGlobal( subscriber )
		streamLoop( writer , hub , subscriber , lifetime , func() bool {
			return writeEvent( writer , "global" , hub.Global() )
		} )
	} )
	return
}

// @route("POST /api/global/chat", summary="Global chat", tag="Global chat",
// )
func ( handlers *Handlers ) GlobalChat( c fiber.Ctx , in *globalChatIn ) ( err error ) {
	body := in.globalChatRequest
	if chat_err := handlers.Hub.GlobalChat( security.UserFrom( c ) , body.Text ); chat_err != nil {
		err = handlers.tableError( c , chat_err )
		return
	}
	err = c.JSON( handlers.Hub.Global() )
	return
}

// chatSeq reads the :seq of a chat line from the path.
func chatSeq( c fiber.Ctx ) ( seq int , ok bool ) {
	seq , parse_err := strconv.Atoi( c.Params( "seq" ) )
	ok = parse_err == nil && seq > 0
	return
}

// EditGlobalChat and DeleteGlobalChat change one of the caller's own lines,
// or anyone's for the owner.
//
// @route("POST /api/global/chat/{seq}/edit", summary="Edit global chat", tag="Global chat",
// )
func ( handlers *Handlers ) EditGlobalChat( c fiber.Ctx , in *editGlobalChatIn ) ( err error ) {
	body := in.globalChatRequest
	seq , ok := chatSeq( c )
	if ok == false {
		err = badRequest( c , "no_such_message" )
		return
	}
	if chat_err := handlers.Hub.EditGlobalChat( security.UserFrom( c ) , handlers.isOwner( c ) , seq , body.Text ); chat_err != nil {
		err = handlers.tableError( c , chat_err )
		return
	}
	err = c.JSON( handlers.Hub.Global() )
	return
}

// @route("POST /api/global/chat/{seq}/delete", summary="Delete global chat", tag="Global chat",
// )
func ( handlers *Handlers ) DeleteGlobalChat( c fiber.Ctx , in *deleteGlobalChatIn ) ( err error ) {
	seq , ok := chatSeq( c )
	if ok == false {
		err = badRequest( c , "no_such_message" )
		return
	}
	if chat_err := handlers.Hub.DeleteGlobalChat( security.UserFrom( c ) , handlers.isOwner( c ) , seq ); chat_err != nil {
		err = handlers.tableError( c , chat_err )
		return
	}
	err = c.JSON( handlers.Hub.Global() )
	return
}
