package game

// LegalCards is the single definition of "what may this seat play right
// now". PlayCard checks against it, and the table view sends it to the
// player, which is how the browser dims illegal cards without knowing a
// single rule.
//
//   - Follow the color led if you can. The bird is trump, so it follows trump
//     and leading it leads trump.
//   - With RookAnytime, the bird may also be played when you could follow.
//   - Without RookFirstTrick, the bird may not appear on the first trick at
//     all, unless it is literally the only card you hold.
func ( state *State ) LegalCards( seat int ) ( legal []Card ) {
	if state.Phase != PhasePlaying || state.Turn != seat { return }

	hand := state.Hands[ seat ]
	rook_banned := len( state.Completed ) == 0 && state.Options.RookFirstTrick == false

	without_banned_rook := func( cards []Card ) ( result []Card ) {
		result = cards
		if rook_banned == false { return }
		trimmed , found := removeCard( cards , TheRook )
		if found && len( trimmed ) > 0 {
			result = trimmed
		}
		return
	}

	if len( state.Trick.Plays ) == 0 {
		legal = without_banned_rook( append( []Card( nil ) , hand... ) )
		return
	}

	led := state.suitOf( state.Trick.Plays[ 0 ].Card )
	following := []Card{}
	for _ , card := range hand {
		if state.suitOf( card ) == led {
			if card.IsRook() && rook_banned { continue }
			following = append( following , card )
		}
	}
	if len( following ) == 0 {
		legal = without_banned_rook( append( []Card( nil ) , hand... ) )
		return
	}
	legal = following
	if state.Options.RookAnytime && rook_banned == false && containsCard( hand , TheRook ) && containsCard( legal , TheRook ) == false {
		legal = append( legal , TheRook )
	}
	return
}

// PlayCard lays one card. When it completes a trick the trick is scored and
// its winner leads the next; when it completes the thirteenth, the hand is
// scored.
func ( state *State ) PlayCard( seat int , card Card ) ( err error ) {
	if state.Phase != PhasePlaying {
		err = ErrWrongPhase
		return
	}
	if state.Turn != seat {
		err = ErrNotYourTurn
		return
	}
	if containsCard( state.Hands[ seat ] , card ) == false {
		err = ErrNotInHand
		return
	}
	if containsCard( state.LegalCards( seat ) , card ) == false {
		err = ErrMustFollowSuit
		if card.IsRook() && len( state.Completed ) == 0 && state.Options.RookFirstTrick == false {
			err = ErrRookFirstTrick
		}
		return
	}

	state.Hands[ seat ] , _ = removeCard( state.Hands[ seat ] , card )
	if len( state.Trick.Plays ) == 0 {
		state.Trick.Leader = seat
	}
	state.Trick.Plays = append( state.Trick.Plays , Play{ Seat: seat , Card: card } )
	state.log( Event{ Kind: "play" , Seat: seat , Card: card.ID() , Team: TeamOf( seat ) } )

	if len( state.Trick.Plays ) < 4 {
		state.Turn = nextSeat( seat )
		return
	}
	state.finishTrick()
	return
}

// CurrentWinner is the seat holding the trick so far, or -1 on an empty one.
// Bots use it to decide whether their partner already has the trick.
func ( state *State ) CurrentWinner() ( seat int ) {
	seat = -1
	if len( state.Trick.Plays ) == 0 { return }
	seat = state.Trick.Plays[ state.winningPlay( state.Trick.Plays ) ].Seat
	return
}

// winningPlay returns the index of the play that takes the trick: the highest
// trump if any was played, else the highest card of the color led.
func ( state *State ) winningPlay( plays []Play ) ( best int ) {
	led := state.suitOf( plays[ 0 ].Card )
	for index := 1; index < len( plays ); index++ {
		if state.beats( plays[ index ].Card , plays[ best ].Card , led ) {
			best = index
		}
	}
	return
}

// Beats reports whether challenger takes a trick currently held by holder.
func ( state *State ) beats( challenger Card , holder Card , led Color ) ( result bool ) {
	challenger_suit := state.suitOf( challenger )
	holder_suit := state.suitOf( holder )
	challenger_trump := challenger_suit == state.Trump
	holder_trump := holder_suit == state.Trump

	switch {
	case challenger_trump && holder_trump == false:
		result = true
	case challenger_trump == false && holder_trump:
		result = false
	case challenger_trump && holder_trump:
		// Two trumps on an off-color lead: the higher trump, never "did it
		// follow the color led", which neither did.
		result = challenger.rank( state.Options.RookRank ) > holder.rank( state.Options.RookRank )
	case challenger_suit != led:
		// Off-color and not trump: a discard, which can never win.
		result = false
	case holder_suit != led:
		result = true
	default:
		result = challenger.rank( state.Options.RookRank ) > holder.rank( state.Options.RookRank )
	}
	return
}

func ( state *State ) finishTrick() {
	trick := state.Trick
	winner_index := state.winningPlay( trick.Plays )
	trick.Winner = trick.Plays[ winner_index ].Seat
	for _ , play := range trick.Plays {
		trick.Points += play.Card.Points()
	}

	team := TeamOf( trick.Winner )
	state.Taken[ team ] += trick.Points
	state.TricksWon[ team ]++
	state.Completed = append( state.Completed , trick )
	state.log( Event{ Kind: "trick" , Seat: trick.Winner , Card: trick.Plays[ winner_index ].Card.ID() , Amount: trick.Points , Team: team } )

	if len( state.Completed ) == HandSize {
		state.finishHand()
		return
	}
	state.Trick = Trick{ Leader: trick.Winner , Winner: -1 }
	state.Turn = trick.Winner
}

// Continue is each seat saying "seen the summary, deal the next hand". The
// next hand is dealt, from deck, once all four have said so; deck is ignored
// until then, and nil means "shuffle a new one". The tables package answers
// for bots immediately so only people are actually waited on.
func ( state *State ) Continue( seat int , deck []Card ) ( dealt bool , err error ) {
	if deck != nil && ValidDeck( deck ) == false {
		err = ErrBadDeck
		return
	}
	if state.Phase != PhaseHandOver {
		err = ErrWrongPhase
		return
	}
	if state.Ready[ seat ] {
		err = ErrAlreadyReady
		return
	}
	state.Ready[ seat ] = true
	for candidate := 0; candidate < 4; candidate++ {
		if state.Ready[ candidate ] == false { return }
	}
	if deck == nil { deck = ShuffledDeck( nil ) }
	state.BidStarter = nextSeat( state.BidStarter )
	state.deal( deck )
	dealt = true
	return
}
