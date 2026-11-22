package routes

import (
	sync "sync"
	time "time"

	fiber "github.com/gofiber/fiber/v3"

	apidoc "rook/server/apidoc"
	security "rook/server/security"
)

// Chat from a watch link. Anyone with the link may watch, and so anyone may
// talk: someone signed in chats under their own name (and their session's
// CSRF token, like any other post), someone who is not under a name they
// typed, marked as a guest.

type watchChatRequest struct {
	Name      string `json:"name,omitempty"`
	Text      string `json:"text,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
}

// guestChatGuard slows down guests, who have no account to answer for what
// they post: one line every couple of seconds and a few dozen a minute per
// address, on top of the app-wide rate limit. In memory, like pinGuard.
type guestChatGuard struct {
	mu   sync.Mutex
	sent map[ string ][]time.Time
}

var guestChats = &guestChatGuard{ sent: map[ string ][]time.Time{} }

const (
	guestChatGap       = 2 * time.Second
	guestChatPerMinute = 20
)

func ( guard *guestChatGuard ) allow( address string ) ( ok bool ) {
	guard.mu.Lock()
	defer guard.mu.Unlock()
	now := time.Now()
	recent := []time.Time{}
	for _ , at := range guard.sent[ address ] {
		if now.Sub( at ) < time.Minute { recent = append( recent , at ) }
	}
	if len( recent ) >= guestChatPerMinute || ( len( recent ) > 0 && now.Sub( recent[ len( recent )-1 ] ) < guestChatGap ) {
		guard.sent[ address ] = recent
		return
	}
	guard.sent[ address ] = append( recent , now )
	// Forget quiet addresses now and then, so the map cannot grow forever.
	if len( guard.sent ) > 4096 {
		for key , times := range guard.sent {
			if len( times ) == 0 || now.Sub( times[ len( times )-1 ] ) >= time.Minute { delete( guard.sent , key ) }
		}
	}
	ok = true
	return
}

// @route("POST /api/watch/{code}/chat", summary="Watch chat", tag="Watch", access=PUBLIC,
// )
func ( handlers *Handlers ) WatchChat( c fiber.Ctx , in *watchChatIn ) ( err error ) {
	if security.SameOrigin( c ) == false {
		err = forbidden( c , "cross-site request" )
		return
	}
	body := in.watchChatRequest
	id , code_err := handlers.Hub.ByCode( c.Params( "code" ) , true )
	if code_err != nil {
		err = handlers.tableError( c , code_err )
		return
	}
	if user := security.UserFrom( c ); user != nil {
		if handlers.Guard.CheckCSRF( c , csrfFromBody( c ) ) == false {
			err = forbidden( c , "invalid csrf token" )
			return
		}
		err = handlers.tableResult( c , id , handlers.Hub.Chat( id , user , body.Text ) )
		return
	}
	if guestChats.allow( c.IP() ) == false {
		err = apidoc.Refuse( c , fiber.StatusTooManyRequests , "chat_too_fast" , "" )
		return
	}
	err = handlers.tableResult( c , id , handlers.Hub.GuestChat( id , body.Name , body.Text ) )
	return
}
