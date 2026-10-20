package game

import (
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

// TestBotLevelsAreInOrder checks each level beats the one below it. The
// samples are small enough for every test run; ROOK_BOT_LEAGUE=1 plays many
// more deals and prints the margins, which is how the levels were tuned.
func TestBotLevelsAreInOrder( t *testing.T ) {
	deals := map[ Level ]int{ LevelNormal: 150 , LevelHard: 150 , LevelExpert: 12 }
	if os.Getenv( "ROOK_BOT_LEAGUE" ) != "" {
		deals = map[ Level ]int{ LevelNormal: 3000 , LevelHard: 3000 , LevelExpert: 300 }
	} else if testing.Short() {
		t.Skip( "plays hundreds of hands" )
	}
	for index := 1; index < len( Levels ); index++ {
		strong , weak := Levels[ index ] , Levels[ index-1 ]
		edge := duel( t , strong , weak , deals[ strong ] , 7000 )
		t.Logf( "%s over %s: %+.1f points a hand over %d deals" , strong , weak , edge , deals[ strong ] )
		if edge <= 0 {
			t.Errorf( "%s did not beat %s: %+.1f a hand" , strong , weak , edge )
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
		state.expertPlay( state.Turn )
	}
}

func BenchmarkExpertBid( b *testing.B ) {
	state , err := NewGame( DefaultOptions() , 0 , seeded( 42 ) )
	if err != nil { b.Fatal( err ) }
	b.ResetTimer()
	for b.Loop() {
		state.expertBid( state.Turn )
	}
}
