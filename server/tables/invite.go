package tables

import (
	game "rook/server/game"
	models "rook/server/models"
)

// Inviting someone to play, from the player menu: "Invite as partner" or
// "Invite as opponent". It holds a seat for them the way an admin's invite
// does (Seat.Pending), so everything that already handles a held seat --
// the lobby card's Take seat / Decline, the seat freeing when they leave --
// works unchanged. What it adds is who asked, so the invitee gets a notice
// on their global stream wherever they are in the app.

const (
	InviteAsPartner  = "partner"
	InviteAsOpponent = "opponent"
)

// InviteView is one notice: a seat someone is holding for you.
type InviteView struct {
	TableID   string `json:"table_id"`
	TableName string `json:"table_name"`
	FromID    uint64 `json:"from_id"`
	FromName  string `json:"from_name"`
	// As is partner or opponent of the person who asked, while they are
	// still at the table.
	As   string `json:"as,omitempty"`
	Seat int    `json:"seat"`
}

// invitees is everyone a seat is being held for, by someone at the table.
// Called with the table locked.
func ( table *Table ) invitees() ( result []uint64 ) {
	for _ , seat := range table.Seats {
		if seat.Kind == KindHuman && seat.Pending && seat.InvitedBy != 0 { result = append( result , seat.UserID ) }
	}
	return
}

// inviteesChanged is who was invited before or after a change, but not
// both.
func inviteesChanged( before []uint64 , after []uint64 ) ( result []uint64 ) {
	count := map[ uint64 ]int{}
	for _ , user_id := range before { count[ user_id ]++ }
	for _ , user_id := range after { count[ user_id ]-- }
	for user_id , difference := range count {
		if difference != 0 { result = append( result , user_id ) }
	}
	return
}

// Invite holds a seat for invitee_id beside or across from the inviter. With
// a table_id the inviter must be sitting there before the game starts, and
// be its host (or allowed to invite); without one a new table is opened
// for them, called name. A computer player in the seat wanted gives it up.
// It returns the table the seat is at.
func ( hub *Hub ) Invite( inviter *models.User , admin bool , invitee_id uint64 , as string , table_id string , name string ) ( id string , err error ) {
	if as != InviteAsPartner && as != InviteAsOpponent {
		err = ErrBadAction
		return
	}
	if invitee_id == inviter.ID {
		err = ErrInviteSelf
		return
	}
	invitee , get_err := models.GetUser( hub.store , invitee_id )
	if get_err != nil || invitee == nil || invitee.Disabled() {
		err = ErrNoSuchUser
		return
	}

	created := false
	if table_id == "" {
		if _ , ok := validTableName( name ); ok == false { name = inviter.DisplayName }
		if table_id , err = hub.Create( inviter , name , game.DefaultOptions() ); err != nil { return }
		created = true
	}

	err = hub.mutate( table_id , func( table *Table ) ( fn_err error ) {
		mine := table.seatOf( inviter.ID )
		switch {
		case mine < 0:
			fn_err = ErrNotSeated
		case table.HostID != inviter.ID && inviter.MayInvite() == false && admin == false:
			fn_err = ErrCannotInvite
		case table.Game != nil:
			fn_err = ErrGameInProgress
		case table.seatOf( invitee.ID ) >= 0:
			fn_err = ErrAlreadySeated
		}
		if fn_err != nil { return }

		candidates := []int{ ( mine + 2 ) % 4 }
		if as == InviteAsOpponent { candidates = []int{ ( mine + 1 ) % 4 , ( mine + 3 ) % 4 } }
		seat := -1
		// An empty chair first; failing that, a computer player's.
		for _ , kind := range []string{ KindEmpty , KindBot } {
			for _ , candidate := range candidates {
				if seat < 0 && table.Seats[ candidate ].Kind == kind { seat = candidate }
			}
		}
		if seat < 0 {
			fn_err = ErrNoSeatFor
			return
		}
		table.Seats[ seat ] = Seat{ Pending: true , InvitedBy: inviter.ID , InvitedByName: inviter.DisplayName }
		fillSeat( &table.Seats[ seat ] , invitee )
		return
	} )
	if err != nil && created {
		// Nothing to show for the table opened for this: take it away again.
		hub.Delete( table_id , inviter , true )
	}
	id = table_id
	return
}

// invitesFor is every seat held for user_id by someone at the table.
func ( hub *Hub ) invitesFor( user_id uint64 ) ( invites []InviteView ) {
	invites = []InviteView{}
	hub.mu.RLock()
	tables := make( []*Table , 0 , len( hub.tables ) )
	for _ , table := range hub.tables {
		tables = append( tables , table )
	}
	hub.mu.RUnlock()
	for _ , table := range tables {
		table.mu.Lock()
		for index , seat := range table.Seats {
			if seat.Kind != KindHuman || seat.UserID != user_id || seat.Pending == false || seat.InvitedBy == 0 { continue }
			invite := InviteView{ TableID: table.ID , TableName: table.Name , FromID: seat.InvitedBy , FromName: seat.InvitedByName , Seat: index }
			if from := table.seatOf( seat.InvitedBy ); from >= 0 {
				invite.As = InviteAsOpponent
				if from % 2 == index % 2 { invite.As = InviteAsPartner }
			}
			invites = append( invites , invite )
		}
		table.mu.Unlock()
	}
	return
}

// PersonalView is the global snapshot plus what is only this person's: their
// direct-message conversations and the seats held for them.
type PersonalView struct {
	GlobalView
	DMs     []DMView     `json:"dms"`
	Invites []InviteView `json:"invites"`
}

func ( hub *Hub ) GlobalFor( user_id uint64 ) ( view PersonalView ) {
	view.GlobalView = hub.Global()
	online := map[ uint64 ]bool{}
	for _ , person := range view.Online { online[ person.ID ] = true }
	view.DMs = hub.dmViews( user_id , online )
	view.Invites = hub.invitesFor( user_id )
	return
}
