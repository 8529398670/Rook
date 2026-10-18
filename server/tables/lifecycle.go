package tables

import (
	models "rook/server/models"
)

// Restarting and deleting a table: the host's call, or an admin's. Both are
// final -- a restart throws away the game's history along with its scores,
// so there is nothing for undo to step back into.

func ( table *Table ) mayManage( user *models.User , admin bool ) ( err error ) {
	if admin == false && ( user == nil || table.HostID != user.ID ) {
		err = ErrNotHost
	}
	return
}

// Restart begins the game again at the same table with the same people:
// scores back to zero and, if every seat is filled, a fresh deal at once.
// With a seat empty it goes back to the waiting room instead, where the host
// deals once the table is full.
func ( hub *Hub ) Restart( id string , user *models.User , admin bool ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if fn_err = table.mayManage( user , admin ); fn_err != nil { return }
		seat := 0
		if user != nil {
			if mine := table.seatOf( user.ID ); mine >= 0 { seat = mine }
		}
		first_bidder := table.firstBidder( seat )
		// The game being thrown away goes into the history first; with the
		// log starting over, the next game's place in it says nothing about
		// the games before.
		hub.archiveAbandoned( table )
		table.Archives = nil
		table.Game , table.Base , table.Log , table.Cursor , table.Held = nil , nil , nil , 0 , false
		for index := range table.Seats {
			// Someone invited but never arrived would hold up the new game.
			table.Seats[ index ].Pending = false
		}
		if table.emptySeats() > 0 { return }
		fn_err = table.perform( Action{ Kind: ActionStart , Seat: seat , First: first_bidder } )
		return
	} )
	return
}

// Delete removes the table for everyone. Anyone still looking at it is told
// it is gone, the same way as when the last person leaves.
func ( hub *Hub ) Delete( id string , user *models.User , admin bool ) ( err error ) {
	table := hub.table( id )
	if table == nil {
		err = ErrNoSuchTable
		return
	}
	table.mu.Lock()
	if err = table.mayManage( user , admin ); err != nil {
		table.mu.Unlock()
		return
	}
	if table.bot_timer != nil {
		table.bot_timer.Stop()
		table.bot_timer = nil
	}
	hub.archiveAbandoned( table )
	subscribers := table.subscriberList()
	table.mu.Unlock()

	hub.remove( id )
	for _ , subscriber := range subscribers {
		subscriber.ring()
	}
	hub.notifyLobby()
	return
}
