package game

import (
	math "math"
	rand "math/rand/v2"
)

// Reading the table: what Master knows that Expert does not. Expert treats
// every deal of the unseen cards that fits the voids as equally likely. A
// strong player does not -- the auction and the trump call say a lot:
//
//   - Someone who bid up to 120 holds a strong hand.
//   - Someone who passed at 75 does not.
//   - The bidder named the color they were longest and strongest in.
//
// So Master draws several times as many imagined deals as it needs, scores
// each by how well it explains those choices, and keeps the ones that do,
// in proportion. It judges a hand the way the bots size one up for bidding
// (hardEstimate, hardTrumpScore) -- a rough model of any player, person or
// bot, and only ever applied to the cards the deal imagines, never to a
// real hidden hand.

// Tuned against Expert over three sets of 300 deals. Trusting the reading
// loosely beat trusting it strictly -- people (and bots) bid and call trump
// for reasons a rough estimate misses -- and a bigger pool helped: scale 6
// and trump 2 gained +1.5 a hand, 10 and 4 gained +4, and 15 and 6 gained
// +9 with a pool of 8 and +10 with 16.
const (
	// readingPool is how many deals are drawn for each one kept.
	readingPool = 16
	// readingScale is how far, in points of estimate, a hand can sit on the
	// wrong side of a bid or pass before that deal starts to look unlikely.
	readingScale = 15.0
	// readingTrump is the same for the trump call, in hardTrumpScore points
	// (about 5 per card of the color).
	readingTrump = 6.0
)

// hasEvidence reports whether anyone else has bid, passed or named trump --
// whether there is anything to read yet.
func ( memory *botMemory ) hasEvidence() ( result bool ) {
	state := memory.state
	for seat := 0; seat < 4; seat++ {
		if seat == memory.seat { continue }
		if state.LastBid[ seat ] > 0 || state.PassedAt[ seat ] > 0 || ( seat == state.Bidder && state.Trump != NoColor ) {
			result = true
			return
		}
	}
	return
}

// plausibility is the log of how well an imagined deal explains what every
// other seat did. Each seat is judged on the 13 cards it held: its imagined
// hand plus the cards it has already played (for the bidder, the hand it
// kept after the kitty, which is the best there is to go on).
func ( memory *botMemory ) plausibility( dealt [5][]Card ) ( score float64 ) {
	state := memory.state
	for seat := 0; seat < 4; seat++ {
		if seat == memory.seat || dealt[ seat ] == nil { continue }
		hand := append( []Card( nil ) , dealt[ seat ]... )
		for _ , trick := range state.Completed {
			for _ , play := range trick.Plays {
				if play.Seat == seat { hand = append( hand , play.Card ) }
			}
		}
		for _ , play := range state.Trick.Plays {
			if play.Seat == seat { hand = append( hand , play.Card ) }
		}
		estimate := float64( hardEstimate( hand ) )
		// Bidding up to an amount says the hand looked worth at least that.
		if bid := state.LastBid[ seat ]; bid > 0 {
			score += logSigmoid( ( estimate - float64( bid ) + 5 )/readingScale )
		}
		// Passing says the next step up did not look worth it -- loosely,
		// since a seat also passes rather than outbid its own partner.
		if passed := state.PassedAt[ seat ]; passed > 0 {
			score += logSigmoid( ( float64( passed + BidStep + 10 ) - estimate )/readingScale )
		}
		// The trump call: the named color should be about the best one.
		if seat == state.Bidder && state.Trump != NoColor {
			named := hardTrumpScore( hand , state.Trump )
			best := named
			for _ , color := range Colors {
				if other := hardTrumpScore( hand , color ); other > best { best = other }
			}
			score += float64( named - best )/readingTrump
		}
	}
	return
}

func logSigmoid( x float64 ) ( result float64 ) {
	result = -math.Log1p( math.Exp( -x ) )
	return
}

// resample picks n indexes in proportion to exp(weights), by systematic
// resampling: one random offset, then evenly spaced draws along the running
// total, so the kept deals follow the weights closely.
func resample( weights []float64 , n int , source *rand.Rand ) ( picked []int ) {
	top := math.Inf( -1 )
	for _ , weight := range weights {
		if weight > top { top = weight }
	}
	total := 0.0
	scaled := make( []float64 , len( weights ) )
	for index , weight := range weights {
		scaled[ index ] = math.Exp( weight - top )
		total += scaled[ index ]
	}
	step := total / float64( n )
	at := source.Float64() * step
	running := 0.0
	index := 0
	for len( picked ) < n && index < len( scaled ) {
		running += scaled[ index ]
		for at < running && len( picked ) < n {
			picked = append( picked , index )
			at += step
		}
		index++
	}
	for len( picked ) < n {
		picked = append( picked , len( scaled )-1 )
	}
	return
}
