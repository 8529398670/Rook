package routes

import (
	bufio "bufio"
	json "encoding/json"
	strings "strings"
	time "time"

	fiber "github.com/gofiber/fiber/v3"

	security "rook/server/security"
	tables "rook/server/tables"
)

// Live updates are Server-Sent Events: a plain GET the browser keeps open,
// read with EventSource. Compared with a WebSocket that means no extra
// dependency, the session cookie authenticates it like any other request, and
// actions stay ordinary POSTs with CSRF tokens.
//
// Every event carries a complete snapshot, never a delta. A phone that drops
// off the network and comes back reads one fresh snapshot and is exactly in
// sync -- there is no history of messages for it to have missed.
//
// Each stream ends itself after EventStreamLifetime. fasthttp's WriteTimeout
// covers a whole streamed response, so a stream that tried to live forever
// would be cut off mid-write; ending it cleanly first lets EventSource
// reconnect (after the "retry" below) without the browser ever seeing an
// error.

const heartbeatInterval = 10 * time.Second

func prepareStream( c fiber.Ctx ) {
	c.Set( fiber.HeaderContentType , "text/event-stream" )
	c.Set( fiber.HeaderCacheControl , "no-store" )
	c.Set( fiber.HeaderConnection , "keep-alive" )
	// Tell nginx-style proxies not to buffer, or events arrive in clumps.
	c.Set( "X-Accel-Buffering" , "no" )
}

func writeEvent( writer *bufio.Writer , name string , payload any ) ( ok bool ) {
	data , err := json.Marshal( payload )
	if err != nil { return }
	writer.WriteString( "event: " + name + "\ndata: " )
	writer.Write( data )
	writer.WriteString( "\n\n" )
	ok = writer.Flush() == nil
	return
}

// streamLoop sends one snapshot immediately, then another each time the
// doorbell rings, until the client leaves, the lifetime runs out, or the
// server shuts down. send returns false to end the stream.
func streamLoop( writer *bufio.Writer , hub *tables.Hub , subscriber *tables.Subscriber , lifetime time.Duration , send func() bool ) {
	writer.WriteString( "retry: 1000\n\n" )
	if send() == false { return }

	deadline := time.NewTimer( lifetime )
	defer deadline.Stop()
	heartbeat := time.NewTicker( heartbeatInterval )
	defer heartbeat.Stop()

	for {
		select {
		case <-subscriber.Notify:
			if send() == false { return }
		case <-heartbeat.C:
			// A comment line: keeps idle proxies from closing the connection
			// and is how a vanished client is noticed (the flush fails).
			writer.WriteString( ": ping\n\n" )
			if writer.Flush() != nil { return }
		case <-deadline.C:
			return
		case <-hub.Done():
			return
		}
	}
}

func ( handlers *Handlers ) TableEvents( c fiber.Ctx ) ( err error ) {
	err = handlers.streamTable( c , c.Params( "table_id" ) )
	return
}

// WatchEvents is the same stream for a watch link, open to anyone who has
// the link -- signed in or not.
func ( handlers *Handlers ) WatchEvents( c fiber.Ctx ) ( err error ) {
	id , code_err := handlers.Hub.ByCode( c.Params( "code" ) , true )
	if code_err != nil {
		err = handlers.tableError( c , code_err )
		return
	}
	err = handlers.streamTable( c , id )
	return
}

// GetWatch is a watch link's first snapshot.
func ( handlers *Handlers ) GetWatch( c fiber.Ctx ) ( err error ) {
	id , code_err := handlers.Hub.ByCode( c.Params( "code" ) , true )
	if code_err != nil {
		err = handlers.tableError( c , code_err )
		return
	}
	err = handlers.tableResult( c , id , nil )
	return
}

// GetWatchRecord is the game log for a watch link.
func ( handlers *Handlers ) GetWatchRecord( c fiber.Ctx ) ( err error ) {
	id , code_err := handlers.Hub.ByCode( c.Params( "code" ) , true )
	if code_err != nil {
		err = handlers.tableError( c , code_err )
		return
	}
	record , record_err := handlers.Hub.Record( id )
	if record_err != nil {
		err = handlers.tableError( c , record_err )
		return
	}
	err = c.JSON( record )
	return
}

func ( handlers *Handlers ) streamTable( c fiber.Ctx , id string ) ( err error ) {
	// Copy what the stream needs now: c is recycled once this handler
	// returns, long before the stream writer finishes. That includes the
	// table id -- a route param is a view into c's own path buffer, which the
	// next request to reuse c overwrites, and the stream would then look up
	// a table that does not exist and tell the player it closed.
	id = strings.Clone( id )
	viewer := viewerFrom( c )
	if viewer.User != nil {
		copied := *viewer.User
		viewer.User = &copied
	}
	options := viewOptions( c )
	user_id := uint64( 0 )
	if viewer.User != nil { user_id = viewer.User.ID }
	hub := handlers.Hub
	lifetime := handlers.Config.EventStreamLifetime

	subscriber , subscribe_err := hub.SubscribeTable( id , user_id , viewer.Admin )
	if subscribe_err != nil {
		err = handlers.tableError( c , subscribe_err )
		return
	}

	prepareStream( c )
	err = c.SendStreamWriter( func( writer *bufio.Writer ) {
		defer hub.UnsubscribeTable( id , subscriber )
		streamLoop( writer , hub , subscriber , lifetime , func() ( ok bool ) {
			view , view_err := hub.View( id , viewer , options )
			if view_err != nil {
				// The table is gone (everyone left, or it was purged). Say so
				// once so the page can go back to the lobby instead of
				// reconnecting forever.
				writeEvent( writer , "gone" , fiber.Map{ "id": id } )
				return
			}
			ok = writeEvent( writer , "table" , view )
			return
		} )
	} )
	return
}

func ( handlers *Handlers ) LobbyEvents( c fiber.Ctx ) ( err error ) {
	user := *security.UserFrom( c )
	hub := handlers.Hub
	lifetime := handlers.Config.EventStreamLifetime
	subscriber := hub.SubscribeLobby( user.ID )

	prepareStream( c )
	err = c.SendStreamWriter( func( writer *bufio.Writer ) {
		defer hub.UnsubscribeLobby( subscriber )
		streamLoop( writer , hub , subscriber , lifetime , func() bool {
			return writeEvent( writer , "lobby" , hub.List( &user ) )
		} )
	} )
	return
}
