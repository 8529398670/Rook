package routes

import (
	strconv "strconv"

	fiber "github.com/gofiber/fiber/v3"

	security "rook/server/security"
)

// Direct messages and invitations to play, from the player menu. Every
// reply is the caller's global snapshot (tables.PersonalView), the same
// thing their global stream sends, so the page can draw it straight away.

type dmRequest struct {
	Text      string `json:"text,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

// dmReadRequest is how far the caller has read.
type dmReadRequest struct {
	Seq       int    `json:"seq,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

type playInviteRequest struct {
	UserID uint64 `json:"user_id,omitempty"`
	// As is "partner" or "opponent".
	As string `json:"as,omitempty"`
	// TableID is the waiting table the caller is sitting at; empty opens a
	// new one, called Name.
	TableID   string `json:"table_id,omitempty"`
	Name      string `json:"name,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

type sendDMIn struct {
	pathUserID
	dmRequest
}

type dmUserIn struct {
	pathUserID
}

type readDMIn struct {
	pathUserID
	dmReadRequest
}

type dmLineIn struct {
	pathUserIDSeq
	dmRequest
}

type deleteDMIn struct {
	pathUserIDSeq
}

type invitePlayerIn struct {
	playInviteRequest
}

// dmUser reads the :user_id of the other person from the path.
func dmUser( c fiber.Ctx ) ( user_id uint64 , ok bool ) {
	user_id , parse_err := strconv.ParseUint( c.Params( "user_id" ) , 10 , 64 )
	ok = parse_err == nil && user_id > 0
	return
}

// dmReply answers with the caller's snapshot, or the error.
func ( handlers *Handlers ) dmReply( c fiber.Ctx , action_err error ) ( err error ) {
	if action_err != nil {
		err = handlers.tableError( c , action_err )
		return
	}
	err = c.JSON( handlers.Hub.GlobalFor( security.UserFrom( c ).ID ) )
	return
}

// @route("POST /api/dm/{user_id}", summary="Send direct message", tag="Direct messages",
// )
func ( handlers *Handlers ) SendDM( c fiber.Ctx , in *sendDMIn ) ( err error ) {
	other , ok := dmUser( c )
	if ok == false {
		err = badRequest( c , "no_such_user" )
		return
	}
	err = handlers.dmReply( c , handlers.Hub.SendDM( security.UserFrom( c ) , other , in.Text ) )
	return
}

// OpenDM opens the conversation's tab, starting one if need be.
//
// @route("POST /api/dm/{user_id}/open", summary="Open direct message", tag="Direct messages",
// )
func ( handlers *Handlers ) OpenDM( c fiber.Ctx , in *dmUserIn ) ( err error ) {
	other , ok := dmUser( c )
	if ok == false {
		err = badRequest( c , "no_such_user" )
		return
	}
	err = handlers.dmReply( c , handlers.Hub.OpenDM( security.UserFrom( c ) , other ) )
	return
}

// CloseDM closes the conversation's tab; the next message opens it again.
//
// @route("POST /api/dm/{user_id}/close", summary="Close direct message", tag="Direct messages",
// )
func ( handlers *Handlers ) CloseDM( c fiber.Ctx , in *dmUserIn ) ( err error ) {
	other , ok := dmUser( c )
	if ok == false {
		err = badRequest( c , "no_such_user" )
		return
	}
	err = handlers.dmReply( c , handlers.Hub.CloseDM( security.UserFrom( c ) , other ) )
	return
}

// ReadDM marks the conversation read up to seq.
//
// @route("POST /api/dm/{user_id}/read", summary="Mark direct messages read", tag="Direct messages",
// )
func ( handlers *Handlers ) ReadDM( c fiber.Ctx , in *readDMIn ) ( err error ) {
	other , ok := dmUser( c )
	if ok == false {
		err = badRequest( c , "no_such_user" )
		return
	}
	err = handlers.dmReply( c , handlers.Hub.ReadDM( security.UserFrom( c ) , other , in.Seq ) )
	return
}

// @route("POST /api/dm/{user_id}/{seq}/edit", summary="Edit direct message", tag="Direct messages",
// )
func ( handlers *Handlers ) EditDM( c fiber.Ctx , in *dmLineIn ) ( err error ) {
	other , ok := dmUser( c )
	seq , seq_ok := chatSeq( c )
	if ok == false || seq_ok == false {
		err = badRequest( c , "no_such_message" )
		return
	}
	err = handlers.dmReply( c , handlers.Hub.EditDM( security.UserFrom( c ) , other , seq , in.Text ) )
	return
}

// @route("POST /api/dm/{user_id}/{seq}/delete", summary="Delete direct message", tag="Direct messages",
// )
func ( handlers *Handlers ) DeleteDM( c fiber.Ctx , in *deleteDMIn ) ( err error ) {
	other , ok := dmUser( c )
	seq , seq_ok := chatSeq( c )
	if ok == false || seq_ok == false {
		err = badRequest( c , "no_such_message" )
		return
	}
	err = handlers.dmReply( c , handlers.Hub.DeleteDM( security.UserFrom( c ) , other , seq ) )
	return
}

// InvitePlayer holds a seat for someone beside or across from the caller:
// at the waiting table they are sitting at, or at a new one. The invitee
// gets a notice on their global stream.
//
// @route("POST /api/invite", summary="Invite player", tag="Direct messages",
// )
func ( handlers *Handlers ) InvitePlayer( c fiber.Ctx , in *invitePlayerIn ) ( err error ) {
	body := in.playInviteRequest
	id , invite_err := handlers.Hub.Invite( security.UserFrom( c ) , viewerFrom( c ).Admin , body.UserID , body.As , body.TableID , body.Name )
	if invite_err != nil {
		err = handlers.tableError( c , invite_err )
		return
	}
	err = c.JSON( fiber.Map{ "table_id": id } )
	return
}
