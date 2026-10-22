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
