package game

import (
	rand "math/rand/v2"
	sort "sort"
)

// The Expert bot searches. For each decision it imagines many deals of the
// cards it cannot see -- each one consistent with everything it has seen,
// including which seats have shown a void -- and plays every choice forward
// to the end of the hand with all four seats playing Hard. It picks what
// scores best on average.
//
// The imagined deals never peek: they are drawn from the same unseen cards
// the memory tracks, so Expert knows exactly what a careful person at the
// table would, and only thinks further ahead.
//
// The randomness is seeded from the position, so the same position always
// gets the same answer: a hint does not change if you ask twice, and a
// recorded game replays exactly.

const (
	// expertRollouts is roughly how many hands one card-play decision plays
	// out, shared among the legal cards. It keeps a decision to a few tens
	// of milliseconds at the start of a hand and much less later on. More
	// is stronger -- 180 rollouts beat Hard by about 12 points a hand, 360
	// by about 29, 900 by about 35 -- with less to gain past this.
	expertRollouts   = 900
	expertMinSamples = 20
	expertMaxSamples = 80
	// expertBidSamples is how many deals a bid imagines, and
	// expertBurySamples how many each choice of trump is tried on.
	expertBidSamples  = 32
	expertBurySamples = 16
)

// Clone is a deep copy: the tables package works on one outside the table's
// lock, so a slow decision never holds the table up.
func ( state *State ) Clone() ( clone *State ) {
	clone = state.rolloutClone()
	clone.PickedUp = append( []Card( nil ) , state.PickedUp... )
	clone.History = make( []HandResult , len( state.History ) )
	for index , result := range state.History {
		result.Kitty = append( []Card( nil ) , result.Kitty... )
		clone.History[ index ] = result
	}
	clone.Events = append( []Event( nil ) , state.Events... )
	clone.silent = false
	return
}

// rolloutClone copies what playing on changes, and drops the log and the
// score sheet, which an imagined hand never reads.
func ( state *State ) rolloutClone() ( clone *State ) {
	value := *state
	clone = &value
	for seat := range clone.Hands {
		clone.Hands[ seat ] = append( []Card( nil ) , state.Hands[ seat ]... )
	}
	clone.Kitty = append( []Card( nil ) , state.Kitty... )
	clone.PickedUp = nil
	clone.Trick.Plays = append( []Play( nil ) , state.Trick.Plays... )
	clone.Completed = make( []Trick , len( state.Completed ) , HandSize )
	for index , trick := range state.Completed {
		trick.Plays = append( []Play( nil ) , trick.Plays... )
		clone.Completed[ index ] = trick
	}
	clone.History = nil
	clone.Events = nil
	clone.silent = true
	return
}

// seedFor derives the search's random seed from the position: same position,
// same answer.
func ( state *State ) seedFor( seat int ) ( source *rand.Rand ) {
	hash := uint64( 14695981039346656037 )
	mix := func( value int ) {
		hash ^= uint64( value )
		hash *= 1099511628211
	}
	mix( state.HandNumber )
	mix( seat )
	mix( len( state.Phase ) )
	mix( state.Bid )
	for _ , card := range state.Hands[ seat ] {
		mix( cardIndex( card ) )
	}
	for _ , trick := range state.Completed {
		for _ , play := range trick.Plays {
			mix( cardIndex( play.Card ) )
		}
	}
	for _ , play := range state.Trick.Plays {
		mix( cardIndex( play.Card ) + 100 )
	}
	source = rand.New( rand.NewPCG( hash , hash^0x9e3779b97f4a7c15 ) )
	return
}

// imagine deals the cards this seat has not seen into the places it cannot
// see -- the other hands, and the kitty unless this seat buried it -- in the
// right numbers, and with no seat given a color it has shown it is out of.
func ( memory *botMemory ) imagine( source *rand.Rand ) ( world *State ) {
	state := memory.state
	world = state.rolloutClone()

	unseen := []Card{}
	for index , hidden := range memory.unseen {
		if hidden { unseen = append( unseen , cardAt( index ) ) }
	}
	// Places 0-3 are seats, 4 is the kitty.
	var need [5]int
	for seat := 0; seat < 4; seat++ {
		if seat != memory.seat { need[ seat ] = len( state.Hands[ seat ] ) }
	}
	if memory.seat != state.Bidder || state.Phase != PhasePlaying {
		need[ 4 ] = len( state.Kitty )
	}
	total := 0
	for _ , count := range need {
		total += count
	}
	if total != len( unseen ) { return } // cannot happen; play on with the real deal rather than crash

	may := func( place int , card Card , strict bool ) ( result bool ) {
		if need[ place ] == 0 { return }
		if place == 4 || strict == false || state.Trump == NoColor {
			result = true
			return
		}
		result = memory.void[ place ][ colorIndex( state.suitOf( card ) ) ] == false
		return
	}

	var dealt [5][]Card
	for attempt := 0; attempt < 24; attempt++ {
		strict := attempt < 20
		left := need
		for place := range dealt {
			dealt[ place ] = dealt[ place ][ :0 ]
		}
		source.Shuffle( len( unseen ) , func( i int , j int ) { unseen[ i ] , unseen[ j ] = unseen[ j ] , unseen[ i ] } )
		// The most constrained cards go first, so a seat with a long list
		// of voids is not left holding only cards it cannot have.
		places := func( card Card ) ( count int ) {
			for place := 0; place < 5; place++ {
				if may( place , card , strict ) { count++ }
			}
			return
		}
		sort.SliceStable( unseen , func( i int , j int ) bool { return places( unseen[ i ] ) < places( unseen[ j ] ) } )
		ok := true
		for _ , card := range unseen {
			room := 0
			for place := 0; place < 5; place++ {
				if left[ place ] > 0 && may( place , card , strict ) { room += left[ place ] }
			}
			if room == 0 {
				ok = false
				break
			}
			pick := source.IntN( room )
			for place := 0; place < 5; place++ {
				if left[ place ] == 0 || may( place , card , strict ) == false { continue }
				if pick < left[ place ] {
					dealt[ place ] = append( dealt[ place ] , card )
					left[ place ]--
					break
				}
				pick -= left[ place ]
			}
		}
		if ok { break }
	}
	for seat := 0; seat < 4; seat++ {
		if seat != memory.seat { world.Hands[ seat ] = append( []Card( nil ) , dealt[ seat ]... ) }
	}
	if need[ 4 ] > 0 { world.Kitty = append( []Card( nil ) , dealt[ 4 ]... ) }
	return
}

// playOut finishes the hand with every seat playing Hard.
func ( state *State ) playOut() {
	for state.Phase == PhasePlaying {
		seat := state.Turn
		if err := state.PlayCard( seat , state.hardPlay( seat ) ); err != nil { return }
	}
}

// margin is how far team's score moved ahead of the other team's since
// before.
func ( state *State ) margin( team int , before [2]int ) ( result int ) {
	result = ( state.Scores[ team ] - before[ team ] ) - ( state.Scores[ 1-team ] - before[ 1-team ] )
	return
}

func ( state *State ) expertPlay( seat int ) ( card Card ) {
	legal := state.LegalCards( seat )
	if len( legal ) == 0 { return }
	card = legal[ 0 ]
	if len( legal ) == 1 { return }

	memory := state.remember( seat )
	source := state.seedFor( seat )
	samples := expertRollouts / len( legal )
	if samples < expertMinSamples { samples = expertMinSamples }
	if samples > expertMaxSamples { samples = expertMaxSamples }

	team := TeamOf( seat )
	totals := make( []int , len( legal ) )
	for sample := 0; sample < samples; sample++ {
		world := memory.imagine( source )
		for index , candidate := range legal {
			trial := world.rolloutClone()
			if err := trial.PlayCard( seat , candidate ); err != nil { continue }
			trial.playOut()
			totals[ index ] += trial.margin( team , state.Scores )
		}
	}

	// Ties go to what Hard would play, which is usually the card a person
	// would expect.
	card = state.hardPlay( seat )
	best := -1 << 30
	for index , candidate := range legal {
		if candidate == card { best = totals[ index ] }
	}
	for index , candidate := range legal {
		if totals[ index ] > best {
			card , best = candidate , totals[ index ]
		}
	}
	return
}

// expertBid weighs raising against passing on the same imagined deals of the
// other 44 cards. Raising: this seat takes the bid at the next step, buries
// and plays the hand out. Passing: whoever holds the bid keeps it -- or, if
// nobody has bid, the seat whose imagined hand looks strongest takes it at
// the opening. It raises when that scores better on average, then plays to
// the score as the guide says: careful when a set could cost the game,
// bolder when the other team is about to win.
func ( state *State ) expertBid( seat int ) ( amount int ) {
	minimum := state.MinNextBid()
	if minimum > MaxBid { return }
	memory := state.remember( seat )
	source := state.seedFor( seat )
	team := TeamOf( seat )

	gain := 0
	for sample := 0; sample < expertBidSamples; sample++ {
		world := memory.imagine( source )

		raise := world.rolloutClone()
		raise.Bid = minimum
		raise.awardBid( seat )
		buried , trump := raise.hardBury( seat )
		if err := raise.Bury( seat , buried , trump ); err != nil { continue }
		raise.playOut()

		pass := world.rolloutClone()
		other := state.HighBidder
		if other < 0 {
			best := -1
			for candidate := 0; candidate < 4; candidate++ {
				if candidate == seat || state.Passed[ candidate ] { continue }
				if estimate := hardEstimate( pass.Hands[ candidate ] ); estimate > best { other , best = candidate , estimate }
			}
		}
		if other < 0 {
			gain += raise.margin( team , state.Scores )
			continue
		}
		pass.awardBid( other )
		buried , trump = pass.hardBury( other )
		if err := pass.Bury( other , buried , trump ); err != nil { continue }
		pass.playOut()
		gain += raise.margin( team , state.Scores ) - pass.margin( team , state.Scores )
	}

	threshold := 0
	if target := state.Options.PlayTo; target > 0 {
		ours , theirs := target-state.Scores[ team ] , target-state.Scores[ 1-team ]
		switch {
		case theirs <= 70 && theirs < ours:
			threshold -= 10
		case ours <= 50 && theirs > 90:
			threshold += 15
		}
	}
	if gain > threshold*expertBidSamples {
		amount = minimum
	}
	return
}

// expertBury tries each color as trump, with Hard's bury for it, on the same
// imagined deals of the other hands, and keeps whichever scores best.
func ( state *State ) expertBury( seat int ) ( buried []Card , trump Color ) {
	hand := state.Hands[ seat ]
	buried , trump = state.hardBury( seat )
	memory := state.remember( seat )
	source := state.seedFor( seat )
	team := TeamOf( seat )

	type option struct {
		trump  Color
		buried []Card
		total  int
	}
	options := []*option{ { trump: trump , buried: buried } }
	for _ , color := range Colors {
		if color == trump { continue }
		length := 0
		for _ , card := range hand {
			if card.Color == color || card.IsRook() { length++ }
		}
		if length >= 5 {
			options = append( options , &option{ trump: color , buried: state.hardBuryFor( hand , color ) } )
		}
	}
	if len( options ) == 1 { return }

	for sample := 0; sample < expertBurySamples; sample++ {
		world := memory.imagine( source )
		for _ , choice := range options {
			trial := world.rolloutClone()
			if err := trial.Bury( seat , choice.buried , choice.trump ); err != nil {
				choice.total -= 1 << 20
				continue
			}
			trial.playOut()
			choice.total += trial.margin( team , state.Scores )
		}
	}
	best := options[ 0 ]
	for _ , choice := range options[ 1: ] {
		if choice.total > best.total { best = choice }
	}
	buried , trump = best.buried , best.trump
	return
}
