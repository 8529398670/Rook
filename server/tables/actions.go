package tables

import (
	rand "math/rand/v2"
	strconv "strconv"
	strings "strings"
	time "time"
	unicode "unicode"
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

// AutoPlayLevelOf is the level auto play plays user's cards at: their own
// setting, or game.HelpLevel.
func AutoPlayLevelOf( user *models.User ) ( level game.Level ) {
	level = game.HelpLevel
	if parsed , ok := game.ParseLevel( user.AutoPlayLevel ); ok && user.AutoPlayLevel != "" {
		level = parsed
	}
	return
}

// SetAutoPlay switches auto play on or off for the user's seat: while on,
// the server plays a bot's choice of card for them -- at the level they set
// -- on each of their turns until the hand ends. Only while cards are being played -- bidding and the
// kitty are real decisions.
func ( hub *Hub ) SetAutoPlay( id string , user *models.User , on bool ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if _ , fn_err = table.seated( user ); fn_err != nil { return }
		if table.Game == nil || table.Game.Phase != game.PhasePlaying {
			fn_err = ErrNotPlaying
			return
		}
		if on == false {
			delete( table.AutoPlay , user.ID )
			delete( table.AutoPlayLevel , user.ID )
			return
		}
		if table.AutoPlay == nil { table.AutoPlay = map[ uint64 ]int{} }
		if table.AutoPlayLevel == nil { table.AutoPlayLevel = map[ uint64 ]game.Level{} }
		table.AutoPlay[ user.ID ] = table.Game.HandNumber
		table.AutoPlayLevel[ user.ID ] = AutoPlayLevelOf( user )
		return
	} )
	return
}

// AddBot fills an empty seat with a computer player of the given level ("" is
// Normal). Anyone seated may do it, before a game or to unpause one.
func ( hub *Hub ) AddBot( id string , user *models.User , seat int , level string ) ( err error ) {
	err = hub.AddBotLooking( id , user , seat , level , BotLook{} )
	return
}

// BotLook is what a bot with a random look is drawn from, for a player who
// switched random bots on: a name from Names and an icon from Icons, neither
// already in use at the table, and any hue. Empty lists keep the usual name
// and the robot badge.
type BotLook struct {
	Names []string
	Icons []string
}

// pickUnused draws one of choices that is not in used, or any of them when
// every one is taken.
func pickUnused( choices []string , used map[ string ]bool ) ( result string ) {
	free := []string{}
	for _ , choice := range choices {
		if used[ choice ] == false { free = append( free , choice ) }
	}
	if len( free ) == 0 { free = choices }
	if len( free ) > 0 { result = free[ rand.IntN( len( free ) ) ] }
	return
}

// dress gives a new bot seat a random name, icon and color from look.
func ( table *Table ) dress( bot *Seat , look BotLook ) {
	names , icons := map[ string ]bool{} , map[ string ]bool{}
	for _ , seat := range table.Seats {
		names[ seat.shownName() ] = true
		icons[ seat.Icon ] = true
	}
	if len( look.Names ) > 0 { bot.Name = pickUnused( look.Names , names ) }
	if len( look.Icons ) > 0 {
		bot.Icon = pickUnused( look.Icons , icons )
		bot.Color = "h" + strconv.Itoa( rand.IntN( 360 ) )
	}
}

// AddBotLooking is AddBot with a random look (see BotLook).
func ( hub *Hub ) AddBotLooking( id string , user *models.User , seat int , level string , look BotLook ) ( err error ) {
	parsed , ok := game.ParseLevel( level )
	if ok == false {
		err = game.ErrBadLevel
		return
	}
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
		bot := Seat{ Kind: KindBot , Bot: table.nextBotNumber() , Level: parsed }
		table.dress( &bot , look )
		table.Seats[ seat ] = bot
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

// RemovePlayer takes someone else -- a person or a bot -- out of their seat:
// the host's call, or an admin's, at any point in the game. A seat held for
// an invite counts too, which cancels the hold. Mid-game that pauses the
// table, exactly as if they had left; taking yourself out is what Leave is
// for.
func ( hub *Hub ) RemovePlayer( id string , caller *models.User , admin bool , seat int ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if admin == false && table.HostID != caller.ID {
			fn_err = ErrNotHost
			return
		}
		if validSeat( seat ) == false || table.seatOf( caller.ID ) == seat {
			fn_err = ErrBadSeat
			return
		}
		if table.Seats[ seat ].Kind == KindEmpty {
			fn_err = ErrNotAPlayer
			return
		}
		table.Seats[ seat ] = Seat{}
		return
	} )
	return
}

// RenameSeat gives the player in a seat -- a person or a bot -- a different
// name at this table: the host's call, or an admin's. An empty name gives
// them their own back.
func ( hub *Hub ) RenameSeat( id string , caller *models.User , admin bool , seat int , name string ) ( err error ) {
	name = strings.TrimSpace( name )
	if utf8.RuneCountInString( name ) > maxNameLength {
		err = ErrBadSeatName
		return
	}
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if admin == false && table.HostID != caller.ID {
			fn_err = ErrNotHost
			return
		}
		if validSeat( seat ) == false {
			fn_err = ErrBadSeat
			return
		}
		if table.Seats[ seat ].Kind == KindEmpty {
			fn_err = ErrNotAPlayer
			return
		}
		if name == table.Seats[ seat ].Name { name = "" }
		table.Seats[ seat ].Alias = name
		return
	} )
	return
}

// ShuffleBot gives a bot a fresh random look -- name, emoji and color --
// none of them what it or anyone else at the table has now: the host's call,
// or an admin's. A name the host gave it is dropped for the new one.
func ( hub *Hub ) ShuffleBot( id string , caller *models.User , admin bool , seat int , look BotLook ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if admin == false && table.HostID != caller.ID {
			fn_err = ErrNotHost
			return
		}
		if validSeat( seat ) == false {
			fn_err = ErrBadSeat
			return
		}
		if table.Seats[ seat ].Kind != KindBot {
			fn_err = ErrNotABot
			return
		}
		bot := table.Seats[ seat ]
		past := append( []BotDress{} , bot.Past... )
		past = append( past , BotDress{ Name: bot.Name , Alias: bot.Alias , Icon: bot.Icon , Color: bot.Color } )
		if len( past ) > maxBotPast { past = past[ len( past ) - maxBotPast : ] }
		table.dress( &bot , look )
		bot.Alias = ""
		bot.Past = past
		table.Seats[ seat ] = bot
		return
	} )
	return
}

// UnshuffleBot puts back the look a bot had before its last shuffle: the
// host's call, or an admin's. Each call steps one shuffle further back.
func ( hub *Hub ) UnshuffleBot( id string , caller *models.User , admin bool , seat int ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if admin == false && table.HostID != caller.ID {
			fn_err = ErrNotHost
			return
		}
		if validSeat( seat ) == false {
			fn_err = ErrBadSeat
			return
		}
		if table.Seats[ seat ].Kind != KindBot {
			fn_err = ErrNotABot
			return
		}
		bot := table.Seats[ seat ]
		if len( bot.Past ) == 0 {
			fn_err = ErrNothingToUndo
			return
		}
		last := bot.Past[ len( bot.Past ) - 1 ]
		bot.Past = append( []BotDress{} , bot.Past[ : len( bot.Past ) - 1 ]... )
		if len( bot.Past ) == 0 { bot.Past = nil }
		bot.Name , bot.Alias , bot.Icon , bot.Color = last.Name , last.Alias , last.Icon , last.Color
		table.Seats[ seat ] = bot
		return
	} )
	return
}

// SetBotLook picks a bot's emoji and background color by hand: the host's
// call, or an admin's. "" for either is the robot badge and the usual color.
func ( hub *Hub ) SetBotLook( id string , caller *models.User , admin bool , seat int , icon string , color string ) ( err error ) {
	if models.ValidIcon( icon ) == false || models.ValidColor( color ) == false {
		err = ErrBadLook
		return
	}
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if admin == false && table.HostID != caller.ID {
			fn_err = ErrNotHost
			return
		}
		if validSeat( seat ) == false {
			fn_err = ErrBadSeat
			return
		}
		if table.Seats[ seat ].Kind != KindBot {
			fn_err = ErrNotABot
			return
		}
		table.Seats[ seat ].Icon = icon
		table.Seats[ seat ].Color = color
		return
	} )
	return
}

// SetBotLevel changes how well a computer player plays: the host's call, or
// an admin's, at any point. Mid-hand the bot simply plays its next move at
// the new level.
func ( hub *Hub ) SetBotLevel( id string , caller *models.User , admin bool , seat int , level string ) ( err error ) {
	parsed , ok := game.ParseLevel( level )
	if ok == false || level == "" {
		err = game.ErrBadLevel
		return
	}
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if admin == false && table.HostID != caller.ID {
			fn_err = ErrNotHost
			return
		}
		if validSeat( seat ) == false {
			fn_err = ErrBadSeat
			return
		}
		if table.Seats[ seat ].Kind != KindBot {
			fn_err = ErrNotABot
			return
		}
		table.Seats[ seat ].Level = parsed
		return
	} )
	return
}

// IsHost says whether user_id hosts the table, for routes that let the host
// do something the user's own flags would not.
func ( hub *Hub ) IsHost( id string , user_id uint64 ) ( result bool ) {
	table := hub.table( id )
	if table == nil { return }
	table.mu.Lock()
	result = table.HostID == user_id
	table.mu.Unlock()
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
		if table.seatOf( invitee.ID ) >= 0 {
			fn_err = ErrAlreadySeated
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

// Who deals a new table's first hand, besides a seat number, for Start.
const (
	DealerHost   = -1 // whoever pressed Deal
	DealerRandom = -2 // a seat drawn at random
)

// Start deals. Only the host may, and only with all four seats filled. The
// host names the first dealer -- a seat, or DealerRandom -- and the deal moves
// on one seat each hand from there. After a game ends, the same call starts a
// rematch with the deal moved on, whatever dealer is passed.
func ( hub *Hub ) Start( id string , user *models.User , dealer int ) ( err error ) {
	if dealer != DealerHost && dealer != DealerRandom && validSeat( dealer ) == false {
		err = ErrBadSeat
		return
	}
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
		seat := table.seatOf( user.ID )
		switch dealer {
		case DealerHost:
			dealer = seat
		case DealerRandom:
			dealer = rand.IntN( 4 )
		}
		fn_err = table.perform( Action{ Kind: ActionStart , Seat: seat , First: table.firstBidder( dealer ) } )
		return
	} )
	return
}

// firstBidder is who opens the auction of a new game: the seat on the
// dealer's left. A first game is dealt by the dealer given; a rematch or
// restart passes the deal on one seat, the way every new hand does.
func ( table *Table ) firstBidder( dealer int ) ( seat int ) {
	if table.Game != nil {
		seat = ( table.Game.BidStarter + 1 ) % 4
		return
	}
	seat = ( dealer + 1 ) % 4
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

// Reshuffle deals the hand again, for a player who has not bid or passed in
// it yet. The route checks that they may.
func ( hub *Hub ) Reshuffle( id string , user *models.User ) ( err error ) {
	err = hub.play( id , user , Action{ Kind: ActionReshuffle } )
	return
}

func ( hub *Hub ) Continue( id string , user *models.User ) ( err error ) {
	err = hub.play( id , user , Action{ Kind: ActionContinue } )
	return
}

// Chat is plain text; the browser renders it with textContent, so nothing a
// player types can become markup. Someone signed in but not seated is
// watching, and chats as a watcher under their own name.
func ( hub *Hub ) Chat( id string , user *models.User , text string ) ( err error ) {
	if hub.Settings().TableChatOff {
		err = ErrChatOff
		return
	}
	text , ok := validChat( text )
	if ok == false {
		err = ErrBadChat
		return
	}
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		if table.seatOf( user.ID ) < 0 {
			table.addChat( ChatLine{ Seat: -1 , Name: user.DisplayName , Text: text , Watcher: true , UserID: user.ID } )
			return
		}
		seat , fn_err := table.seated( user )
		if fn_err != nil { return }
		table.addChat( ChatLine{ Seat: seat , Name: table.Seats[ seat ].shownName() , Text: text , UserID: user.ID } )
		return
	} )
	return
}

// chatLineOf finds line seq and checks it is user's own -- unless user is a
// moderator (the owner), who may change anyone's. Called with the table
// locked.
func ( table *Table ) chatLineOf( user *models.User , moderator bool , seq int ) ( index int , err error ) {
	index = -1
	for candidate , line := range table.Chat {
		if line.Seq == seq {
			index = candidate
			break
		}
	}
	switch {
	case index < 0:
		err = ErrNoSuchMessage
	case moderator:
	case table.Chat[ index ].UserID == 0 || table.Chat[ index ].UserID != user.ID:
		err = ErrNotYourMessage
	}
	return
}

// EditChat rewrites one of your own lines (anyone's, for a moderator),
// marking it edited.
func ( hub *Hub ) EditChat( id string , user *models.User , moderator bool , seq int , text string ) ( err error ) {
	if hub.Settings().TableChatOff {
		err = ErrChatOff
		return
	}
	text , ok := validChat( text )
	if ok == false {
		err = ErrBadChat
		return
	}
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		index , fn_err := table.chatLineOf( user , moderator , seq )
		if fn_err != nil { return }
		table.Chat[ index ].Text , table.Chat[ index ].Edited = text , true
		return
	} )
	return
}

// DeleteChat takes one of your own lines (anyone's, for a moderator) away
// for everyone.
func ( hub *Hub ) DeleteChat( id string , user *models.User , moderator bool , seq int ) ( err error ) {
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		index , fn_err := table.chatLineOf( user , moderator , seq )
		if fn_err != nil { return }
		table.Chat = append( table.Chat[ :index:index ] , table.Chat[ index+1: ]... )
		return
	} )
	return
}

// GuestChat is chat from someone watching through a watch link without
// signing in, under a name they typed. Their lines are marked as a guest's,
// so a guest cannot pass for a player of the same name.
func ( hub *Hub ) GuestChat( id string , name string , text string ) ( err error ) {
	if hub.Settings().TableChatOff {
		err = ErrChatOff
		return
	}
	name , ok := validGuestName( name )
	if ok == false {
		err = ErrBadGuestName
		return
	}
	if text , ok = validChat( text ); ok == false {
		err = ErrBadChat
		return
	}
	err = hub.mutate( id , func( table *Table ) ( fn_err error ) {
		table.addChat( ChatLine{ Seat: -1 , Name: name , Text: text , Watcher: true , Guest: true } )
		return
	} )
	return
}

func validChat( text string ) ( result string , ok bool ) {
	result = strings.TrimSpace( text )
	ok = result != "" && utf8.RuneCountInString( result ) <= maxChatLength
	return
}

func validGuestName( name string ) ( result string , ok bool ) {
	result = strings.Join( strings.Fields( name ) , " " )
	length := utf8.RuneCountInString( result )
	if length < 1 || length > maxGuestNameLength { return }
	for _ , character := range result {
		if unicode.IsControl( character ) || unicode.Is( unicode.Cf , character ) { return }
	}
	ok = true
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
