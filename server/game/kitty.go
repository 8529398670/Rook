package game

// Bury puts five of the bidder's 18 cards back as the kitty and names trump,
// in one action: the old version took them together too, and splitting them
// would only add a state where the kitty is half-done.
//
// Any card may be buried, counters and the bird included. Burying points is a
// real gamble -- the kitty goes to whoever takes the last trick -- and that
// is a choice for the player, not the rules.
func ( state *State ) Bury( seat int , cards []Card , trump Color ) ( err error ) {
	if state.Phase != PhaseKitty {
		err = ErrWrongPhase
		return
	}
	if state.Turn != seat || state.Bidder != seat {
		err = ErrNotYourTurn
		return
	}
	if len( cards ) != KittySize {
		err = ErrMustBuryFive
		return
	}
	if _ , ok := ParseColor( trump.String() ); ok == false {
		err = ErrBadTrump
		return
	}

	hand := state.Hands[ seat ]
	for index , card := range cards {
		if containsCard( cards[ :index ] , card ) {
			err = ErrDuplicateCard
			return
		}
		if containsCard( hand , card ) == false {
			err = ErrNotInHand
			return
		}
	}
	for _ , card := range cards {
		hand , _ = removeCard( hand , card )
	}

	state.Hands[ seat ] = hand
	state.Kitty = append( []Card( nil ) , cards... )
	state.PickedUp = nil
	state.Trump = trump
	state.Phase = PhasePlaying
	state.Trick = Trick{ Leader: seat , Winner: -1 }
	state.Turn = seat
	state.log( Event{ Kind: "trump" , Seat: seat , Color: trump.String() , Team: TeamOf( seat ) } )
	return
}
