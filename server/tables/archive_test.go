package tables

import (
	errors "errors"
	strings "strings"
	testing "testing"
	time "time"

	game "rook/server/game"
	models "rook/server/models"
)

// oneHandTable is one person against three bots in a game decided by a
// single hand, so it can be played to the end quickly.
func oneHandTable( t *testing.T , hub *Hub , person *models.User ) ( id string ) {
	t.Helper()
	options := game.DefaultOptions()
	options.PlayTo = 0
	id , err := hub.Create( person , "Quick" , options )
	if err != nil { t.Fatal( err ) }
	for seat := 1; seat < 4; seat++ {
		if err := hub.AddBot( id , person , seat ); err != nil { t.Fatal( err ) }
	}
	if err := hub.Start( id , person , DealerHost ); err != nil { t.Fatal( err ) }
	return
}

func playToTheEnd( t *testing.T , hub *Hub , id string , person *models.User ) {
	t.Helper()
	deadline := time.Now().Add( 10 * time.Second )
	for {
		view , _ := hub.View( id , Viewer{ User: person } , ViewOptions{} )
		g := view.Game
		if g.Phase == game.PhaseGameOver { return }
		if time.Now().After( deadline ) { t.Fatalf( "stuck in %s" , g.Phase ) }
		if g.Turn == 0 {
			switch g.Phase {
			case game.PhaseBidding:
				hub.Pass( id , person )
			case game.PhaseKitty:
				hub.Bury( id , person , g.Hand[ :5 ] , "red" )
			case game.PhasePlaying:
				hub.Play( id , person , g.Legal[ 0 ] )
			}
		}
		if g.Phase == game.PhaseHandOver { hub.Continue( id , person ) }
		time.Sleep( 2 * time.Millisecond )
	}
}

func games( t *testing.T , hub *Hub ) ( list []GameSummary ) {
	t.Helper()
	list , err := hub.Games( 0 , 100 )
	if err != nil { t.Fatal( err ) }
	return
}

func TestAFinishedGameIsArchivedOnce( t *testing.T ) {
	hub , store , _ := testHub( t )
	host := testUser( t , store , "Ana" )
	id := oneHandTable( t , hub , host )
	playToTheEnd( t , hub , id , host )

	// More changes after the end (chat) must not archive it again.
	if err := hub.Chat( id , host , "gg" ); err != nil { t.Fatal( err ) }
	list := games( t , hub )
	if len( list ) != 1 || list[ 0 ].Status != GameFinished || list[ 0 ].Winner < 0 || list[ 0 ].TableName != "Quick" {
		t.Fatalf( "after one game: %+v" , list )
	}
	if list[ 0 ].Seats[ 0 ].Name != "Ana" || list[ 0 ].Seats[ 1 ].Kind != KindBot {
		t.Fatalf( "seats: %+v" , list[ 0 ].Seats )
	}
	full , err := hub.Game( list[ 0 ].ID )
	if err != nil || len( full.Record.Hands ) != 1 || full.Record.Hands[ 0 ].Result == nil {
		t.Fatalf( "full record: %+v %v" , full.Record , err )
	}

	// An admin undo reopens the game: its row goes until it ends again.
	if err := hub.Undo( id ); err != nil { t.Fatal( err ) }
	if list := games( t , hub ); len( list ) != 0 {
		t.Fatalf( "an undone ending is still archived: %+v" , list )
	}
	if err := hub.Redo( id ); err != nil { t.Fatal( err ) }
	if list := games( t , hub ); len( list ) != 1 {
		t.Fatalf( "redoing the ending: %d rows" , len( list ) )
	}

	// Deleting the table keeps the finished game, and does not add a row.
	if err := hub.Delete( id , host , false ); err != nil { t.Fatal( err ) }
	if list := games( t , hub ); len( list ) != 1 || list[ 0 ].Status != GameFinished {
		t.Fatalf( "after delete: %+v" , list )
	}
	if _ , err := hub.Game( "nope" ); !errors.Is( err , ErrNoSuchGame ) {
		t.Fatalf( "unknown game: %v" , err )
	}
}

func TestAbandonedGamesAreArchived( t *testing.T ) {
	hub , store , _ := testHub( t )
	host := testUser( t , store , "Ana" )

	// Restarted mid-game.
	id := soloTable( t , hub , host )
	playUntilMoves( t , hub , id , host , 6 )
	if err := hub.Restart( id , host , false ); err != nil { t.Fatal( err ) }
	if list := games( t , hub ); len( list ) != 1 || list[ 0 ].Status != GameAbandoned || list[ 0 ].Winner != -1 {
		t.Fatalf( "after restart: %+v" , list )
	}

	// Deleted mid-game.
	if err := hub.Delete( id , host , false ); err != nil { t.Fatal( err ) }
	if list := games( t , hub ); len( list ) != 2 || list[ 0 ].Status != GameAbandoned {
		t.Fatalf( "after delete: %+v" , list )
	}

	// The last person leaving.
	other := soloTable( t , hub , host )
	if err := hub.Leave( other , host ); err != nil { t.Fatal( err ) }
	if list := games( t , hub ); len( list ) != 3 {
		t.Fatalf( "after leaving: %d rows" , len( list ) )
	}

	// A table that never dealt has nothing to keep.
	empty , _ := hub.Create( host , "Empty" , game.DefaultOptions() )
	if err := hub.Delete( empty , host , false ); err != nil { t.Fatal( err ) }
	if list := games( t , hub ); len( list ) != 3 {
		t.Fatalf( "an undealt table was archived: %d rows" , len( list ) )
	}
}

func TestWatchersAndGuestsChat( t *testing.T ) {
	hub , store , _ := testHub( t )
	host := testUser( t , store , "Ana" )
	watcher := testUser( t , store , "Ben" )
	id := soloTable( t , hub , host )

	if err := hub.Chat( id , watcher , "hello from the rail" ); err != nil { t.Fatal( err ) }
	if err := hub.GuestChat( id , "  Cleo   M " , "hi all" ); err != nil { t.Fatal( err ) }
	for _ , bad := range []string{ "" , "   " , strings.Repeat( "x" , maxGuestNameLength+1 ) , "a‮b" } {
		if err := hub.GuestChat( id , bad , "hi" ); !errors.Is( err , ErrBadGuestName ) {
			t.Fatalf( "guest name %q: %v" , bad , err )
		}
	}
	if err := hub.GuestChat( id , "Cleo" , "  " ); !errors.Is( err , ErrBadChat ) {
		t.Fatalf( "empty guest message: %v" , err )
	}

	view , _ := hub.View( id , Viewer{} , ViewOptions{} )
	if len( view.Chat ) != 2 {
		t.Fatalf( "chat: %+v" , view.Chat )
	}
	signed , guest := view.Chat[ 0 ] , view.Chat[ 1 ]
	if signed.Seat != -1 || signed.Watcher == false || signed.Guest || signed.Name != "Ben" {
		t.Fatalf( "signed-in watcher line: %+v" , signed )
	}
	if guest.Seat != -1 || guest.Guest == false || guest.Name != "Cleo M" {
		t.Fatalf( "guest line: %+v" , guest )
	}
}

func TestForgetUserEmptiesSeats( t *testing.T ) {
	hub , store , _ := testHub( t )
	host := testUser( t , store , "Ana" )
	guest := testUser( t , store , "Ben" )
	id , _ := hub.Create( host , "Pair" , game.DefaultOptions() )
	if err := hub.Sit( id , guest , 1 ); err != nil { t.Fatal( err ) }

	hub.ForgetUser( guest.ID )
	view , _ := hub.View( id , Viewer{ User: host } , ViewOptions{} )
	if view.Seats[ 1 ].Kind != KindEmpty {
		t.Fatalf( "seat still held: %+v" , view.Seats[ 1 ] )
	}

	// The host gone too: nobody is left, so the table goes.
	hub.ForgetUser( host.ID )
	if _ , err := hub.View( id , Viewer{} , ViewOptions{} ); !errors.Is( err , ErrNoSuchTable ) {
		t.Fatalf( "empty table kept: %v" , err )
	}
}
