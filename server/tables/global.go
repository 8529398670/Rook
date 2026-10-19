package tables

import (
	log "log"
	sort "sort"
	strings "strings"
	time "time"

	models "rook/server/models"
)

// The global chat is one conversation shared by everyone signed in, at any
// table or none. Its stream is also how the app knows who is online: every
// signed-in page keeps one open for as long as it is up, so "has a global
// stream open" is "has the app open".

const (
	maxGlobalLines  = 100
	viewGlobalLines = 50
)

// OnlineUser is one person on the "online now" list.
type OnlineUser struct {
	ID          uint64 `json:"id"`
	DisplayName string `json:"display_name"`
	Icon        string `json:"icon,omitempty"`
	Color       string `json:"color,omitempty"`
}

// GlobalView is the whole snapshot a global stream sends: like a table's
// view, never a delta.
type GlobalView struct {
	Enabled bool                    `json:"enabled"`
	Chat    []models.GlobalChatLine `json:"chat"`
	Online  []OnlineUser            `json:"online"`
}

// SubscribeGlobal opens a global stream. The first one a person opens brings
// them online, and everyone's list is rung.
func ( hub *Hub ) SubscribeGlobal( user_id uint64 ) ( subscriber *Subscriber ) {
	subscriber = newSubscriber( user_id , false )
	hub.global_mu.Lock()
	hub.global[ subscriber ] = struct{}{}
	hub.presence[ user_id ]++
	arrived := hub.presence[ user_id ] == 1 && hub.wasOnline( user_id ) == false
	others := hub.globalList()
	hub.global_mu.Unlock()
	if arrived {
		for _ , other := range others {
			other.ring()
		}
	}
	return
}

// UnsubscribeGlobal closes a global stream. As with a table, a person whose
// last stream closed stays online for onlineGrace -- every stream ends on
// purpose every few seconds and reconnects -- and only then is everyone rung.
func ( hub *Hub ) UnsubscribeGlobal( subscriber *Subscriber ) {
	hub.global_mu.Lock()
	delete( hub.global , subscriber )
	user_id := subscriber.UserID
	hub.presence[ user_id ]--
	if hub.presence[ user_id ] <= 0 {
		delete( hub.presence , user_id )
		hub.presence_left[ user_id ] = time.Now()
	}
	hub.global_mu.Unlock()
	time.AfterFunc( onlineGrace + 100*time.Millisecond , func() {
		hub.global_mu.Lock()
		if left , ok := hub.presence_left[ user_id ]; ok && time.Since( left ) >= onlineGrace {
			delete( hub.presence_left , user_id )
		}
		others := hub.globalList()
		hub.global_mu.Unlock()
		for _ , other := range others {
			other.ring()
		}
	} )
}

// wasOnline is true inside someone's grace period. Called with global_mu
// held.
func ( hub *Hub ) wasOnline( user_id uint64 ) ( result bool ) {
	left , ok := hub.presence_left[ user_id ]
	result = ok && time.Since( left ) < onlineGrace
	return
}

// globalList copies the global streams so they can be rung after the lock is
// released. Called with global_mu held.
func ( hub *Hub ) globalList() ( result []*Subscriber ) {
	result = make( []*Subscriber , 0 , len( hub.global ) )
	for subscriber := range hub.global {
		result = append( result , subscriber )
	}
	return
}

func ( hub *Hub ) notifyGlobal() {
	hub.global_mu.Lock()
	others := hub.globalList()
	hub.global_mu.Unlock()
	for _ , other := range others {
		other.ring()
	}
}

// GlobalChat adds a line to the shared chat. Plain text, rendered with
// textContent, like a table's chat.
func ( hub *Hub ) GlobalChat( user *models.User , text string ) ( err error ) {
	if hub.Settings().GlobalChatOff {
		err = ErrChatOff
		return
	}
	text , ok := validChat( text )
	if ok == false {
		err = ErrBadChat
		return
	}
	hub.global_mu.Lock()
	hub.global_seq++
	hub.global_chat = append( hub.global_chat , models.GlobalChatLine{
		Seq: hub.global_seq , UserID: user.ID , Name: user.DisplayName , Icon: user.Icon , Color: user.Color ,
		Text: text , At: time.Now().UTC() ,
	} )
	if len( hub.global_chat ) > maxGlobalLines {
		hub.global_chat = append( []models.GlobalChatLine( nil ) , hub.global_chat[ len( hub.global_chat )-maxGlobalLines: ]... )
	}
	// Saved under the lock so two messages can never be written out of
	// order. A failed write is logged, as with a table: the line is already
	// in memory and the next message writes the whole list again.
	if save_err := models.SaveGlobalChat( hub.store , hub.global_chat ); save_err != nil {
		log.Printf( "[tables] could not save the global chat: %v" , save_err )
	}
	others := hub.globalList()
	hub.global_mu.Unlock()
	for _ , other := range others {
		other.ring()
	}
	return
}

// Global is the snapshot for a global stream. Names and avatars on the
// online list are read fresh, so a rename shows at once; a disabled or
// deleted account drops off.
func ( hub *Hub ) Global() ( view GlobalView ) {
	view = GlobalView{
		Enabled: hub.Settings().GlobalChatOff == false,
		Chat:    []models.GlobalChatLine{},
		Online:  []OnlineUser{},
	}
	hub.global_mu.Lock()
	chat := hub.global_chat
	if len( chat ) > viewGlobalLines { chat = chat[ len( chat )-viewGlobalLines: ] }
	view.Chat = append( view.Chat , chat... )
	ids := []uint64{}
	for user_id := range hub.presence {
		ids = append( ids , user_id )
	}
	for user_id := range hub.presence_left {
		if _ , open := hub.presence[ user_id ]; open == false && hub.wasOnline( user_id ) { ids = append( ids , user_id ) }
	}
	hub.global_mu.Unlock()

	for _ , user_id := range ids {
		user , err := models.GetUser( hub.store , user_id )
		if err != nil || user.Disabled() { continue }
		view.Online = append( view.Online , OnlineUser{ ID: user.ID , DisplayName: user.DisplayName , Icon: user.Icon , Color: user.Color } )
	}
	sort.Slice( view.Online , func( a , b int ) bool {
		return strings.ToLower( view.Online[ a ].DisplayName ) < strings.ToLower( view.Online[ b ].DisplayName )
	} )
	return
}
