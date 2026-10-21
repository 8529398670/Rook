package game

// MinNextBid is the smallest bid the seat whose turn it is may make.
func ( state *State ) MinNextBid() ( result int ) {
	result = state.Bid + BidStep
	return
}

// PlaceBid raises the bid. A bid of 180 -- every counter in the deck -- cannot
// be beaten, so it ends the auction on the spot.
func ( state *State ) PlaceBid( seat int , amount int ) ( err error ) {
	if state.Phase != PhaseBidding {
		err = ErrWrongPhase
		return
	}
	if state.Turn != seat {
		err = ErrNotYourTurn
		return
	}
	if amount%BidStep != 0 {
		err = ErrBidStep
		return
	}
	if amount < state.MinNextBid() {
		err = ErrBidTooLow
		return
	}
	if amount > MaxBid {
		err = ErrBidTooHigh
		return
	}

	state.Bid = amount
	state.HighBidder = seat
	state.LastBid[ seat ] = amount
	state.log( Event{ Kind: "bid" , Seat: seat , Amount: amount , Team: TeamOf( seat ) } )

	if amount == MaxBid {
		state.awardBid( seat )
		return
	}
	state.Turn = state.nextBidder( seat )
	return
}

// Pass is final for the rest of the hand. Once three seats have passed, the
// fourth takes the bid at whatever it stands at -- including the opening 65
// if they never bid at all.
func ( state *State ) Pass( seat int ) ( err error ) {
	if state.Phase != PhaseBidding {
		err = ErrWrongPhase
		return
	}
	if state.Turn != seat {
		err = ErrNotYourTurn
		return
	}

	state.Passed[ seat ] = true
	state.PassedAt[ seat ] = state.Bid
	state.log( Event{ Kind: "pass" , Seat: seat , Team: TeamOf( seat ) } )

	passed := 0
	remaining := -1
	for candidate := 0; candidate < 4; candidate++ {
		if state.Passed[ candidate ] {
			passed++
		} else {
			remaining = candidate
		}
	}
	if passed == 3 {
		state.awardBid( remaining )
		return
	}
	state.Turn = state.nextBidder( seat )
	return
}

// Reshuffle deals this hand again from a new deck, for a seat that has not
// bid or passed yet. Whatever bids the others made are thrown away and the
// auction starts over with the same opener; it is still the same hand.
func ( state *State ) Reshuffle( seat int , deck []Card ) ( err error ) {
	if state.Phase != PhaseBidding {
		err = ErrWrongPhase
		return
	}
	if seat < 0 || seat > 3 {
		err = ErrNotYourTurn
		return
	}
	if state.LastBid[ seat ] > 0 || state.Passed[ seat ] {
		err = ErrAlreadyBid
		return
	}
	if ValidDeck( deck ) == false {
		err = ErrBadDeck
		return
	}
	state.redeal( deck )
	state.log( Event{ Kind: "reshuffle" , Seat: seat , Amount: state.HandNumber , Team: TeamOf( seat ) } )
	return
}

func ( state *State ) nextBidder( seat int ) ( result int ) {
	result = nextSeat( seat )
	for state.Passed[ result ] {
		result = nextSeat( result )
	}
	return
}

// awardBid hands the kitty to the winner. Their hand grows to 18 cards until
// they bury five.
func ( state *State ) awardBid( seat int ) {
	state.Bidder = seat
	state.HighBidder = seat
	state.PickedUp = append( []Card( nil ) , state.Kitty... )
	state.Hands[ seat ] = append( state.Hands[ seat ] , state.Kitty... )
	state.Kitty = nil
	state.Phase = PhaseKitty
	state.Turn = seat
	state.log( Event{ Kind: "won_bid" , Seat: seat , Amount: state.Bid , Team: TeamOf( seat ) } )
}
