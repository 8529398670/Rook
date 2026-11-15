package game

import (
	testing "testing"
)

// partnerSureTrick is hand 3 of a real game (table n28gFwtvQV37) up to move
// 162: Bartholomew (seat 2) bid 120, named black and buried every yellow he
// had. His partner Diane led the yellow 1 -- the top yellow, with eight
// yellows still out behind her -- and Tater Tot followed. Bartholomew is to
// play with no yellow left.
func partnerSureTrick( t *testing.T ) ( state *State ) {
	t.Helper()
	deck := cards( t ,
		"R8" , "R13" , "R14" , "Y2" , "Y5" , "Y7" , "Y8" , "G8" , "B5" , "B13" , "ROOK" , "G1" , "Y1" ,
		"R3" , "R5" , "R9" , "R10" , "R11" , "Y4" , "Y12" , "G5" , "G9" , "B10" , "B6" , "G4" , "Y3" ,
		"R1" , "G3" , "G6" , "G11" , "G12" , "B1" , "B2" , "B4" , "B7" , "B8" , "B12" , "B14" , "G10" ,
		"R2" , "R4" , "R6" , "R7" , "R12" , "Y9" , "Y14" , "G13" , "G14" , "B3" , "B11" , "B9" , "G7" ,
		"Y10" , "Y6" , "Y11" , "Y13" , "G2" ,
	)
	state , err := NewGameWithDeck( Options{ RookRank: RookHigh , RookFirstTrick: true , PlayTo: 500 } , 1 , deck )
	if err != nil { t.Fatal( err ) }
	state.HandNumber = 3
	state.Scores = [2]int{ 230 , -105 }
	for _ , bid := range []struct{ seat , amount int }{ { 1 , 70 } , { 2 , 75 } , { 3 , 80 } , { 0 , 110 } , { 1 , 115 } , { 2 , 120 } , { 3 , 0 } , { 0 , 0 } , { 1 , 0 } } {
		if bid.amount == 0 {
			err = state.Pass( bid.seat )
		} else {
			err = state.PlaceBid( bid.seat , bid.amount )
		}
		if err != nil { t.Fatalf( "seat %d bidding %d: %v" , bid.seat , bid.amount , err ) }
	}
	if err = state.Bury( 2 , cards( t , "Y10" , "Y6" , "Y11" , "Y13" , "G2" ) , Black ); err != nil { t.Fatal( err ) }
	for _ , play := range []struct{ seat int; id string }{
		{ 2 , "B7" } , { 3 , "B9" } , { 0 , "ROOK" } , { 1 , "B6" } ,
		{ 0 , "G1" } , { 1 , "G4" } , { 2 , "G10" } , { 3 , "G7" } ,
		{ 0 , "Y1" } , { 1 , "Y3" } ,
	} {
		mustPlay( t , state , play.seat , play.id )
	}
	if state.Turn != 2 { t.Fatalf( "seat %d to play, want 2" , state.Turn ) }
	return
}

// TestSearchDoesNotTrumpPartnersSureTrick: with a card worth nothing to
// throw, no level trumps a trick its partner is sure to take.
func TestSearchDoesNotTrumpPartnersSureTrick( t *testing.T ) {
	state := partnerSureTrick( t )
	for _ , level := range []Level{ LevelHard , LevelExpert , LevelMaster } {
		played := state.Clone().BotPlay( 2 , level )
		if state.isTrump( played ) {
			t.Errorf( "%s trumped its partner's sure trick with %s" , level , played.ID() )
		}
	}
}

// trumpDeck deals a strong black hand to seat 0, the bird with one low black
// to seat 1, and leaves B8 B10 R13 Y13 G13 in the kitty. rotate moves every
// hand round by that many seats, so the strong hand can sit anywhere.
func trumpDeck( t *testing.T , rotate int ) ( deck []Card ) {
	t.Helper()
	hands := [4][]string{
		{ "B1" , "B6" , "B9" , "B11" , "B12" , "B13" , "B14" , "R1" , "R14" , "Y1" , "Y14" , "G1" , "G14" },
		{ "ROOK" , "B3" , "R2" , "R3" , "R6" , "R9" , "Y4" , "Y6" , "Y9" , "G3" , "G5" , "G8" , "G9" },
		{ "B2" , "B4" , "R4" , "R5" , "R7" , "R8" , "Y2" , "Y3" , "Y5" , "Y7" , "G2" , "G4" , "G6" },
		{ "B5" , "B7" , "R10" , "R11" , "R12" , "Y8" , "Y10" , "Y11" , "Y12" , "G7" , "G10" , "G11" , "G12" },
	}
	for seat := 0; seat < 4; seat++ {
		deck = append( deck , cards( t , hands[ ( seat+4-rotate )%4 ]... )... )
	}
	deck = append( deck , cards( t , "B8" , "B10" , "R13" , "Y13" , "G13" )... )
	return
}

// TestBotsDoNotOutbidAnUncontestedPartner is the feedback that started it:
// both opponents passed, and the bot partner kept raising its person to
// 175. Seat 2 holds the strong hand; seat 0 holds the bid, 1 and 3 passed.
func TestBotsDoNotOutbidAnUncontestedPartner( t *testing.T ) {
	state , err := NewGameWithDeck( Options{ RookRank: RookHigh , RookFirstTrick: true , PlayTo: 500 } , 0 , trumpDeck( t , 2 ) )
	if err != nil { t.Fatal( err ) }
	for _ , bid := range []struct{ seat , amount int }{ { 0 , 100 } , { 1 , 0 } , { 2 , 105 } , { 3 , 0 } , { 0 , 110 } } {
		if bid.amount == 0 {
			err = state.Pass( bid.seat )
		} else {
			err = state.PlaceBid( bid.seat , bid.amount )
		}
		if err != nil { t.Fatalf( "seat %d bidding %d: %v" , bid.seat , bid.amount , err ) }
	}
	if state.Turn != 2 || state.PartnerUncontested( 2 ) == false { t.Fatalf( "seat %d to bid, uncontested %v" , state.Turn , state.PartnerUncontested( 2 ) ) }
	for _ , level := range Levels {
		if amount := state.Clone().BotBid( 2 , level ); amount != 0 {
			t.Errorf( "%s raised its uncontested partner to %d" , level , amount )
		}
	}
	// Without the rule this hand does raise: the setting really changes
	// something.
	for _ , level := range []Level{ LevelNormal , LevelHard } {
		if amount := state.Clone().BotBidOutbidPartner( 2 , level ); amount == 0 {
			t.Errorf( "%s with the rule off passed a hand it should raise" , level )
		}
	}
	// With an opponent still in, the partner may still raise.
	state.Passed[ 3 ] = false
	if state.PartnerUncontested( 2 ) { t.Fatalf( "uncontested with seat 3 still bidding" ) }
}

// defending puts the strong hand's owner, seat 0, in as bidder with black
// trump and seat 1 -- the bird, B3 and three other colors -- defending.
func defending( t *testing.T ) ( state *State ) {
	t.Helper()
	state , err := NewGameWithDeck( Options{ RookRank: RookHigh , RookFirstTrick: true , PlayTo: 500 } , 0 , trumpDeck( t , 0 ) )
	if err != nil { t.Fatal( err ) }
	if err = state.PlaceBid( 0 , 120 ); err != nil { t.Fatal( err ) }
	for seat := 1; seat < 4; seat++ {
		if err = state.Pass( seat ); err != nil { t.Fatal( err ) }
	}
	if err = state.Bury( 0 , cards( t , "R13" , "Y13" , "G13" , "R14" , "Y14" ) , Black ); err != nil { t.Fatal( err ) }
	return
}

// TestDefendersHoldTheBird: the bidder leads a low trump worth nothing.
// The defender with the bird and a low trump keeps the bird for a trick
// worth catching -- and takes the bidder's trump 1 with it.
func TestDefendersHoldTheBird( t *testing.T ) {
	low := defending( t )
	mustPlay( t , low , 0 , "B6" )
	high := defending( t )
	mustPlay( t , high , 0 , "B1" )
	for _ , level := range []Level{ LevelSkilled , LevelHard , LevelExpert , LevelMaster } {
		if played := low.Clone().BotPlay( 1 , level ); played.IsRook() {
			t.Errorf( "%s spent the bird on the bidder's B6" , level )
		}
		if played := high.Clone().BotPlay( 1 , level ); played.IsRook() == false {
			t.Errorf( "%s let the bidder's B1 go with the bird in hand (played %s)" , level , played.ID() )
		}
	}
}

// TestDefendersDoNotLeadTrump: on lead with other colors in hand, no level
// that plays the guide leads trump against the bidder.
func TestDefendersDoNotLeadTrump( t *testing.T ) {
	state := defending( t )
	for _ , play := range []string{ "R1" , "R2" , "R4" , "R10" } {
		mustPlay( t , state , state.Turn , play )
	}
	// The bidder won with the R1; it leads, and the defender gets the
	// lead after it takes a trick of its own.
	mustPlay( t , state , 0 , "G1" )
	mustPlay( t , state , 1 , "G3" )
	mustPlay( t , state , 2 , "G2" )
	mustPlay( t , state , 3 , "G7" )
	mustPlay( t , state , 0 , "Y1" )
	mustPlay( t , state , 1 , "Y4" )
	mustPlay( t , state , 2 , "Y2" )
	mustPlay( t , state , 3 , "Y8" )
	// Hand the lead to seat 1 directly: the point is what it leads.
	state.Turn = 1
	state.Trick = Trick{ Leader: 1 , Winner: -1 }
	for _ , level := range []Level{ LevelSkilled , LevelHard , LevelExpert , LevelMaster } {
		if played := state.Clone().BotPlay( 1 , level ); state.isTrump( played ) {
			t.Errorf( "%s led trump (%s) defending" , level , played.ID() )
		}
	}
}
