package tables

import (
	errors "errors"
	testing "testing"
	time "time"

	config "rook/server/config"
	db "rook/server/db"
	game "rook/server/game"
	models "rook/server/models"
)

func testHub( t *testing.T ) ( hub *Hub , store *db.Store , cfg *config.Config ) {
	t.Helper()
	t.Setenv( "APP_DIR" , t.TempDir() )
	t.Setenv( "BOT_DELAY_MS" , "0" )
	cfg , err := config.Load()
	if err != nil { t.Fatal( err ) }
	store , err = db.Open( cfg )
	if err != nil { t.Fatal( err ) }
	t.Cleanup( func() { store.Close() } )
	hub , err = New( store , cfg )
	if err != nil { t.Fatal( err ) }
	t.Cleanup( hub.Close )
	return
}

func testUser( t *testing.T , store *db.Store , name string ) ( user *models.User ) {
	t.Helper()
	user , err := models.CreateUser( store , name , models.RoleUser )
	if err != nil { t.Fatal( err ) }
	return
}

// waitFor polls the view until ready says so; bots move on timers.
func waitFor( t *testing.T , hub *Hub , id string , user *models.User , ready func( view TableView ) bool ) ( view TableView ) {
	t.Helper()
	deadline := time.Now().Add( 10 * time.Second )
	for {
		var err error
		view , err = hub.View( id , Viewer{ User: user } , ViewOptions{} )
		if err != nil { t.Fatal( err ) }
		if ready( view ) { return }
		if time.Now().After( deadline ) {
			t.Fatalf( "timed out; status %s game %+v" , view.Status , view.Game )
		}
		time.Sleep( 2 * time.Millisecond )
	}
}

func TestSeatingRules( t *testing.T ) {
	hub , store , _ := testHub( t )
	host := testUser( t , store , "Ana" )
	guest := testUser( t , store , "Ben" )

	id , err := hub.Create( host , "  Friday cards  " , game.DefaultOptions() )
	if err != nil { t.Fatal( err ) }
	view , _ := hub.View( id , Viewer{ User: host } , ViewOptions{} )
	if view.Name != "Friday cards" || view.You != 0 || view.IsHost == false || view.Status != StatusWaiting {
		t.Fatalf( "unexpected new table %+v" , view )
	}

	if _ , err := hub.Create( host , "   " , game.DefaultOptions() ); !errors.Is( err , ErrBadName ) {
		t.Fatalf( "blank name: %v" , err )
	}
	if err := hub.Sit( id , guest , 0 ); !errors.Is( err , ErrSeatTaken ) {
		t.Fatalf( "taken seat: %v" , err )
	}
	if err := hub.Sit( id , guest , 2 ); err != nil { t.Fatal( err ) }
	if err := hub.Sit( id , guest , 1 ); !errors.Is( err , ErrAlreadySeated ) {
		t.Fatalf( "second seat: %v" , err )
	}
	if err := hub.AddBot( id , guest , 1 ); err != nil { t.Fatal( err ) }
	if err := hub.Start( id , guest , DealerHost ); !errors.Is( err , ErrNotHost ) {
		t.Fatalf( "non-host start: %v" , err )
	}
	if err := hub.Start( id , host , DealerHost ); !errors.Is( err , ErrSeatsNotFull ) {
		t.Fatalf( "start with an empty seat: %v" , err )
	}

	// The host leaving hands hosting to the next person seated.
	if err := hub.Leave( id , host ); err != nil { t.Fatal( err ) }
	if view , _ := hub.View( id , Viewer{ User: guest } , ViewOptions{} ); view.IsHost == false {
		t.Fatalf( "hosting was not handed on" )
	}
	// The last person leaving removes the table, bots or not.
	if err := hub.Leave( id , guest ); err != nil { t.Fatal( err ) }
	if _ , err := hub.View( id , Viewer{ User: guest } , ViewOptions{} ); !errors.Is( err , ErrNoSuchTable ) {
		t.Fatalf( "abandoned table still there: %v" , err )
	}
	stored , _ := models.ListTables( store )
	if len( stored ) != 0 {
		t.Fatalf( "abandoned table still saved" )
	}
}

// Whoever presses Deal deals the first hand, so the seat on their left
// opens the bidding -- not the dealer.
func TestTheHostDealsTheFirstHand( t *testing.T ) {
	hub , store , _ := testHub( t )
	ana := testUser( t , store , "Ana" )
	ben := testUser( t , store , "Ben" )
	id , _ := hub.Create( ana , "Cards" , game.DefaultOptions() )
	if err := hub.Sit( id , ben , 2 ); err != nil { t.Fatal( err ) }
	// Ana steps away and back, which leaves Ben -- in seat 2 -- hosting.
	if err := hub.Leave( id , ana ); err != nil { t.Fatal( err ) }
	if err := hub.Sit( id , ana , 0 ); err != nil { t.Fatal( err ) }
	for _ , seat := range []int{ 1 , 3 } {
		if err := hub.AddBot( id , ben , seat ); err != nil { t.Fatal( err ) }
	}
	if err := hub.Start( id , ben , DealerHost ); err != nil { t.Fatal( err ) }
	view , _ := hub.View( id , Viewer{ User: ana } , ViewOptions{} )
	if view.Game.Dealer != 2 || view.Game.BidStarter != 3 {
		t.Fatalf( "dealer %d first bidder %d, want 2 and 3" , view.Game.Dealer , view.Game.BidStarter )
	}
}

// The host can instead name any seat to deal first, or have one drawn.
func TestTheHostPicksTheFirstDealer( t *testing.T ) {
	hub , store , _ := testHub( t )
	ana := testUser( t , store , "Ana" )
	table := func() ( id string ) {
		id , _ = hub.Create( ana , "Cards" , game.DefaultOptions() )
		for seat := 1; seat < 4; seat++ {
			if err := hub.AddBot( id , ana , seat ); err != nil { t.Fatal( err ) }
		}
		return
	}

	id := table()
	if err := hub.Start( id , ana , 4 ); !errors.Is( err , ErrBadSeat ) {
		t.Fatalf( "dealer seat 4: %v" , err )
	}
	if err := hub.Start( id , ana , 3 ); err != nil { t.Fatal( err ) }
	view , _ := hub.View( id , Viewer{ User: ana } , ViewOptions{} )
	if view.Game.Dealer != 3 || view.Game.BidStarter != 0 {
		t.Fatalf( "dealer %d first bidder %d, want 3 and 0" , view.Game.Dealer , view.Game.BidStarter )
	}

	seen := map[ int ]bool{}
	for draw := 0; draw < 40 && len( seen ) < 4; draw++ {
		id = table()
		if err := hub.Start( id , ana , DealerRandom ); err != nil { t.Fatal( err ) }
		view , _ = hub.View( id , Viewer{ User: ana } , ViewOptions{} )
		seen[ view.Game.Dealer ] = true
		hub.Delete( id , ana , false )
	}
	if len( seen ) < 2 {
		t.Fatalf( "random dealer never varied: %v" , seen )
	}
}

func TestInvitedSeatIsHeldUntilTheyArrive( t *testing.T ) {
	hub , store , _ := testHub( t )
	host := testUser( t , store , "Ana" )
	invitee := testUser( t , store , "Cy" )
	id , _ := hub.Create( host , "Cards" , game.DefaultOptions() )

	if err := hub.Reserve( id , 3 , invitee ); err != nil { t.Fatal( err ) }
	if err := hub.CanReserve( id , 3 ); !errors.Is( err , ErrSeatTaken ) {
		t.Fatalf( "reserved seat offered again: %v" , err )
	}
	if view , _ := hub.View( id , Viewer{ User: host } , ViewOptions{} ); view.Seats[ 3 ].Pending == false || view.Seats[ 3 ].Name != "Cy" {
		t.Fatalf( "seat not held: %+v" , view.Seats[ 3 ] )
	}
	subscriber , err := hub.SubscribeTable( id , invitee.ID , false )
	if err != nil { t.Fatal( err ) }
	defer hub.UnsubscribeTable( id , subscriber )
	view , _ := hub.View( id , Viewer{ User: invitee } , ViewOptions{} )
	if view.You != 3 || view.Seats[ 3 ].Pending || view.Seats[ 3 ].Online == false {
		t.Fatalf( "arrival not recorded: you %d seat %+v" , view.You , view.Seats[ 3 ] )
	}
}

// TestAHandWithBots plays one person through a whole hand against three
// computer players, then checks the game survives a reload and that leaving
// mid-game pauses rather than ends it.
func TestAHandWithBots( t *testing.T ) {
	hub , store , cfg := testHub( t )
	person := testUser( t , store , "Ana" )
	options := game.DefaultOptions()
	options.PlayTo = 300
	id , _ := hub.Create( person , "Solo" , options )
	for seat := 1; seat < 4; seat++ {
		if err := hub.AddBot( id , person , seat ); err != nil { t.Fatal( err ) }
	}
	if err := hub.Start( id , person , DealerHost ); err != nil { t.Fatal( err ) }

	for {
		view := waitFor( t , hub , id , person , func( view TableView ) bool {
			return view.Game.Phase == game.PhaseHandOver || view.Game.Turn == 0
		} )
		if view.Game.Phase == game.PhaseHandOver { break }

		var err error
		switch view.Game.Phase {
		case game.PhaseBidding:
			err = hub.Pass( id , person )
		case game.PhaseKitty:
			if len( view.Game.PickedUp ) != 5 || len( view.Game.Hand ) != 18 {
				t.Fatalf( "bidder should see the kitty in hand" )
			}
			err = hub.Bury( id , person , view.Game.Hand[ :5 ] , "red" )
		case game.PhasePlaying:
			if len( view.Game.Legal ) == 0 { t.Fatalf( "my turn with no legal cards" ) }
			err = hub.Play( id , person , view.Game.Legal[ 0 ] )
		}
		if err != nil { t.Fatalf( "%s: %v" , view.Game.Phase , err ) }
	}

	// Bots confirm the summary at once; only the person is waited on.
	view , _ := hub.View( id , Viewer{ User: person } , ViewOptions{} )
	if view.Game.Ready != [4]bool{ false , true , true , true } {
		t.Fatalf( "ready flags %v" , view.Game.Ready )
	}
	if len( view.Game.History ) != 1 || view.Game.History[ 0 ].Taken[ 0 ]+view.Game.History[ 0 ].Taken[ 1 ] != 180 {
		t.Fatalf( "bad score sheet %+v" , view.Game.History )
	}
	if err := hub.Continue( id , person ); err != nil { t.Fatal( err ) }
	view = waitFor( t , hub , id , person , func( view TableView ) bool { return view.Game.HandNumber == 2 } )
	version := view.Version

	// A restart picks the game up exactly where it was.
	hub.Close()
	reloaded , err := New( store , cfg )
	if err != nil { t.Fatal( err ) }
	defer reloaded.Close()
	again , err := reloaded.View( id , Viewer{ User: person } , ViewOptions{} )
	if err != nil || again.Version < version || again.Game.HandNumber != 2 || len( again.Game.Hand ) == 0 {
		t.Fatalf( "reload lost the game: %v %+v" , err , again.Game )
	}

	// Removing a bot mid-game pauses the table; a person sitting resumes it.
	if err := reloaded.RemoveBot( id , person , 2 ); err != nil { t.Fatal( err ) }
	paused , _ := reloaded.View( id , Viewer{ User: person } , ViewOptions{} )
	if paused.Status != StatusPaused {
		t.Fatalf( "status %s, want paused" , paused.Status )
	}
	if paused.Game.Turn == 0 && paused.Game.Phase == game.PhaseBidding {
		if err := reloaded.Pass( id , person ); !errors.Is( err , ErrPaused ) {
			t.Fatalf( "moved while paused: %v" , err )
		}
	}
	friend := testUser( t , store , "Dee" )
	if err := reloaded.Sit( id , friend , 2 ); err != nil { t.Fatal( err ) }
	resumed , _ := reloaded.View( id , Viewer{ User: friend } , ViewOptions{} )
	if resumed.Status != StatusPlaying || resumed.You != 2 || len( resumed.Game.Hand ) == 0 {
		t.Fatalf( "takeover failed: %s you %d hand %d" , resumed.Status , resumed.You , len( resumed.Game.Hand ) )
	}
}

func TestRemovePlayer( t *testing.T ) {
	hub , store , _ := testHub( t )
	host := testUser( t , store , "Ana" )
	guest := testUser( t , store , "Ben" )
	third := testUser( t , store , "Cy" )
	invited := testUser( t , store , "Di" )

	id , err := hub.Create( host , "Kick test" , game.DefaultOptions() )
	if err != nil { t.Fatal( err ) }
	if err = hub.Sit( id , guest , 1 ); err != nil { t.Fatal( err ) }
	if err = hub.Sit( id , third , 2 ); err != nil { t.Fatal( err ) }
	if err = hub.Reserve( id , 3 , invited ); err != nil { t.Fatal( err ) }
	if err = hub.Reserve( id , 3 , guest ); errors.Is( err , ErrSeatTaken ) == false { t.Fatalf( "reserve taken seat: %v" , err ) }

	if err = hub.RemovePlayer( id , guest , false , 2 ); errors.Is( err , ErrNotHost ) == false { t.Fatalf( "non-host remove: %v" , err ) }
	if err = hub.RemovePlayer( id , host , false , 0 ); errors.Is( err , ErrBadSeat ) == false { t.Fatalf( "remove self: %v" , err ) }

	// A held seat counts: removing it cancels the invite.
	if err = hub.RemovePlayer( id , host , false , 3 ); err != nil { t.Fatal( err ) }
	if err = hub.RemovePlayer( id , host , false , 3 ); errors.Is( err , ErrNotAPlayer ) == false { t.Fatalf( "remove empty: %v" , err ) }

	// Someone already at the table cannot be held a second seat.
	if err = hub.Reserve( id , 3 , guest ); errors.Is( err , ErrAlreadySeated ) == false { t.Fatalf( "reserve seated: %v" , err ) }
	if err = hub.AddBot( id , host , 3 ); err != nil { t.Fatal( err ) }
	if err = hub.Start( id , host , DealerHost ); err != nil { t.Fatal( err ) }

	// Mid-game, removing a player pauses the table. An admin may do it too.
	if err = hub.RemovePlayer( id , invited , true , 1 ); err != nil { t.Fatal( err ) }
	view , _ := hub.View( id , Viewer{ User: host } , ViewOptions{} )
	if view.Status != StatusPaused || view.Seats[ 1 ].Kind != KindEmpty {
		t.Fatalf( "after remove: status %s seat %+v" , view.Status , view.Seats[ 1 ] )
	}
	// A bot too, mid-game: the host frees its seat, and the table waits.
	if err = hub.RemovePlayer( id , host , false , 3 ); err != nil { t.Fatalf( "remove bot: %v" , err ) }
	view , _ = hub.View( id , Viewer{ User: host } , ViewOptions{} )
	if view.Seats[ 3 ].Kind != KindEmpty || view.Status != StatusPaused { t.Fatalf( "after removing bot: %s %+v" , view.Status , view.Seats[ 3 ] ) }
}

func TestRenameSeat( t *testing.T ) {
	hub , store , _ := testHub( t )
	host := testUser( t , store , "Ana" )
	guest := testUser( t , store , "Ben" )

	id , err := hub.Create( host , "Rename test" , game.DefaultOptions() )
	if err != nil { t.Fatal( err ) }
	if err = hub.Sit( id , guest , 1 ); err != nil { t.Fatal( err ) }
	if err = hub.AddBot( id , host , 2 ); err != nil { t.Fatal( err ) }

	if err = hub.RenameSeat( id , guest , false , 0 , "Boss" ); errors.Is( err , ErrNotHost ) == false { t.Fatalf( "non-host rename: %v" , err ) }
	if err = hub.RenameSeat( id , host , false , 3 , "Nobody" ); errors.Is( err , ErrNotAPlayer ) == false { t.Fatalf( "rename empty: %v" , err ) }
	if err = hub.RenameSeat( id , host , false , 1 , "  Benny  " ); err != nil { t.Fatal( err ) }
	if err = hub.RenameSeat( id , host , false , 2 , "Robo" ); err != nil { t.Fatal( err ) }
	if err = hub.Chat( id , guest , "hi" ); err != nil { t.Fatal( err ) }

	view , _ := hub.View( id , Viewer{ User: host } , ViewOptions{} )
	if view.Seats[ 1 ].Name != "Benny" || view.Seats[ 1 ].Renamed == false || view.Seats[ 2 ].Name != "Robo" {
		t.Fatalf( "after rename: %+v" , view.Seats )
	}
	if view.Chat[ len( view.Chat )-1 ].Name != "Benny" { t.Fatalf( "chat name: %+v" , view.Chat ) }

	// An empty name gives them their own back; so does leaving the seat.
	if err = hub.RenameSeat( id , host , false , 2 , "" ); err != nil { t.Fatal( err ) }
	if err = hub.Leave( id , guest ); err != nil { t.Fatal( err ) }
	if err = hub.Sit( id , guest , 1 ); err != nil { t.Fatal( err ) }
	view , _ = hub.View( id , Viewer{ User: host } , ViewOptions{} )
	if view.Seats[ 1 ].Name != "Ben" || view.Seats[ 2 ].Renamed {
		t.Fatalf( "after reset: %+v" , view.Seats )
	}
}
