package tables

import (
	log "log"
	sort "sort"
	strings "strings"
	time "time"

	models "rook/server/models"
)

// Direct messages: a private conversation between two people, shown as a
// tab of its own beside the global chat. They travel on the global stream --
// each person's snapshot carries their own conversations -- so a message
// reaches every page the recipient has open, at a table or in the lobby.
// Conversations are kept whole (the last maxDMLines lines), so they are
// still there after a reload or on another device, and a message sent to
// someone offline is waiting when they next sign in.

const (
	maxDMLines  = 100
	viewDMLines = 50
)

// DMView is one conversation as one of its two people sees it.
type DMView struct {
	UserID      uint64          `json:"user_id"`
	DisplayName string          `json:"display_name"`
	Icon        string          `json:"icon,omitempty"`
	Color       string          `json:"color,omitempty"`
	Online      bool            `json:"online"`
	Lines       []models.DMLine `json:"lines"`
	// Read is the last line this person has seen; Unread counts the other
	// person's lines after it.
	Read   int `json:"read"`
	Unread int `json:"unread"`
}

// ringUsers wakes every global stream the given people have open.
func ( hub *Hub ) ringUsers( user_ids ...uint64 ) {
	if len( user_ids ) == 0 { return }
	wanted := map[ uint64 ]bool{}
	for _ , user_id := range user_ids { wanted[ user_id ] = true }
	hub.global_mu.Lock()
	ringing := []*Subscriber{}
	for subscriber := range hub.global {
		if wanted[ subscriber.UserID ] { ringing = append( ringing , subscriber ) }
	}
	hub.global_mu.Unlock()
	for _ , subscriber := range ringing {
		subscriber.ring()
	}
}

// dmPartner checks the person on the other end: a real, enabled account
// that is not the sender.
func ( hub *Hub ) dmPartner( user *models.User , other_id uint64 ) ( other *models.User , err error ) {
	if other_id == user.ID {
		err = ErrMessageSelf
		return
	}
	other , get_err := models.GetUser( hub.store , other_id )
	if get_err != nil || other == nil || other.Disabled() {
		other , err = nil , ErrNoSuchUser
	}
	return
}

// conversation finds or starts the pair's conversation. Called with
// global_mu held.
func ( hub *Hub ) conversation( a uint64 , b uint64 ) ( conversation *models.DMConversation ) {
	key := models.DMKey( a , b )
	conversation = hub.dms[ key ]
	if conversation == nil {
		if b < a { a , b = b , a }
		conversation = &models.DMConversation{
			Users: [2]uint64{ a , b },
			Lines: []models.DMLine{},
			Read:  map[ uint64 ]int{},
			Open:  map[ uint64 ]bool{},
		}
		hub.dms[ key ] = conversation
	}
	return
}

// saveDM is called with global_mu held, so two messages can never be
// written out of order. A failed write is logged: the line is in memory and
// the next change writes the whole conversation again.
func ( hub *Hub ) saveDM( conversation *models.DMConversation ) {
	if err := models.SaveDM( hub.store , conversation ); err != nil {
		log.Printf( "[tables] could not save a direct message conversation: %v" , err )
	}
}

// SendDM adds a line to the conversation with other_id and opens its tab
// for both of them.
func ( hub *Hub ) SendDM( user *models.User , other_id uint64 , text string ) ( err error ) {
	text , ok := validChat( text )
	if ok == false {
		err = ErrBadChat
		return
	}
	if _ , err = hub.dmPartner( user , other_id ); err != nil { return }
	hub.global_mu.Lock()
	conversation := hub.conversation( user.ID , other_id )
	conversation.Seq++
	conversation.Lines = append( conversation.Lines , models.DMLine{
		Seq: conversation.Seq , UserID: user.ID , Name: user.DisplayName , Text: text , At: time.Now().UTC() ,
	} )
	if len( conversation.Lines ) > maxDMLines {
		conversation.Lines = append( []models.DMLine( nil ) , conversation.Lines[ len( conversation.Lines )-maxDMLines: ]... )
	}
	conversation.Read[ user.ID ] = conversation.Seq
	conversation.Open[ user.ID ] , conversation.Open[ other_id ] = true , true
	hub.saveDM( conversation )
	hub.global_mu.Unlock()
	hub.ringUsers( user.ID , other_id )
	return
}

// OpenDM opens the tab for a conversation with other_id, starting one if
// they have never talked.
func ( hub *Hub ) OpenDM( user *models.User , other_id uint64 ) ( err error ) {
	if _ , err = hub.dmPartner( user , other_id ); err != nil { return }
	hub.global_mu.Lock()
	conversation := hub.conversation( user.ID , other_id )
	conversation.Open[ user.ID ] = true
	hub.saveDM( conversation )
	hub.global_mu.Unlock()
	hub.ringUsers( user.ID )
	return
}

// CloseDM closes this person's tab and counts everything in it as read. The
// conversation stays: it opens again with the next message either way.
func ( hub *Hub ) CloseDM( user *models.User , other_id uint64 ) ( err error ) {
	hub.global_mu.Lock()
	conversation := hub.dms[ models.DMKey( user.ID , other_id ) ]
	if conversation != nil {
		conversation.Open[ user.ID ] = false
		conversation.Read[ user.ID ] = conversation.Seq
		hub.saveDM( conversation )
	}
	hub.global_mu.Unlock()
	hub.ringUsers( user.ID )
	return
}

// ReadDM marks the conversation read up to seq, for every page the person
// has open.
func ( hub *Hub ) ReadDM( user *models.User , other_id uint64 , seq int ) ( err error ) {
	hub.global_mu.Lock()
	conversation := hub.dms[ models.DMKey( user.ID , other_id ) ]
	changed := false
	if conversation != nil {
		if seq > conversation.Seq { seq = conversation.Seq }
		if seq > conversation.Read[ user.ID ] {
			conversation.Read[ user.ID ] = seq
			hub.saveDM( conversation )
			changed = true
		}
	}
	hub.global_mu.Unlock()
	if changed { hub.ringUsers( user.ID ) }
	return
}

// changeDM runs change on line seq of the conversation, which must be the
// caller's own line, then saves and rings both people.
func ( hub *Hub ) changeDM( user *models.User , other_id uint64 , seq int , change func( conversation *models.DMConversation , index int ) ) ( err error ) {
	hub.global_mu.Lock()
	conversation := hub.dms[ models.DMKey( user.ID , other_id ) ]
	index := -1
	if conversation != nil {
		for candidate , line := range conversation.Lines {
			if line.Seq == seq {
				index = candidate
				break
			}
		}
	}
	switch {
	case index < 0:
		err = ErrNoSuchMessage
	case conversation.Lines[ index ].UserID != user.ID:
		err = ErrNotYourMessage
	}
	if err != nil {
		hub.global_mu.Unlock()
		return
	}
	change( conversation , index )
	hub.saveDM( conversation )
	hub.global_mu.Unlock()
	hub.ringUsers( user.ID , other_id )
	return
}

func ( hub *Hub ) EditDM( user *models.User , other_id uint64 , seq int , text string ) ( err error ) {
	text , ok := validChat( text )
	if ok == false {
		err = ErrBadChat
		return
	}
	err = hub.changeDM( user , other_id , seq , func( conversation *models.DMConversation , index int ) {
		conversation.Lines[ index ].Text , conversation.Lines[ index ].Edited = text , true
	} )
	return
}

func ( hub *Hub ) DeleteDM( user *models.User , other_id uint64 , seq int ) ( err error ) {
	err = hub.changeDM( user , other_id , seq , func( conversation *models.DMConversation , index int ) {
		conversation.Lines = append( conversation.Lines[ :index:index ] , conversation.Lines[ index+1: ]... )
	} )
	return
}

// forgetDMs drops a deleted account's conversations.
func ( hub *Hub ) forgetDMs( user_id uint64 ) {
	hub.global_mu.Lock()
	others := []uint64{}
	for key , conversation := range hub.dms {
		if conversation.Has( user_id ) == false { continue }
		others = append( others , conversation.Other( user_id ) )
		delete( hub.dms , key )
		if err := models.DeleteDM( hub.store , key ); err != nil {
			log.Printf( "[tables] could not delete a direct message conversation: %v" , err )
		}
	}
	hub.global_mu.Unlock()
	hub.ringUsers( others... )
}

// dmViews is user_id's open conversations, by the other person's name.
// online is who is on the online list now.
func ( hub *Hub ) dmViews( user_id uint64 , online map[ uint64 ]bool ) ( views []DMView ) {
	views = []DMView{}
	type open struct {
		other uint64
		lines []models.DMLine
		read  int
	}
	opened := []open{}
	hub.global_mu.Lock()
	for _ , conversation := range hub.dms {
		if conversation.Has( user_id ) == false || conversation.Open[ user_id ] == false { continue }
		lines := conversation.Lines
		if len( lines ) > viewDMLines { lines = lines[ len( lines )-viewDMLines: ] }
		opened = append( opened , open{ other: conversation.Other( user_id ) , lines: append( []models.DMLine{} , lines... ) , read: conversation.Read[ user_id ] } )
	}
	hub.global_mu.Unlock()

	for _ , entry := range opened {
		other , err := models.GetUser( hub.store , entry.other )
		if err != nil || other == nil || other.Disabled() { continue }
		view := DMView{
			UserID: other.ID , DisplayName: other.DisplayName , Icon: other.Icon , Color: other.Color ,
			Online: online[ other.ID ] , Lines: entry.lines , Read: entry.read ,
		}
		for _ , line := range entry.lines {
			if line.UserID != user_id && line.Seq > entry.read { view.Unread++ }
		}
		views = append( views , view )
	}
	sort.Slice( views , func( a , b int ) bool {
		return strings.ToLower( views[ a ].DisplayName ) < strings.ToLower( views[ b ].DisplayName )
	} )
	return
}
