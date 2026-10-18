package tables

import (
	errors "errors"
	testing "testing"
)

func TestRestartAndDelete( t *testing.T ) {
	hub , store , _ := testHub( t )
	host := testUser( t , store , "Ana" )
	guest := testUser( t , store , "Ben" )
	id := soloTable( t , hub , host )
	playUntilMoves( t , hub , id , host , 20 )

	if err := hub.Restart( id , guest , false ); !errors.Is( err , ErrNotHost ) {
		t.Fatalf( "only the host restarts: %v" , err )
	}
	if err := hub.Delete( id , guest , false ); !errors.Is( err , ErrNotHost ) {
		t.Fatalf( "only the host deletes: %v" , err )
	}

	if err := hub.Restart( id , host , false ); err != nil { t.Fatal( err ) }
	view , _ := hub.View( id , Viewer{ User: host , Admin: true } , ViewOptions{} )
	if view.Game == nil || view.Game.HandNumber != 1 || view.Game.Scores != [2]int{} || view.Moves > 3 || view.Held {
		t.Fatalf( "after restart: game %+v moves %d held %v" , view.Game , view.Moves , view.Held )
	}

	// With a seat empty, a restart goes back to the waiting room.
	hub.Undo( id ) // hold it so the bots stand still
	table := hub.table( id )
	table.mu.Lock()
	table.Seats[ 2 ] = Seat{}
	table.mu.Unlock()
	if err := hub.Restart( id , guest , true ); err != nil { t.Fatal( err ) } // an admin may
	if waiting , _ := hub.View( id , Viewer{ User: host } , ViewOptions{} ); waiting.Status != StatusWaiting || waiting.Game != nil {
		t.Fatalf( "restart with an empty seat: status %s" , waiting.Status )
	}

	if err := hub.Delete( id , host , false ); err != nil { t.Fatal( err ) }
	if _ , err := hub.View( id , Viewer{ User: host } , ViewOptions{} ); !errors.Is( err , ErrNoSuchTable ) {
		t.Fatalf( "deleted table still there: %v" , err )
	}
	if err := hub.Delete( id , host , false ); !errors.Is( err , ErrNoSuchTable ) {
		t.Fatalf( "deleting twice: %v" , err )
	}
}
