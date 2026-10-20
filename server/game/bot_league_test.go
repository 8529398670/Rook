package game

import (
	math "math"
	os "os"
	testing "testing"
)

// duel plays deals single hands twice each, with the two levels swapping
// seats on the second play, so neither level is helped by the cards: the
// same hands go to each in turn. It returns how many points per hand strong
// scored more than weak, on average.
func duel( t testing.TB , strong Level , weak Level , deals int , seed uint64 ) ( edge float64 ) {
	total := 0
	for deal := 0; deal < deals; deal++ {
		deck := ShuffledDeck( seeded( seed + uint64( deal ) ) )
		for swap := 0; swap < 2; swap++ {
			var levels [4]Level
			for seat := range levels {
				levels[ seat ] = weak
				if ( seat%2 == 0 ) == ( swap == 0 ) { levels[ seat ] = strong }
			}
			state , err := NewGameWithDeck( Options{ RookRank: RookHigh , RookFirstTrick: true } , deal%4 , deck )
			if err != nil { t.Fatal( err ) }
			for state.Phase != PhaseGameOver {
				seat := state.Turn
				if moved , err := state.BotMove( seat , levels[ seat ] ); err != nil || moved == false {
					t.Fatalf( "deal %d seat %d (%s) phase %s: %v" , deal , seat , levels[ seat ] , state.Phase , err )
				}
			}
			strong_team := swap
			total += state.Scores[ strong_team ] - state.Scores[ 1-strong_team ]
		}
	}
	edge = float64( total ) / float64( 2*deals )
	return
}

// TestBotLevelsAreInOrder checks each level beats the one below it. With
// ROOK_BOT_LEAGUE=1 it plays every neighbouring pair over thousands of
// deals (hundreds for the searching levels) and prints the margins, which is
// how the levels were tuned. Otherwise it plays few enough deals for every
// test run -- too few to see Master's +9 over Expert through the noise, so
// the quick run checks Master over Hard instead.
func TestBotLevelsAreInOrder( t *testing.T ) {
	type pair struct {
		strong , weak Level
		deals int
	}
	pairs := []pair{
		{ LevelNormal , LevelEasy , 150 },
		{ LevelSkilled , LevelNormal , 150 },
		{ LevelHard , LevelSkilled , 150 },
		{ LevelExpert , LevelHard , 12 },
		{ LevelMaster , LevelHard , 12 },
	}
	if os.Getenv( "ROOK_BOT_LEAGUE" ) != "" {
		pairs = []pair{
			{ LevelNormal , LevelEasy , 3000 },
			{ LevelSkilled , LevelNormal , 3000 },
			{ LevelHard , LevelSkilled , 3000 },
			{ LevelExpert , LevelHard , 300 },
			{ LevelMaster , LevelExpert , 300 },
		}
	} else if testing.Short() {
		t.Skip( "plays hundreds of hands" )
	}
	for _ , match := range pairs {
		edge := duel( t , match.strong , match.weak , match.deals , 7000 )
		t.Logf( "%s over %s: %+.1f points a hand over %d deals" , match.strong , match.weak , edge , match.deals )
		if edge <= 0 {
			t.Errorf( "%s did not beat %s: %+.1f a hand" , match.strong , match.weak , edge )
		}
	}
}

func BenchmarkExpertPlay( b *testing.B ) {
	state , err := NewGame( DefaultOptions() , 0 , seeded( 42 ) )
	if err != nil { b.Fatal( err ) }
	for state.Phase != PhasePlaying {
		if _ , err := state.BotMove( state.Turn , LevelHard ); err != nil { b.Fatal( err ) }
	}
	b.ResetTimer()
	for b.Loop() {
		state.expertPlay( state.Turn , false )
	}
}

func BenchmarkExpertBid( b *testing.B ) {
	state , err := NewGame( DefaultOptions() , 0 , seeded( 42 ) )
	if err != nil { b.Fatal( err ) }
	b.ResetTimer()
	for b.Loop() {
		state.expertBid( state.Turn , false )
	}
}

func BenchmarkMasterPlay( b *testing.B ) {
	state , err := NewGame( DefaultOptions() , 0 , seeded( 42 ) )
	if err != nil { b.Fatal( err ) }
	for state.Phase != PhasePlaying {
		if _ , err := state.BotMove( state.Turn , LevelHard ); err != nil { b.Fatal( err ) }
	}
	b.ResetTimer()
	for b.Loop() {
		state.expertPlay( state.Turn , true )
	}
}

// TestResampleFollowsTheWeights checks Master keeps imagined deals in
// proportion to how well they explain the table: a deal e times as likely
// is kept about e times as often, and an impossible one never.
func TestResampleFollowsTheWeights( t *testing.T ) {
	weights := []float64{ 0 , 1 , math.Inf( -1 ) , 0 }
	counts := make( []int , len( weights ) )
	for _ , index := range resample( weights , 10000 , seeded( 1 ) ) {
		counts[ index ]++
	}
	if counts[ 2 ] != 0 { t.Fatalf( "kept an impossible deal: %v" , counts ) }
	if ratio := float64( counts[ 1 ] ) / float64( counts[ 0 ] ); ratio < 2.6 || ratio > 2.85 {
		t.Fatalf( "weights not followed: %v (ratio %.2f)" , counts , ratio )
	}
}

// TestMasterReadsTheTrumpCall checks the reading itself: once the bidder
// has named trump, the deals Master keeps give the bidder more of it than
// deals drawn blind.
func TestMasterReadsTheTrumpCall( t *testing.T ) {
	state , err := NewGame( DefaultOptions() , 0 , seeded( 5 ) )
	if err != nil { t.Fatal( err ) }
	for state.Phase != PhasePlaying {
		if _ , err := state.BotMove( state.Turn , LevelHard ); err != nil { t.Fatal( err ) }
	}
	watcher := ( state.Bidder + 1 ) % 4
	memory := state.remember( watcher )
	trump := func( worlds []*State ) ( count int ) {
		for _ , world := range worlds {
			for _ , card := range world.Hands[ state.Bidder ] {
				if world.isTrump( card ) { count++ }
			}
		}
		return
	}
	blind := trump( memory.imagineMany( seeded( 6 ) , 400 , false ) )
	read := trump( memory.imagineMany( seeded( 6 ) , 400 , true ) )
	if read <= blind {
		t.Fatalf( "reading the trump call gave the bidder %d trump over 400 deals, blind gave %d" , read , blind )
	}
	t.Logf( "bidder's trump over 400 deals: %d blind, %d reading the table" , blind , read )
}
