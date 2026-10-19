package routes

import (
	bufio "bufio"
	strconv "strconv"

	fiber "github.com/gofiber/fiber/v3"

	security "rook/server/security"
)

// The global chat and the "online now" list: one snapshot, one stream, one
// way to talk. Any signed-in player. The stream is opened by every signed-in
// page and kept for as long as it is up, which is what puts someone on the
// online list (see tables/global.go).

type globalChatRequest struct {
	Text      string `json:"text"`
	CSRFToken string `json:"csrf_token"`
}

func ( handlers *Handlers ) GetGlobal( c fiber.Ctx ) ( err error ) {
	err = c.JSON( handlers.Hub.Global() )
	return
}

func ( handlers *Handlers ) GlobalEvents( c fiber.Ctx ) ( err error ) {
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

func ( handlers *Handlers ) GlobalChat( c fiber.Ctx ) ( err error ) {
	var body globalChatRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
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
func ( handlers *Handlers ) EditGlobalChat( c fiber.Ctx ) ( err error ) {
	var body globalChatRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
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

func ( handlers *Handlers ) DeleteGlobalChat( c fiber.Ctx ) ( err error ) {
	var body csrfOnlyRequest
	if ok , decode_err := handlers.decode( c , &body , func() string { return body.CSRFToken } ); ok == false {
		err = decode_err
		return
	}
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
