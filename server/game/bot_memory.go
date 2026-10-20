package game

// botMemory is what one seat knows partway through a hand: everything it
// could work out at a real table by watching, and nothing more. Hard and
// Expert both read it.
//
//   - The cards it has seen: its own hand, every card played, and -- for
//     the bidder only -- the five it buried.
//   - Which seats have shown a void: a seat that did not follow the color
//     led has none of it left.
type botMemory struct {
	state *State
	seat  int
	// unseen marks every card this seat has not seen, by cardIndex. They are
	// in the other three hands, or in the kitty.
	unseen       [57]bool
	unseen_count [4]int // unseen cards of each effective color, by colorIndex
	top          [4]int // the highest rank among them, 0 when none
	// void[seat][colorIndex]: that seat has shown it holds none of the
	// color (counted as the effective color, so the bird is trump).
	void [4][4]bool
}

// cardIndex numbers the 57 cards 0-56, the bird last.
func cardIndex( card Card ) ( index int ) {
	if card.IsRook() {
		index = 56
		return
	}
	index = colorIndex( card.Color )*14 + card.Number - 1
	return
}

func cardAt( index int ) ( card Card ) {
	if index == 56 {
		card = TheRook
		return
	}
	card = Card{ Color: Colors[ index/14 ] , Number: index%14 + 1 }
	return
}

func colorIndex( color Color ) ( index int ) {
	index = int( color )
	return
}

func ( state *State ) remember( seat int ) ( memory botMemory ) {
	memory = botMemory{ state: state , seat: seat }
	for index := range memory.unseen {
		memory.unseen[ index ] = true
	}
	seen := func( card Card ) { memory.unseen[ cardIndex( card ) ] = false }
	for _ , card := range state.Hands[ seat ] {
		seen( card )
	}
	if seat == state.Bidder && state.Phase == PhasePlaying {
		for _ , card := range state.Kitty {
			seen( card )
		}
	}
	for number := 0; number <= len( state.Completed ); number++ {
		plays := state.Trick.Plays
		if number < len( state.Completed ) { plays = state.Completed[ number ].Plays }
		if len( plays ) == 0 { continue }
		led := state.suitOf( plays[ 0 ].Card )
		for at , play := range plays {
			seen( play.Card )
			if at == 0 || state.suitOf( play.Card ) == led { continue }
			// The bird played off-color under "Rook anytime" says nothing:
			// it may be played while holding the color led.
			if play.Card.IsRook() && state.Options.RookAnytime { continue }
			memory.void[ play.Seat ][ colorIndex( led ) ] = true
		}
	}
	if state.Trump != NoColor {
		for index , unseen := range memory.unseen {
			if unseen == false { continue }
			card := cardAt( index )
			suit := colorIndex( state.suitOf( card ) )
			memory.unseen_count[ suit ]++
			if rank := card.rank( state.Options.RookRank ); rank > memory.top[ suit ] { memory.top[ suit ] = rank }
		}
	}
	return
}

// isBoss reports whether no card this seat has not seen could beat card in
// its own color -- whether it wins any trick of its color that nobody trumps.
func ( memory *botMemory ) isBoss( card Card ) ( result bool ) {
	state := memory.state
	if state.Trump == NoColor { return }
	result = card.rank( state.Options.RookRank ) > memory.top[ colorIndex( state.suitOf( card ) ) ]
	return
}

// trumpOut is how many trump this seat has not seen yet.
func ( memory *botMemory ) trumpOut() ( count int ) {
	if memory.state.Trump == NoColor { return }
	count = memory.unseen_count[ colorIndex( memory.state.Trump ) ]
	return
}

// threatened reports whether some opponent still to play to this trick might
// beat winning. An opponent can overtake in the color led if any higher card
// of it is out, and can trump in once it has shown it is out of the color --
// or, when so few of the color are left that someone probably is.
func ( memory *botMemory ) threatened( winning Card , led Color , after []int ) ( result bool ) {
	state := memory.state
	rook_rank := state.Options.RookRank
	trump := colorIndex( state.Trump )
	lead := colorIndex( led )
	rank := winning.rank( rook_rank )
	winning_trump := state.isTrump( winning )
	for _ , other := range after {
		if TeamOf( other ) == TeamOf( memory.seat ) { continue }
		// Overtaking in the color led (trump included, when trump was led).
		if memory.void[ other ][ lead ] == false && state.suitOf( winning ) == led && memory.top[ lead ] > rank {
			result = true
			return
		}
		if led == state.Trump || memory.void[ other ][ trump ] { continue }
		// Trumping in, by someone out of the color led.
		out_of_led := memory.void[ other ][ lead ] || memory.unseen_count[ lead ] <= 2
		if out_of_led && memory.unseen_count[ trump ] > 0 && ( winning_trump == false || memory.top[ trump ] > rank ) {
			result = true
			return
		}
		// The bird, which "Rook anytime" lets anyone play.
		if state.Options.RookAnytime && memory.unseen[ cardIndex( TheRook ) ] && ( winning_trump == false || TheRook.rank( rook_rank ) > rank ) {
			result = true
			return
		}
	}
	return
}

// stillToPlay lists the seats that play to the current trick after seat.
func ( state *State ) stillToPlay( seat int ) ( after []int ) {
	for next , left := nextSeat( seat ) , 3-len( state.Trick.Plays ); left > 0; next , left = nextSeat( next ) , left-1 {
		after = append( after , next )
	}
	return
}
