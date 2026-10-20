package game

// The Easy bot plays like someone in their first few games: it knows the
// rules and wants tricks, but not yet what a trick is worth. It bids
// timidly, wins with its biggest card, throws whatever is lowest even when
// that hands points to the other team, never helps its partner, buries
// without a plan -- and now and then simply plays the wrong card. Those
// slips are seeded from the position, like Expert's search, so the same
// position always gets the same card and a recorded game replays exactly.

// easySlip is how often, out of easySlipOutOf, Easy plays a card at random.
const (
	easySlip      = 1
	easySlipOutOf = 3
)

// easyBid counts the obvious things -- 1s, the bird, a long color -- and bids
// up to that, against its own partner too. It is timid: bolder bidding alone
// would beat Normal, since the bidder's side gets the kitty.
func ( state *State ) easyBid( seat int ) ( amount int ) {
	hand := state.Hands[ seat ]
	longest := 0
	for _ , color := range Colors {
		length := 0
		for _ , card := range hand {
			if card.Color == color { length++ }
		}
		if length > longest { longest = length }
	}
	limit := 45 + longest*5
	for _ , card := range hand {
		if card.Number == 1 { limit += 8 }
		if card.IsRook() { limit += 8 }
	}
	if limit > 130 { limit = 130 }
	limit -= limit % BidStep
	if minimum := state.MinNextBid(); minimum <= limit {
		amount = minimum
	}
	return
}

// easyBury names its longest color and buries the five lowest off-color
// cards, points or not.
func ( state *State ) easyBury( seat int ) ( buried []Card , trump Color ) {
	hand := state.Hands[ seat ]
	trump = Colors[ 0 ]
	longest := -1
	for _ , color := range Colors {
		length := 0
		for _ , card := range hand {
			if card.Color == color { length++ }
		}
		if length > longest { trump , longest = color , length }
	}
	keep := func( card Card ) ( value int ) {
		value = card.rank( state.Options.RookRank )
		if card.IsRook() || card.Color == trump { value += 1000 }
		return
	}
	remaining := append( []Card( nil ) , hand... )
	for len( buried ) < KittySize {
		worst := 0
		for index := range remaining {
			if keep( remaining[ index ] ) < keep( remaining[ worst ] ) { worst = index }
		}
		buried = append( buried , remaining[ worst ] )
		remaining = append( remaining[ :worst ] , remaining[ worst+1: ]... )
	}
	return
}

// easyPlay leads the top card of its longest color, wins with the biggest
// card that wins, and otherwise plays its lowest card.
func ( state *State ) easyPlay( seat int ) ( card Card ) {
	legal := state.LegalCards( seat )
	if len( legal ) == 0 { return }
	card = legal[ 0 ]
	if len( legal ) == 1 { return }
	rook_rank := state.Options.RookRank
	if source := state.seedFor( seat ); source.IntN( easySlipOutOf ) < easySlip {
		card = legal[ source.IntN( len( legal ) ) ]
		return
	}

	plays := state.Trick.Plays
	if len( plays ) == 0 {
		length := map[ Color ]int{}
		for _ , candidate := range legal {
			length[ state.suitOf( candidate ) ]++
		}
		for _ , candidate := range legal[ 1: ] {
			suit , best := state.suitOf( candidate ) , state.suitOf( card )
			if length[ suit ] > length[ best ] || ( length[ suit ] == length[ best ] && candidate.rank( rook_rank ) > card.rank( rook_rank ) ) {
				card = candidate
			}
		}
		return
	}

	led := state.suitOf( plays[ 0 ].Card )
	holding := plays[ state.winningPlay( plays ) ].Card
	won := false
	for _ , candidate := range legal {
		if state.beats( candidate , holding , led ) && ( won == false || candidate.rank( rook_rank ) > card.rank( rook_rank ) ) {
			card , won = candidate , true
		}
	}
	if won { return }
	for _ , candidate := range legal {
		if candidate.rank( rook_rank ) < card.rank( rook_rank ) { card = candidate }
	}
	return
}
