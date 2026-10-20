package game

// The Hard bot plays the strategy guide (language.yaml, guide.strategy) rule
// by rule, with a memory of every card it has seen (bot_memory.go):
//
//   - It knows which cards are boss -- the top one still out in their color
//     -- so it cashes them, and it knows when a trick is safe for its side.
//   - As the bidder it draws trump while the other team may still hold some,
//     keeping one back for the last trick, which carries the kitty.
//   - Defending it never leads trump, and leads low cards from long colors
//     to make the bidder trump them.
//   - It throws points onto a trick its partner is sure to take, and nothing
//     worth points onto one the other team is taking.
//   - It never trumps a trick its partner is already winning.
//
// It is also the policy Expert uses to play out the hands it imagines, so
// it has to stay cheap: no searching here, only what the memory knows.

// hardTrumpScore is the guide's way of sizing up a color for trump: length
// first, then the top cards of the color.
func hardTrumpScore( hand []Card , color Color ) ( score int ) {
	for _ , card := range hand {
		if card.Color != color { continue }
		score += 5
		switch {
		case card.Number == 1:
			score += 6
		case card.Number == 14:
			score += 4
		case card.Number == 13:
			score += 2
		case card.Number >= 11:
			score += 1
		}
	}
	return
}

func hardTrump( hand []Card ) ( best Color ) {
	best = Colors[ 0 ]
	best_score := -1
	for _ , color := range Colors {
		if score := hardTrumpScore( hand , color ); score > best_score {
			best , best_score = color , score
		}
	}
	return
}

// hardEstimate is how many points this hand expects its team to take if it
// wins the bid: Normal's estimate, plus the short colors the guide says
// Normal misses -- they trump the other team's points once the kitty has
// emptied them. The base is 5 under Normal's: Hard defends well enough that
// letting a thin bid go to the other team costs little, and the duplicate
// league (bot_league_test.go) scored 40 best among 30 to 75.
func hardEstimate( hand []Card ) ( estimate int ) {
	trump := hardTrump( hand )
	estimate = 40 + handStrength( hand , trump )*3/2
	for _ , color := range Colors {
		if color == trump { continue }
		length := 0
		has_one := false
		for _ , card := range hand {
			if card.Color != color { continue }
			length++
			if card.Number == 1 { has_one = true }
		}
		switch {
		case length == 0:
			estimate += 6
		case length == 1 && has_one == false:
			estimate += 4
		case length == 2 && has_one == false:
			estimate += 1
		}
	}
	return
}

// hardBid bids up to the estimate. It backs off when its partner holds the
// bid unless its own hand is clearly the stronger, and -- unless its hand is
// poor -- it does not let the other team take the bid cheaply.
func ( state *State ) hardBid( seat int ) ( amount int ) {
	limit := hardEstimate( state.Hands[ seat ] )
	partner_holds := state.HighBidder >= 0 && TeamOf( state.HighBidder ) == TeamOf( seat )
	if partner_holds {
		limit -= 20
	}
	if limit > 170 { limit = 170 }
	limit -= limit % BidStep
	if minimum := state.MinNextBid(); minimum <= limit {
		amount = minimum
	}
	return
}

// hardBury names trump and buries five: it empties short colors so it can
// trump them, keeps its 1s, and buries counters only when its trump is long
// enough to expect the last trick (which carries the kitty).
func ( state *State ) hardBury( seat int ) ( buried []Card , trump Color ) {
	hand := state.Hands[ seat ]
	trump = hardTrump( hand )
	buried = state.hardBuryFor( hand , trump )
	return
}

func ( state *State ) hardBuryFor( hand []Card , trump Color ) ( buried []Card ) {
	rook_rank := state.Options.RookRank
	trump_length := 0
	for _ , card := range hand {
		if card.IsRook() || card.Color == trump { trump_length++ }
	}
	long_trump := trump_length >= 7

	remaining := append( []Card( nil ) , hand... )
	has := func( color Color , number int ) ( result bool ) {
		result = containsCard( remaining , Card{ Color: color , Number: number } )
		return
	}
	// bury_score: higher goes first.
	bury_score := func( card Card , length map[ Color ]int ) ( score int ) {
		if card.IsRook() || card.Color == trump {
			score = -2000 + 30 - card.rank( rook_rank )
			return
		}
		if card.Number == 1 {
			score = -1000
			return
		}
		score = 30 - card.rank( rook_rank )
		if length[ card.Color ] <= 5 {
			score += ( 6 - length[ card.Color ] )*8
		}
		if card.Points() > 0 {
			protected := has( card.Color , 1 ) && ( card.Number != 10 || has( card.Color , 14 ) )
			switch {
			case long_trump && protected == false:
				score += 10
			case long_trump:
				score -= 5
			default:
				score -= 25
			}
		}
		return
	}
	for len( buried ) < KittySize {
		length := map[ Color ]int{}
		for _ , card := range remaining {
			length[ card.Color ]++
		}
		best := 0
		for index := range remaining {
			if bury_score( remaining[ index ] , length ) > bury_score( remaining[ best ] , length ) { best = index }
		}
		buried = append( buried , remaining[ best ] )
		remaining = append( remaining[ :best ] , remaining[ best+1: ]... )
	}
	return
}

// hardCost is what a card is worth keeping: the lower, the sooner it goes
// when there is nothing better to do. Points cost the most, then trump
// (and most of all the last trump while the last trick is still to come),
// then boss cards; a card from a short color costs a little less, since
// emptying the color lets this seat trump it later.
func ( memory *botMemory ) hardCost( card Card ) ( cost int ) {
	state := memory.state
	hand := state.Hands[ memory.seat ]
	cost = card.Points()*10 + card.rank( state.Options.RookRank )
	if state.isTrump( card ) {
		cost += 200
		trumps := 0
		for _ , held := range hand {
			if state.isTrump( held ) { trumps++ }
		}
		if trumps == 1 && len( hand ) > 1 { cost += 80 }
	} else {
		length := 0
		for _ , held := range hand {
			if held.Color == card.Color && state.isTrump( held ) == false { length++ }
		}
		if length < 4 { cost -= ( 4 - length )*3 }
	}
	if memory.isBoss( card ) { cost += 60 }
	return
}

func ( memory *botMemory ) cheapest( cards []Card ) ( result Card ) {
	result = cards[ 0 ]
	best := memory.hardCost( result )
	for _ , card := range cards[ 1: ] {
		if cost := memory.hardCost( card ); cost < best {
			result , best = card , cost
		}
	}
	return
}

func ( state *State ) hardPlay( seat int ) ( card Card ) {
	legal := state.LegalCards( seat )
	if len( legal ) == 0 { return }
	card = legal[ 0 ]
	if len( legal ) == 1 { return }
	memory := state.remember( seat )
	if len( state.Trick.Plays ) == 0 {
		card = memory.hardLead( legal )
		return
	}
	card = memory.hardFollow( legal )
	return
}

// safeToCash reports whether a boss off-color card should win if led: no
// opponent has shown a void in its color while trump may still be out.
func ( memory *botMemory ) safeToCash( card Card ) ( result bool ) {
	state := memory.state
	if memory.isBoss( card ) == false { return }
	if memory.trumpOut() == 0 {
		result = true
		return
	}
	for other := 0; other < 4; other++ {
		if TeamOf( other ) == TeamOf( memory.seat ) { continue }
		if memory.void[ other ][ colorIndex( card.Color ) ] && memory.void[ other ][ colorIndex( state.Trump ) ] == false { return }
	}
	result = true
	return
}

func ( memory *botMemory ) hardLead( legal []Card ) ( card Card ) {
	state := memory.state
	seat := memory.seat
	trump := colorIndex( state.Trump )
	partner := ( seat + 2 ) % 4

	trumps := []Card{}
	others := []Card{}
	for _ , candidate := range legal {
		if state.isTrump( candidate ) {
			trumps = append( trumps , candidate )
		} else {
			others = append( others , candidate )
		}
	}
	opponents_have_trump := false
	if memory.trumpOut() > 0 {
		for other := 0; other < 4; other++ {
			if TeamOf( other ) != TeamOf( seat ) && memory.void[ other ][ trump ] == false { opponents_have_trump = true }
		}
	}

	// The bidder draws trump: a boss trump while it has one, a low one
	// while it is long, always keeping one back for the last trick.
	if seat == state.Bidder && opponents_have_trump && len( trumps ) >= 2 {
		highest := trumps[ 0 ]
		for _ , candidate := range trumps[ 1: ] {
			if candidate.rank( state.Options.RookRank ) > highest.rank( state.Options.RookRank ) { highest = candidate }
		}
		if memory.isBoss( highest ) {
			card = highest
			return
		}
		if len( trumps ) >= 4 {
			card = memory.cheapest( trumps )
			return
		}
	}

	// Cash winners: a boss off-color card nobody can trump, the one worth
	// most first.
	cash := []Card{}
	for _ , candidate := range others {
		if memory.safeToCash( candidate ) { cash = append( cash , candidate ) }
	}
	if len( cash ) > 0 {
		card = cash[ 0 ]
		for _ , candidate := range cash[ 1: ] {
			if candidate.Points() > card.Points() { card = candidate }
		}
		return
	}

	// Lead a color the partner is out of and can trump, while the other
	// team still has to follow it.
	if memory.trumpOut() > 0 && memory.void[ partner ][ trump ] == false {
		feed := []Card{}
		for _ , candidate := range others {
			color := colorIndex( candidate.Color )
			if memory.void[ partner ][ color ] && memory.void[ nextSeat( seat ) ][ color ] == false && memory.void[ ( seat+3 )%4 ][ color ] == false {
				feed = append( feed , candidate )
			}
		}
		if len( feed ) > 0 {
			card = memory.cheapest( feed )
			return
		}
	}

	// Otherwise a low card from the longest color: for the bidder that
	// runs the long color, for defenders it makes the bidder trump.
	if len( others ) > 0 {
		length := map[ Color ]int{}
		for _ , candidate := range others {
			length[ candidate.Color ]++
		}
		longest := []Card{}
		best := 0
		for _ , candidate := range others {
			switch {
			case length[ candidate.Color ] > best:
				best = length[ candidate.Color ]
				longest = []Card{ candidate }
			case length[ candidate.Color ] == best:
				longest = append( longest , candidate )
			}
		}
		card = memory.cheapest( longest )
		if card.Points() == 0 || memory.cheapest( others ).Points() > 0 { return }
		card = memory.cheapest( others )
		return
	}
	card = memory.cheapest( legal )
	return
}

func ( memory *botMemory ) hardFollow( legal []Card ) ( card Card ) {
	state := memory.state
	seat := memory.seat
	plays := state.Trick.Plays
	led := state.suitOf( plays[ 0 ].Card )
	holding := plays[ state.winningPlay( plays ) ]
	after := state.stillToPlay( seat )
	partner_after := false
	for _ , other := range after {
		if TeamOf( other ) == TeamOf( seat ) { partner_after = true }
	}
	worth := 0
	for _ , play := range plays {
		worth += play.Card.Points()
	}

	winners := []Card{}
	safe := []Card{}
	for _ , candidate := range legal {
		if state.beats( candidate , holding.Card , led ) == false { continue }
		winners = append( winners , candidate )
		if memory.threatened( candidate , led , after ) == false { safe = append( safe , candidate ) }
	}

	if TeamOf( holding.Seat ) == TeamOf( seat ) {
		if memory.threatened( holding.Card , led , after ) == false {
			card = memory.feed( legal )
			return
		}
		// The partner may lose it. Overtake with a sure winner of the
		// color led when there are points to protect -- but never trump
		// the partner's trick.
		if worth >= 10 {
			overtakes := []Card{}
			for _ , candidate := range safe {
				if state.isTrump( candidate ) == false || led == state.Trump { overtakes = append( overtakes , candidate ) }
			}
			if len( overtakes ) > 0 {
				card = memory.cheapest( overtakes )
				return
			}
		}
		card = memory.cheapest( legal )
		return
	}

	if len( safe ) > 0 {
		best := memory.cheapestWinner( safe )
		trumps := 0
		for _ , held := range state.Hands[ seat ] {
			if state.isTrump( held ) { trumps++ }
		}
		if worth > 0 || best.Points() > 0 || state.isTrump( best ) == false || trumps >= 4 {
			card = best
			return
		}
	} else if len( winners ) > 0 && partner_after == false && worth >= 10 {
		card = memory.cheapestWinner( winners )
		return
	}
	card = memory.cheapest( legal )
	return
}

// cheapestWinner picks the winner that costs least to spend, except that a
// winning card worth points is the best of all: it wins its own points.
func ( memory *botMemory ) cheapestWinner( winners []Card ) ( card Card ) {
	cost := func( candidate Card ) ( result int ) {
		result = memory.hardCost( candidate ) - candidate.Points()*20
		return
	}
	card = winners[ 0 ]
	for _ , candidate := range winners[ 1: ] {
		if cost( candidate ) < cost( card ) { card = candidate }
	}
	return
}

// feed throws the most points it can spare onto a trick the partner is sure
// to take: not trump it needs for control, and not a boss card that will
// win a trick of its own -- unless the bird is low, when the partner's sure
// trick is the safest place it will ever find.
func ( memory *botMemory ) feed( legal []Card ) ( card Card ) {
	state := memory.state
	counters := []Card{}
	for _ , candidate := range legal {
		if candidate.Points() == 0 { continue }
		if candidate.IsRook() && state.Options.RookRank == RookLow {
			card = candidate
			return
		}
		if memory.isBoss( candidate ) { continue }
		if state.isTrump( candidate ) && state.isTrump( state.Trick.Plays[ 0 ].Card ) == false { continue }
		counters = append( counters , candidate )
	}
	if len( counters ) == 0 {
		card = memory.cheapest( legal )
		return
	}
	card = counters[ 0 ]
	for _ , candidate := range counters[ 1: ] {
		if candidate.Points() > card.Points() || ( candidate.Points() == card.Points() && memory.hardCost( candidate ) < memory.hardCost( card ) ) {
			card = candidate
		}
	}
	return
}
