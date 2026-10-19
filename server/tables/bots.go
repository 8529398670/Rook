package tables

import (
	log "log"
	time "time"

	game "rook/server/game"
	models "rook/server/models"
)

// botAction is what the bot logic would do for seat right now: the move a
// computer player makes, and the suggestion behind a person's hint button.
// One function for both means a hint is exactly as good as a bot -- no more,
// no less.
func botAction( state *game.State , seat int ) ( action Action , ok bool ) {
	if state == nil || state.Turn != seat { return }
	action.Seat = seat
	switch state.Phase {
	case game.PhaseBidding:
		if amount := state.BotBid( seat ); amount > 0 {
			action.Kind , action.Amount = ActionBid , amount
		} else {
			action.Kind = ActionPass
		}
	case game.PhaseKitty:
		buried , trump := state.BotBury( seat )
		action.Kind , action.Trump = ActionBury , trump.String()
		for _ , card := range buried {
			action.Cards = append( action.Cards , card.ID() )
		}
	case game.PhasePlaying:
		action.Kind , action.Card = ActionPlay , state.BotPlay( seat ).ID()
	default:
		return
	}
	ok = true
	return
}

// settleBots does the bot work that needs no thinking time: saying "ready"
// after a hand. Called inside mutate with the table locked.
func ( hub *Hub ) settleBots( table *Table ) {
	state := table.Game
	if state == nil || state.Phase != game.PhaseHandOver || table.emptySeats() > 0 || table.Held { return }
	for seat , occupant := range table.Seats {
		if occupant.Kind == KindBot && table.Game.Ready[ seat ] == false {
			if err := table.perform( Action{ Kind: ActionContinue , Seat: seat , Bot: true } ); err != nil {
				log.Printf( "[tables] bot in seat %d could not continue at %s: %v" , seat , table.ID , err )
			}
		}
	}
}

// computerMoves reports whether the server makes seat's moves: a computer
// player, or a person who switched auto play on for this hand's cards.
func ( table *Table ) computerMoves( seat int ) ( result bool ) {
	result = table.Seats[ seat ].Kind == KindBot || table.autoPlays( seat )
	return
}

// scheduleBot arms a timer if a computer player is now up. The timer carries
// the version it was armed at: if anything changes before it fires -- someone
// leaves, the bot is swapped for a person, an admin steps back through the
// history -- the move is dropped instead of being made against a table that
// has moved on. Called with the table locked.
func ( hub *Hub ) scheduleBot( table *Table ) {
	if table.bot_timer != nil {
		table.bot_timer.Stop()
		table.bot_timer = nil
	}
	state := table.Game
	if state == nil || table.emptySeats() > 0 || table.Held { return }
	switch state.Phase {
	case game.PhaseBidding , game.PhaseKitty , game.PhasePlaying:
	default:
		return
	}
	if state.Turn < 0 || table.computerMoves( state.Turn ) == false { return }

	delay := hub.cfg.BotDelay
	if state.Phase == game.PhasePlaying && len( state.Trick.Plays ) == 0 && len( state.Completed ) > 0 {
		// A trick just finished: leave it on the table long enough to read
		// before a bot leads the next one -- at least the table's pause
		// plus the sweep that follows it, so the cards are gone first. A
		// bot delay of 0 asks for no waiting at all (the tests use it).
		delay *= 2
		sweep := time.Duration( table.trickPause( int( hub.trick_pause.Load() ) ) )*time.Millisecond + 400*time.Millisecond
		if delay > 0 && sweep > delay { delay = sweep }
	}
	id , version , seat := table.ID , table.Version , state.Turn
	table.bot_timer = time.AfterFunc( delay , func() {
		hub.mutate( id , func( table *Table ) ( fn_err error ) {
			if table.Version != version || table.Game == nil || table.Game.Turn != seat || table.computerMoves( seat ) == false {
				fn_err = errStale
				return
			}
			action , ok := botAction( table.Game , seat )
			if ok == false {
				fn_err = errStale
				return
			}
			action.Bot = true
			if fn_err = table.perform( action ); fn_err != nil && fn_err != ErrHeld {
				// A bot only ever picks from the engine's own legal moves, so
				// this is a bug worth hearing about, not a player mistake.
				log.Printf( "[tables] bot in seat %d failed at %s: %v" , seat , id , fn_err )
			}
			return
		} )
	} )
}

// Hint is the bot's choice for a person's seat, offered rather than made.
// The route checks the person is allowed hints; this checks it is their move.
func ( hub *Hub ) Hint( id string , user *models.User ) ( action Action , err error ) {
	table := hub.table( id )
	if table == nil {
		err = ErrNoSuchTable
		return
	}
	table.mu.Lock()
	defer table.mu.Unlock()
	seat := table.seatOf( user.ID )
	switch {
	case seat < 0:
		err = ErrNotSeated
	case table.Game == nil:
		err = ErrNoGame
	case table.Held:
		err = ErrHeld
	case table.emptySeats() > 0:
		err = ErrPaused
	default:
		var ok bool
		if action , ok = botAction( table.Game , seat ); ok == false {
			err = game.ErrNotYourTurn
		}
	}
	return
}
