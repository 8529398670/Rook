package tables

import (
	errors "errors"
	testing "testing"

	game "rook/server/game"
)

func TestAutoPlayFinishesTheHandAndTurnsOffAtTheDeal( t *testing.T ) {
	hub , store , _ := testHub( t )
	person := testUser( t , store , "Ana" )
	id := soloTable( t , hub , person )

	// Bidding and the kitty are real decisions: no auto play yet.
	view := waitFor( t , hub , id , person , func( view TableView ) bool { return view.Game.Turn == 0 } )
	if view.Game.Phase != game.PhaseBidding { t.Fatalf( "phase %s, want bidding" , view.Game.Phase ) }
	if err := hub.SetAutoPlay( id , person , true ); !errors.Is( err , ErrNotPlaying ) {
		t.Fatalf( "auto play during bidding: %v" , err )
	}

	// Get to the cards, then switch it on and let the server play them.
	for {
		view = waitFor( t , hub , id , person , func( view TableView ) bool {
			return view.Game.Turn == 0 || view.Game.Phase == game.PhaseHandOver
		} )
		if view.Game.Phase == game.PhasePlaying { break }
		switch view.Game.Phase {
		case game.PhaseBidding:
			hub.Pass( id , person )
		case game.PhaseKitty:
			hub.Bury( id , person , view.Game.Hand[ :5 ] , "red" )
		default:
			t.Fatalf( "hand ended before any cards: %s" , view.Game.Phase )
		}
	}
	if err := hub.SetAutoPlay( id , person , true ); err != nil { t.Fatal( err ) }
	view , _ = hub.View( id , Viewer{ User: person } , ViewOptions{} )
	if view.AutoPlay == false || view.Seats[ 0 ].AutoPlay == false {
		t.Fatalf( "auto play not shown: you %v seat %v" , view.AutoPlay , view.Seats[ 0 ].AutoPlay )
	}

	// Nobody touches seat 0 again, yet the hand reaches its end.
	view = waitFor( t , hub , id , person , func( view TableView ) bool { return view.Game.Phase == game.PhaseHandOver } )
	if view.AutoPlay {
		t.Fatalf( "auto play still shown after the last trick" )
	}

	// The next deal starts with it off, and nothing plays for seat 0.
	if err := hub.Continue( id , person ); err != nil { t.Fatal( err ) }
	view = waitFor( t , hub , id , person , func( view TableView ) bool { return view.Game.HandNumber == 2 } )
	if view.AutoPlay || view.Seats[ 0 ].AutoPlay {
		t.Fatalf( "auto play carried into the next hand" )
	}
	table := hub.table( id )
	table.mu.Lock()
	left := len( table.AutoPlay )
	table.mu.Unlock()
	if left != 0 { t.Fatalf( "stale auto play entries: %d" , left ) }
}

func TestAutoPlayStaysWithThePersonNotTheSeat( t *testing.T ) {
	hub , store , _ := testHub( t )
	person := testUser( t , store , "Ana" )
	friend := testUser( t , store , "Dee" )
	id := soloTable( t , hub , person )

	table := hub.table( id )
	table.mu.Lock()
	defer table.mu.Unlock()
	table.Game.Phase = game.PhasePlaying
	table.AutoPlay = map[ uint64 ]int{ person.ID: table.Game.HandNumber }
	if table.autoPlays( 0 ) == false { t.Fatalf( "auto play should be on for Ana" ) }

	// Someone else in that chair does not inherit it.
	table.Seats[ 0 ] = Seat{}
	fillSeat( &table.Seats[ 0 ] , friend )
	if table.autoPlays( 0 ) { t.Fatalf( "Dee inherited Ana's auto play" ) }

	// A bot seat is never "auto play"; it is just a bot.
	if table.autoPlays( 1 ) { t.Fatalf( "bot seat reported auto play" ) }
}

// TestAutoPlayUsesThePersonsLevel checks auto play plays at the level the
// person chose -- the best there is until they choose -- and that changing
// it while auto play is on takes effect at once.
func TestAutoPlayUsesThePersonsLevel( t *testing.T ) {
	hub , store , _ := testHub( t )
	person := testUser( t , store , "Ana" )
	id := soloTable( t , hub , person )

	table := hub.table( id )
	table.mu.Lock()
	table.Game.Phase = game.PhasePlaying
	table.mu.Unlock()

	level := func() ( result game.Level ) {
		table.mu.Lock()
		result = table.moveLevel( 0 )
		table.mu.Unlock()
		return
	}
	if err := hub.SetAutoPlay( id , person , true ); err != nil { t.Fatal( err ) }
	if got := level(); got != game.BestLevel { t.Fatalf( "unset level plays at %s, want %s" , got , game.BestLevel ) }

	person.AutoPlayLevel = "skilled"
	if err := hub.SetAutoPlay( id , person , true ); err != nil { t.Fatal( err ) }
	if got := level(); got != game.LevelSkilled { t.Fatalf( "chosen level plays at %s" , got ) }

	person.AutoPlayLevel = "easy"
	hub.RefreshUser( person )
	if got := level(); got != game.LevelEasy { t.Fatalf( "a new level while on plays at %s" , got ) }

	if err := hub.SetAutoPlay( id , person , false ); err != nil { t.Fatal( err ) }
	table.mu.Lock()
	_ , kept := table.AutoPlayLevel[ person.ID ]
	table.mu.Unlock()
	if kept { t.Fatalf( "the level stayed after auto play went off" ) }
}
