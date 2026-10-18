package tables

import (
	strings "strings"
	time "time"
	utf8 "unicode/utf8"

	game "rook/server/game"
	models "rook/server/models"
)

// One method per thing a person can do at a table. Each is a small closure
// handed to mutate, so the checks live next to the action they guard and the
// bookkeeping (save, notify, bots) is never repeated.

func validSeat( seat int ) ( ok bool ) {
	ok = seat >= 0 && seat < 4
	return
}

// seated returns the user's seat, refreshing the name shown there in case
// they renamed themselves since sitting down.
func ( table *Table ) seated( user *models.User ) ( seat int , err error ) {
	seat = table.seatOf( user.ID )
	if seat < 0 {
		err = ErrNotSeated
		return
	}
	fillSeat( &table.Seats[ seat ] , user )
	table.Seats[ seat ].Pending = false
	return
}

func ( hub *Hub ) Sit( id string , user *models.User , seat int ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if validSeat( seat ) == false {
			fn_err = ErrBadSeat
			return
		}
		if table.seatOf( user.ID ) >= 0 {
			fn_err = ErrAlreadySeated
			return
		}
		if table.Seats[ seat ].Kind != KindEmpty {
			fn_err = ErrSeatTaken
			return
		}
		// Taking an empty seat mid-game means taking over that hand.
		table.Seats[ seat ] = Seat{}
		fillSeat( &table.Seats[ seat ] , user )
		return
	} )
	return
}

// Leave frees the seat. Mid-game that pauses the table rather than ending it:
// the hand stays where it is for whoever sits down next, person or bot.
func ( hub *Hub ) Leave( id string , user *models.User ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		seat := table.seatOf( user.ID )
		if seat < 0 {
			fn_err = ErrNotSeated
			return
		}
		table.Seats[ seat ] = Seat{}
		return
	} )
	return
}

// AddBot fills an empty seat with a computer player. Anyone seated may do it,
// before a game or to unpause one.
func ( hub *Hub ) AddBot( id string , user *models.User , seat int ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if _ , fn_err = table.seated( user ); fn_err != nil { return }
		if validSeat( seat ) == false {
			fn_err = ErrBadSeat
			return
		}
		if table.Seats[ seat ].Kind != KindEmpty {
			fn_err = ErrSeatTaken
			return
		}
		table.Seats[ seat ] = Seat{ Kind: KindBot , Bot: table.nextBotNumber() }
		return
	} )
	return
}

// RemoveBot frees a computer player's seat for a person. Mid-game that pauses
// the table until someone sits there.
func ( hub *Hub ) RemoveBot( id string , user *models.User , seat int ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if _ , fn_err = table.seated( user ); fn_err != nil { return }
		if validSeat( seat ) == false {
			fn_err = ErrBadSeat
			return
		}
		if table.Seats[ seat ].Kind != KindBot {
			fn_err = ErrNotABot
			return
		}
		table.Seats[ seat ] = Seat{}
		return
	} )
	return
}

// Reserve holds an empty seat for someone just invited, so the link they are
// sent leads straight to a place at this table.
func ( hub *Hub ) Reserve( id string , seat int , invitee *models.User ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if validSeat( seat ) == false {
			fn_err = ErrBadSeat
			return
		}
		if table.Seats[ seat ].Kind != KindEmpty {
			fn_err = ErrSeatTaken
			return
		}
		table.Seats[ seat ] = Seat{ Pending: true }
		fillSeat( &table.Seats[ seat ] , invitee )
		return
	} )
	return
}

// CanReserve answers the invite route's question before it creates an
// account: is that seat actually free? Creating the person first and then
// failing to seat them would leave an orphaned account behind.
func ( hub *Hub ) CanReserve( id string , seat int ) ( err error ) {
	table := hub.table( id )
	if table == nil {
		err = ErrNoSuchTable
		return
	}
	if validSeat( seat ) == false {
		err = ErrBadSeat
		return
	}
	table.mu.Lock()
	defer table.mu.Unlock()
	if table.Seats[ seat ].Kind != KindEmpty {
		err = ErrSeatTaken
	}
	return
}

// Start deals. Only the host may, and only with all four seats filled. After
// a game ends, the same call starts a rematch with the opening bid moved on.
func ( hub *Hub ) Start( id string , user *models.User ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if _ , fn_err = table.seated( user ); fn_err != nil { return }
		if table.HostID != user.ID {
			fn_err = ErrNotHost
			return
		}
		if table.gameActive() {
			fn_err = ErrGameInProgress
			return
		}
		if table.emptySeats() > 0 {
			fn_err = ErrSeatsNotFull
			return
		}
		first_bidder := 0
		if table.Game != nil {
			first_bidder = ( table.Game.BidStarter + 1 ) % 4
		}
		fn_err = table.perform( Action{ Kind: ActionStart , Seat: table.seatOf( user.ID ) , First: first_bidder } )
		return
	} )
	return
}

// play makes a move for the user's seat, refusing while paused (the hand
// cannot go on with a chair empty) or held by an admin. The move is
// recorded in the table's history, which is what undo replays.
func ( hub *Hub ) play( id string , user *models.User , action Action ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		seat , fn_err := table.seated( user )
		if fn_err != nil { return }
		if table.Game == nil {
			fn_err = ErrNoGame
			return
		}
		if table.emptySeats() > 0 {
			fn_err = ErrPaused
			return
		}
		action.Seat = seat
		fn_err = table.perform( action )
		return
	} )
	return
}

func ( hub *Hub ) Bid( id string , user *models.User , amount int ) ( err error ) {
	err = hub.play( id , user , Action{ Kind: ActionBid , Amount: amount } )
	return
}

func ( hub *Hub ) Pass( id string , user *models.User ) ( err error ) {
	err = hub.play( id , user , Action{ Kind: ActionPass } )
	return
}

func ( hub *Hub ) Bury( id string , user *models.User , card_ids []string , trump string ) ( err error ) {
	for _ , card_id := range card_ids {
		if _ , err = game.ParseCard( card_id ); err != nil { return }
	}
	if _ , ok := game.ParseColor( trump ); ok == false {
		err = game.ErrBadTrump
		return
	}
	err = hub.play( id , user , Action{ Kind: ActionBury , Cards: card_ids , Trump: trump } )
	return
}

func ( hub *Hub ) Play( id string , user *models.User , card_id string ) ( err error ) {
	if _ , err = game.ParseCard( card_id ); err != nil { return }
	err = hub.play( id , user , Action{ Kind: ActionPlay , Card: card_id } )
	return
}

func ( hub *Hub ) Continue( id string , user *models.User ) ( err error ) {
	err = hub.play( id , user , Action{ Kind: ActionContinue } )
	return
}

// Chat is plain text; the browser renders it with textContent, so nothing a
// player types can become markup.
func ( hub *Hub ) Chat( id string , user *models.User , text string ) ( err error ) {
	text = strings.TrimSpace( text )
	if text == "" || utf8.RuneCountInString( text ) > maxChatLength {
		err = ErrBadChat
		return
	}
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		seat , fn_err := table.seated( user )
		if fn_err != nil { return }
		table.addChat( seat , user.DisplayName , text )
		return
	} )
	return
}

// addRequest records (or moves) user's request to take over the bot in
// seat. Called with the table locked; it checks before it changes anything,
// so a refusal leaves the table as it was.
func ( table *Table ) addRequest( user *models.User , seat int ) ( err error ) {
	switch {
	case table.seatOf( user.ID ) >= 0:
		err = ErrAlreadySeated
		return
	case validSeat( seat ) == false:
		err = ErrBadSeat
		return
	case table.Seats[ seat ].Kind != KindBot:
		err = ErrNotABot
		return
	}
	request := SeatRequest{ UserID: user.ID , Name: user.DisplayName , Icon: user.Icon , Color: user.Color , Seat: seat , At: time.Now().UTC() }
	if index := table.requestOf( user.ID ); index >= 0 {
		table.Requests[ index ] = request
		return
	}
	if len( table.Requests ) >= maxRequests {
		err = ErrTooManyRequests
		return
	}
	table.Requests = append( table.Requests , request )
	return
}

// RequestSeat asks the host to let a watcher take over a computer player's
// seat -- at any point, mid-hand included. The bot keeps playing until the
// host answers.
func ( hub *Hub ) RequestSeat( id string , user *models.User , seat int ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		fn_err = table.addRequest( user , seat )
		return
	} )
	return
}

func ( hub *Hub ) CancelRequest( id string , user *models.User ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		index := table.requestOf( user.ID )
		if index < 0 {
			fn_err = ErrNoRequest
			return
		}
		table.dropRequest( index )
		return
	} )
	return
}

// AnswerRequest is the host's yes or no (an admin may answer too, for a host
// who has wandered off). Yes swaps the bot for the person on the spot: they
// pick up the bot's cards wherever the hand stands, and a bot move already
// on its timer is dropped because the seat is no longer a bot's.
func ( hub *Hub ) AnswerRequest( id string , caller *models.User , admin bool , requester_id uint64 , accept bool ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if admin == false && table.HostID != caller.ID {
			fn_err = ErrNotHost
			return
		}
		index := table.requestOf( requester_id )
		if index < 0 {
			fn_err = ErrNoRequest
			return
		}
		request := table.Requests[ index ]
		if accept {
			if table.Seats[ request.Seat ].Kind != KindBot {
				fn_err = ErrSeatChanged
				return
			}
			if table.seatOf( request.UserID ) >= 0 {
				fn_err = ErrAlreadySeated
				return
			}
			table.Seats[ request.Seat ] = Seat{
				Kind: KindHuman , UserID: request.UserID , Name: request.Name , Icon: request.Icon , Color: request.Color ,
			}
		}
		table.dropRequest( index )
		return
	} )
	return
}
