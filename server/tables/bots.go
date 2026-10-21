package tables

import (
	log "log"
	time "time"

	game "rook/server/game"
	models "rook/server/models"
)

// botAction is what a bot of the given level would do for seat right now:
// the move a computer player makes, and -- at game.HelpLevel -- the
// suggestion behind a person's hint button and the move auto play makes.
// Expert and Master search, which takes tens of milliseconds, so callers
// hand it a clone of the game and call it without the table locked.
func botAction( state *game.State , seat int , level game.Level ) ( action Action , ok bool ) {
	if state == nil || state.Turn != seat { return }
	action.Seat = seat
	switch state.Phase {
	case game.PhaseBidding:
		if amount := state.BotBid( seat , level ); amount > 0 {
			action.Kind , action.Amount = ActionBid , amount
		} else {
			action.Kind = ActionPass
		}
	case game.PhaseKitty:
		buried , trump := state.BotBury( seat , level )
		action.Kind , action.Trump = ActionBury , trump.String()
		for _ , card := range buried {
			action.Cards = append( action.Cards , card.ID() )
		}
	case game.PhasePlaying:
		action.Kind , action.Card = ActionPlay , state.BotPlay( seat , level ).ID()
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

// moveLevel is how well the server plays seat's moves: the bot's own level,
// or, for a person on auto play, the level they chose for it.
func ( table *Table ) moveLevel( seat int ) ( level game.Level ) {
	level = game.HelpLevel
	switch table.Seats[ seat ].Kind {
	case KindBot:
		level , _ = game.ParseLevel( string( table.Seats[ seat ].Level ) )
	case KindHuman:
		if chosen , ok := table.AutoPlayLevel[ table.Seats[ seat ].UserID ]; ok { level = chosen }
	}
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
		// Think first, on a copy, with the table unlocked: an Expert or Master move
		// takes tens of milliseconds, and nobody else at the table should
		// wait on it. Then make the move only if nothing changed meanwhile.
		current := func( table *Table ) ( result bool ) {
			result = table.Version == version && table.Game != nil && table.Game.Turn == seat && table.computerMoves( seat )
			return
		}
		table := hub.table( id )
		if table == nil { return }
		table.mu.Lock()
		if current( table ) == false {
			table.mu.Unlock()
			return
		}
		snapshot , level := table.Game.Clone() , table.moveLevel( seat )
		table.mu.Unlock()

		action , ok := botAction( snapshot , seat , level )
		if ok == false { return }
		action.Bot = true
		hub.mutate( id , func( table *Table ) ( fn_err error ) {
			if current( table ) == false {
				fn_err = errStale
				return
			}
			if fn_err = table.perform( action ); fn_err != nil && fn_err != ErrHeld {
				// A bot only ever picks from the engine's own legal moves, so
				// this is a bug worth hearing about, not a player mistake.
				log.Printf( "[tables] bot in seat %d failed at %s: %v" , seat , id , fn_err )
			}
			return
		} )
	} )
}

// Hint is the Expert bot's choice for a person's seat, offered rather than
// made. The route checks the person is allowed hints; this checks it is
// their move. The search runs on a copy, after the table is unlocked.
func ( hub *Hub ) Hint( id string , user *models.User ) ( action Action , err error ) {
	table := hub.table( id )
	if table == nil {
		err = ErrNoSuchTable
		return
	}
	table.mu.Lock()
	seat := table.seatOf( user.ID )
	var snapshot *game.State
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
		snapshot = table.Game.Clone()
	}
	table.mu.Unlock()
	if err != nil { return }
	var ok bool
	if action , ok = botAction( snapshot , seat , game.HelpLevel ); ok == false {
		err = game.ErrNotYourTurn
	}
	return
}
