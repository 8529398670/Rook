package game

// Bots exist so one to three people can still get a game, and so the
// property test can play thousands of games without anyone at the keyboard.
// They come in four levels (bot_level.go): Easy (bot_easy.go), Normal (this
// file -- simple heuristics, the original bot), Hard (bot_hard.go, which
// counts cards) and Expert (bot_expert.go, which searches).
//
// Every decision at every level reads only what that seat could see at a
// real table: its own hand, the bids, trump, the cards already played, and
// the buried kitty only for the seat that buried it. None of them looks at
// another hand.

func ( state *State ) isTrump( card Card ) ( result bool ) {
	result = state.Trump != NoColor && state.suitOf( card ) == state.Trump
	return
}

// bestTrump is the color a bot would name: the longest, weighted toward the
// high cards that win tricks once it is trump.
func bestTrump( hand []Card ) ( best Color ) {
	best = Colors[ 0 ]
	best_strength := -1
	for _ , color := range Colors {
		strength := 0
		for _ , card := range hand {
			if card.Color != color { continue }
			strength += 4
			switch {
			case card.Number == 1:
				strength += 6
			case card.Number >= 13:
				strength += 3
			case card.Number >= 10:
				strength += 1
			}
		}
		if strength > best_strength {
			best , best_strength = color , strength
		}
	}
	return
}

// handStrength estimates how much of the 180 this hand could carry with a
// partner's help. The scale is arbitrary; normalBid's formula was tuned against
// the property test's thousands of bot games until ordinary hands go to about
// 105, long high hands push toward 150, and bidders make it roughly three
// times in four -- close enough to people that a set is a real risk.
func handStrength( hand []Card , trump Color ) ( score int ) {
	trump_length := 0
	for _ , card := range hand {
		switch {
		case card.IsRook():
			score += 12
			trump_length++
		case card.Color == trump:
			trump_length++
			score += 5
			switch {
			case card.Number == 1:
				score += 6
			case card.Number >= 13:
				score += 3
			case card.Number >= 11:
				score += 1
			}
		case card.Number == 1:
			score += 9
		case card.Number == 14:
			score += 3
		case card.Number == 13:
			score += 1
		}
	}
	if trump_length > 4 {
		score += ( trump_length - 4 ) * 3
	}
	return
}

// BotBid returns the amount a bot of this level bids, or 0 to pass.
func ( state *State ) BotBid( seat int , level Level ) ( amount int ) {
	switch level.orNormal() {
	case LevelEasy:
		amount = state.easyBid( seat )
	case LevelHard:
		amount = state.hardBid( seat )
	case LevelExpert:
		amount = state.expertBid( seat )
	default:
		amount = state.normalBid( seat )
	}
	return
}

// BotBury returns the five cards a bot of this level buries and the trump it
// names.
func ( state *State ) BotBury( seat int , level Level ) ( buried []Card , trump Color ) {
	switch level.orNormal() {
	case LevelEasy:
		buried , trump = state.easyBury( seat )
	case LevelHard:
		buried , trump = state.hardBury( seat )
	case LevelExpert:
		buried , trump = state.expertBury( seat )
	default:
		buried , trump = state.normalBury( seat )
	}
	return
}

// BotPlay returns the legal card a bot of this level plays.
func ( state *State ) BotPlay( seat int , level Level ) ( card Card ) {
	switch level.orNormal() {
	case LevelEasy:
		card = state.easyPlay( seat )
	case LevelHard:
		card = state.hardPlay( seat )
	case LevelExpert:
		card = state.expertPlay( seat )
	default:
		card = state.normalPlay( seat )
	}
	return
}

// normalBid returns the amount to bid, or 0 to pass. Bots raise by the minimum
// and stop at their estimate, and they back off when their partner already
// holds the bid rather than bidding against their own team.
func ( state *State ) normalBid( seat int ) ( amount int ) {
	hand := state.Hands[ seat ]
	limit := 45 + handStrength( hand , bestTrump( hand ) )*3/2
	if state.HighBidder >= 0 && TeamOf( state.HighBidder ) == TeamOf( seat ) {
		limit -= 15
	}
	if limit > 170 { limit = 170 }
	limit -= limit % BidStep
	if minimum := state.MinNextBid(); minimum <= limit {
		amount = minimum
	}
	return
}

// normalBury names the strongest color and buries the five off-color cards it
// least wants: low cards in short colors, keeping counters and 1s, so the
// hand ends up with voids it can trump into.
func ( state *State ) normalBury( seat int ) ( buried []Card , trump Color ) {
	hand := state.Hands[ seat ]
	trump = bestTrump( hand )

	color_length := map[ Color ]int{}
	for _ , card := range hand {
		color_length[ card.Color ]++
	}
	keep_value := func( card Card ) ( value int ) {
		if card.IsRook() || card.Color == trump {
			value = 1000 + card.rank( state.Options.RookRank )
			return
		}
		value = card.rank( state.Options.RookRank ) + color_length[ card.Color ]*2
		if card.Points() > 0 { value += 10 }
		return
	}

	remaining := append( []Card( nil ) , hand... )
	for len( buried ) < KittySize {
		worst := 0
		for index := range remaining {
			if keep_value( remaining[ index ] ) < keep_value( remaining[ worst ] ) {
				worst = index
			}
		}
		buried = append( buried , remaining[ worst ] )
		remaining = append( remaining[ :worst ] , remaining[ worst+1: ]... )
	}
	return
}

// normalPlay picks a legal card: take the trick as cheaply as possible when it
// can, feed counters to a partner who is winning, and otherwise throw the
// least valuable card it holds.
func ( state *State ) normalPlay( seat int ) ( card Card ) {
	legal := state.LegalCards( seat )
	if len( legal ) == 0 { return }
	card = legal[ 0 ]
	if len( legal ) == 1 { return }

	rook_rank := state.Options.RookRank
	// lowest: fewest points, then non-trump, then lowest rank.
	lowest := func( cards []Card ) ( result Card ) {
		result = cards[ 0 ]
		for _ , candidate := range cards[ 1: ] {
			if lessValuable( state , candidate , result ) { result = candidate }
		}
		return
	}
	highest_rank := func( cards []Card ) ( result Card ) {
		result = cards[ 0 ]
		for _ , candidate := range cards[ 1: ] {
			if candidate.rank( rook_rank ) > result.rank( rook_rank ) { result = candidate }
		}
		return
	}

	plays := state.Trick.Plays
	if len( plays ) == 0 {
		card = state.normalLead( seat , legal , lowest , highest_rank )
		return
	}

	led := state.suitOf( plays[ 0 ].Card )
	holding := plays[ state.winningPlay( plays ) ]
	partner_holds := TeamOf( holding.Seat ) == TeamOf( seat )
	last_to_play := len( plays ) == 3

	if partner_holds {
		partner_strong := state.isTrump( holding.Card ) || holding.Card.Number == 1
		if last_to_play || partner_strong {
			counters := []Card{}
			for _ , candidate := range legal {
				if candidate.Points() > 0 && state.isTrump( candidate ) == false {
					counters = append( counters , candidate )
				}
			}
			if len( counters ) > 0 {
				card = counters[ 0 ]
				for _ , candidate := range counters[ 1: ] {
					if candidate.Points() > card.Points() { card = candidate }
				}
				return
			}
		}
		card = lowest( legal )
		return
	}

	winners := []Card{}
	for _ , candidate := range legal {
		if state.beats( candidate , holding.Card , led ) {
			winners = append( winners , candidate )
		}
	}
	if len( winners ) == 0 {
		card = lowest( legal )
		return
	}

	cheapest := winners[ 0 ]
	for _ , candidate := range winners[ 1: ] {
		candidate_trump , cheapest_trump := state.isTrump( candidate ) , state.isTrump( cheapest )
		if candidate_trump != cheapest_trump {
			if cheapest_trump { cheapest = candidate }
			continue
		}
		if candidate.rank( rook_rank ) < cheapest.rank( rook_rank ) { cheapest = candidate }
	}

	trick_points := 0
	for _ , play := range plays {
		trick_points += play.Card.Points()
	}
	if trick_points > 0 || last_to_play || state.isTrump( cheapest ) == false {
		card = cheapest
		return
	}
	card = lowest( legal )
	return
}

func ( state *State ) normalLead( seat int , legal []Card , lowest func( []Card ) Card , highest_rank func( []Card ) Card ) ( card Card ) {
	// An off-color 1 is the top card of its color: lead it while nobody is
	// void yet.
	for _ , candidate := range legal {
		if candidate.Number == 1 && state.isTrump( candidate ) == false {
			card = candidate
			return
		}
	}

	trumps := []Card{}
	others := []Card{}
	for _ , candidate := range legal {
		if state.isTrump( candidate ) {
			trumps = append( trumps , candidate )
		} else {
			others = append( others , candidate )
		}
	}

	// The bidding team pulls trump when it is long in it, so the defenders
	// cannot trump its winners later.
	if TeamOf( seat ) == TeamOf( state.Bidder ) && len( trumps ) >= 4 {
		card = highest_rank( trumps )
		return
	}
	if len( others ) > 0 {
		card = lowest( others )
		return
	}
	card = lowest( legal )
	return
}

func lessValuable( state *State , a Card , b Card ) ( result bool ) {
	if a.Points() != b.Points() {
		result = a.Points() < b.Points()
		return
	}
	a_trump , b_trump := state.isTrump( a ) , state.isTrump( b )
	if a_trump != b_trump {
		result = b_trump
		return
	}
	result = a.rank( state.Options.RookRank ) < b.rank( state.Options.RookRank )
	return
}

// BotMove makes whatever move the current phase calls for on behalf of seat,
// at the given level. It returns whether a move was made: a bot can only act
// on its own turn.
func ( state *State ) BotMove( seat int , level Level ) ( moved bool , err error ) {
	switch state.Phase {
	case PhaseBidding:
		if state.Turn != seat { return }
		if amount := state.BotBid( seat , level ); amount > 0 {
			err = state.PlaceBid( seat , amount )
		} else {
			err = state.Pass( seat )
		}
	case PhaseKitty:
		if state.Turn != seat { return }
		buried , trump := state.BotBury( seat , level )
		err = state.Bury( seat , buried , trump )
	case PhasePlaying:
		if state.Turn != seat { return }
		err = state.PlayCard( seat , state.BotPlay( seat , level ) )
	default:
		return
	}
	moved = err == nil
	return
}
