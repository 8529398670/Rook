package tables

import (
	json "encoding/json"
	errors "errors"
	testing "testing"
	time "time"

	game "rook/server/game"
	models "rook/server/models"
)

// soloTable is one person against three bots, dealt and running.
func soloTable( t *testing.T , hub *Hub , person *models.User ) ( id string ) {
	t.Helper()
	id , err := hub.Create( person , "Solo" , game.DefaultOptions() )
	if err != nil { t.Fatal( err ) }
	for seat := 1; seat < 4; seat++ {
		if err := hub.AddBot( id , person , seat ); err != nil { t.Fatal( err ) }
	}
	if err := hub.Start( id , person , DealerHost ); err != nil { t.Fatal( err ) }
	return
}

func gameJSON( t *testing.T , hub *Hub , id string ) ( result string ) {
	t.Helper()
	table := hub.table( id )
	table.mu.Lock()
	defer table.mu.Unlock()
	raw , err := json.Marshal( table.Game )
	if err != nil { t.Fatal( err ) }
	result = string( raw )
	return
}

// playUntilMoves drives the person's seat (passing, burying, playing the
// first legal card) until the game is at least moves moves in.
func playUntilMoves( t *testing.T , hub *Hub , id string , person *models.User , moves int ) {
	t.Helper()
	deadline := time.Now().Add( 10 * time.Second )
	for {
		view , _ := hub.View( id , Viewer{ User: person , Admin: true } , ViewOptions{} )
		if view.Cursor >= moves { return }
		if time.Now().After( deadline ) { t.Fatalf( "stuck at %d moves" , view.Cursor ) }
		g := view.Game
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
		time.Sleep( 2 * time.Millisecond )
	}
}

func TestUndoRedoResetAndHold( t *testing.T ) {
	hub , store , _ := testHub( t )
	person := testUser( t , store , "Ana" )
	admin := Viewer{ User: person , Admin: true }
	id := soloTable( t , hub , person )
	playUntilMoves( t , hub , id , person , 20 )

	// Freeze the table where it is so bots stop moving under the test.
	if err := hub.Undo( id ); err != nil { t.Fatal( err ) }
	if err := hub.Redo( id ); err != nil { t.Fatal( err ) }
	view , _ := hub.View( id , admin , ViewOptions{} )
	if view.Held == false || view.Status != StatusHeld || view.CanRedo {
		t.Fatalf( "after undo+redo: held %v status %s can_redo %v" , view.Held , view.Status , view.CanRedo )
	}
	moves := view.Moves
	at_end := gameJSON( t , hub , id )

	// Replaying the whole log reproduces the game exactly.
	table := hub.table( id )
	table.mu.Lock()
	if err := table.rebuild( table.Cursor ); err != nil { t.Fatal( err ) }
	table.mu.Unlock()
	if gameJSON( t , hub , id ) != at_end {
		t.Fatalf( "replaying the log produced a different game" )
	}

	// Undo three times, then redo them: back to the identical position.
	for step := 0; step < 3; step++ {
		if err := hub.Undo( id ); err != nil { t.Fatal( err ) }
	}
	view , _ = hub.View( id , admin , ViewOptions{} )
	if view.Cursor != moves-3 || view.CanRedo == false {
		t.Fatalf( "cursor %d after three undos from %d" , view.Cursor , moves )
	}
	// Held: nobody may move, and no bot does.
	time.Sleep( 50 * time.Millisecond )
	if again , _ := hub.View( id , admin , ViewOptions{} ); again.Version != view.Version {
		t.Fatalf( "something moved while held" )
	}
	if err := hub.Pass( id , person ); !errors.Is( err , ErrHeld ) && !errors.Is( err , game.ErrNotYourTurn ) && !errors.Is( err , game.ErrWrongPhase ) {
		t.Fatalf( "move while held: %v" , err )
	}
	for step := 0; step < 3; step++ {
		if err := hub.Redo( id ); err != nil { t.Fatal( err ) }
	}
	if gameJSON( t , hub , id ) != at_end {
		t.Fatalf( "undo then redo did not return to the same position" )
	}
	if err := hub.Redo( id ); !errors.Is( err , ErrNothingToRedo ) {
		t.Fatalf( "redo past the end: %v" , err )
	}

	// Reset to the start of the game: no game at all, seats kept.
	if err := hub.ResetTo( id , 0 ); err != nil { t.Fatal( err ) }
	view , _ = hub.View( id , admin , ViewOptions{} )
	if view.Game != nil || view.Status != StatusWaiting || view.Seats[ 1 ].Kind != KindBot {
		t.Fatalf( "reset to 0: game %v status %s" , view.Game != nil , view.Status )
	}
	if err := hub.ResetTo( id , moves+1 ); !errors.Is( err , ErrBadCursor ) {
		t.Fatalf( "reset past the end: %v" , err )
	}

	// Back to move 5, resume, and play on: the old future is replaced.
	if err := hub.ResetTo( id , 5 ); err != nil { t.Fatal( err ) }
	if err := hub.Resume( id ); err != nil { t.Fatal( err ) }
	if err := hub.Resume( id ); !errors.Is( err , ErrNotHeld ) {
		t.Fatalf( "resume twice: %v" , err )
	}
	playUntilMoves( t , hub , id , person , 7 )
	view , _ = hub.View( id , admin , ViewOptions{} )
	if view.CanRedo || view.Moves != view.Cursor {
		t.Fatalf( "a new move after reset should drop the redo history: cursor %d moves %d" , view.Cursor , view.Moves )
	}

	// History survives a restart.
	history , _ := hub.History( id )
	if len( history.Entries ) < view.Moves || history.Entries[ 0 ].Kind != ActionStart { // bots may have moved since
		t.Fatalf( "history %d entries, first %v" , len( history.Entries ) , history.Entries[ 0 ] )
	}
}

func TestWatchersAndPerspectives( t *testing.T ) {
	hub , store , _ := testHub( t )
	person := testUser( t , store , "Ana" )
	watcher := testUser( t , store , "Wes" )
	id := soloTable( t , hub , person )

	// Anonymous watcher from seat 2: sees seat 2's hand, can play nothing,
	// and may not see every hand.
	view , err := hub.View( id , Viewer{} , ViewOptions{ Perspective: 2 , AllHands: true } )
	if err != nil { t.Fatal( err ) }
	if view.You != -1 || view.Perspective != 2 || len( view.Game.Hand ) != 13 || len( view.Game.Legal ) != 0 {
		t.Fatalf( "anonymous watcher: you %d perspective %d hand %d legal %d" , view.You , view.Perspective , len( view.Game.Hand ) , len( view.Game.Legal ) )
	}
	if view.CanSeeAll || view.Game.Hands != nil || view.JoinCode != "" || view.IsAdmin {
		t.Fatalf( "anonymous watcher got too much" )
	}

	// The table setting lets watchers see all hands.
	if err := hub.SetObserversSeeAll( id , true ); err != nil { t.Fatal( err ) }
	view , _ = hub.View( id , Viewer{} , ViewOptions{ Perspective: 1 , AllHands: true } )
	if view.CanSeeAll == false || len( view.Game.Hands ) != 4 || len( view.Game.Hands[ 3 ] ) == 0 {
		t.Fatalf( "table setting did not allow all hands" )
	}
	// ...except to someone signed in who could take over a bot: they see
	// no hidden cards at all, so they cannot look first and sit later.
	view , _ = hub.View( id , Viewer{ User: watcher } , ViewOptions{ Perspective: 1 , AllHands: true } )
	if view.HandsHidden == false || view.CanSeeAll || view.Game.Hands != nil || len( view.Game.Hand ) != 0 || len( view.Game.PickedUp ) != 0 {
		t.Fatalf( "a possible taker saw cards: hidden %v hands %v hand %d" , view.HandsHidden , view.Game.Hands != nil , len( view.Game.Hand ) )
	}
	// ...but never for someone playing.
	view , _ = hub.View( id , Viewer{ User: person , Admin: true } , ViewOptions{ Perspective: 1 , AllHands: true } )
	if view.Perspective != 0 || view.CanSeeAll || view.Game.Hands != nil {
		t.Fatalf( "a seated player saw other hands" )
	}

	// The app-wide setting does the same for every table.
	hub.SetObserversSeeAll( id , false )
	if err := hub.SetSettings( models.AppSettings{ ObserversSeeAll: true } ); err != nil { t.Fatal( err ) }
	if view , _ = hub.View( id , Viewer{} , ViewOptions{ AllHands: true } ); view.Game.Hands == nil {
		t.Fatalf( "app setting did not allow all hands" )
	}
	hub.SetSettings( models.AppSettings{} )

	// An admin watcher may always see everything, and is not counted.
	admin_stream , _ := hub.SubscribeTable( id , 999 , true )
	anon_stream , _ := hub.SubscribeTable( id , 0 , false )
	defer hub.UnsubscribeTable( id , admin_stream )
	defer hub.UnsubscribeTable( id , anon_stream )
	view , _ = hub.View( id , Viewer{ User: watcher , Admin: true } , ViewOptions{ AllHands: true } )
	if view.Game.Hands == nil || view.Watchers != 1 || view.JoinCode == "" {
		t.Fatalf( "admin watcher: hands %v watchers %d" , view.Game.Hands != nil , view.Watchers )
	}
}

func TestJoinLinks( t *testing.T ) {
	hub , store , _ := testHub( t )
	host := testUser( t , store , "Ana" )
	friend := testUser( t , store , "Ben" )
	late := testUser( t , store , "Cy" )
	id , _ := hub.Create( host , "Cards" , game.DefaultOptions() )
	view , _ := hub.View( id , Viewer{ User: host } , ViewOptions{} )

	if _ , err := hub.Join( "nope" , friend ); !errors.Is( err , ErrNoSuchLink ) {
		t.Fatalf( "bad code: %v" , err )
	}
	joined , err := hub.Join( view.JoinCode , friend )
	if err != nil || joined != id { t.Fatalf( "join: %v %s" , err , joined ) }
	if again , _ := hub.Join( view.JoinCode , friend ); again != id {
		t.Fatalf( "joining twice should just open the table" )
	}
	if seated , _ := hub.View( id , Viewer{ User: friend } , ViewOptions{} ); seated.You != 1 {
		t.Fatalf( "friend should be in the first free seat, got %d" , seated.You )
	}
	hub.AddBot( id , host , 2 )
	hub.AddBot( id , host , 3 )
	if _ , err := hub.Join( view.JoinCode , late ); err != nil { t.Fatal( err ) }
	if watching , _ := hub.View( id , Viewer{ User: late } , ViewOptions{} ); watching.You != -1 || watching.MyRequest != 2 {
		t.Fatalf( "a full table should leave a late joiner watching and asking for a bot's seat" )
	}
	if watch_id , err := hub.ByCode( view.WatchCode , true ); err != nil || watch_id != id {
		t.Fatalf( "watch code: %v" , err )
	}
	if _ , err := hub.ByCode( view.WatchCode , false ); err == nil {
		t.Fatalf( "a watch code must not work as a join code" )
	}
}

func TestHintIsALegalMove( t *testing.T ) {
	hub , store , _ := testHub( t )
	person := testUser( t , store , "Ana" )
	id := soloTable( t , hub , person )
	deadline := time.Now().Add( 10 * time.Second )
	plays := 0
	for plays < 5 && time.Now().Before( deadline ) {
		view , _ := hub.View( id , Viewer{ User: person } , ViewOptions{} )
		if view.Game.Turn != 0 {
			// A bot may make it our turn between the view and the hint, so
			// out of turn the only wrong answer is a different error.
			if _ , err := hub.Hint( id , person ); err != nil && !errors.Is( err , game.ErrNotYourTurn ) {
				t.Fatalf( "hint out of turn: %v" , err )
			}
			time.Sleep( 2 * time.Millisecond )
			continue
		}
		hint , err := hub.Hint( id , person )
		if err != nil { t.Fatal( err ) }
		// Taking the hint must always be accepted.
		switch hint.Kind {
		case ActionBid:
			err = hub.Bid( id , person , hint.Amount )
		case ActionPass:
			err = hub.Pass( id , person )
		case ActionBury:
			err = hub.Bury( id , person , hint.Cards , hint.Trump )
		case ActionPlay:
			err = hub.Play( id , person , hint.Card )
			plays++
		}
		if err != nil { t.Fatalf( "hint %+v was refused: %v" , hint , err ) }
	}
	if plays < 5 { t.Fatalf( "only %d hinted plays" , plays ) }
}

func TestRefreshUserUpdatesSeats( t *testing.T ) {
	hub , store , _ := testHub( t )
	person := testUser( t , store , "Ana" )
	id , _ := hub.Create( person , "Cards" , game.DefaultOptions() )
	if err := models.SetUserAvatar( store , person.ID , "🦉" , "plum" ); err != nil { t.Fatal( err ) }
	models.RenameUser( store , person.ID , "Anna" )
	updated , _ := models.GetUser( store , person.ID )
	hub.RefreshUser( updated )
	view , _ := hub.View( id , Viewer{ User: updated } , ViewOptions{} )
	if view.Seats[ 0 ].Name != "Anna" || view.Seats[ 0 ].Icon != "🦉" || view.Seats[ 0 ].Color != "plum" {
		t.Fatalf( "seat not refreshed: %+v" , view.Seats[ 0 ] )
	}
	if err := models.SetUserAvatar( store , person.ID , "<b>" , "" ); err == nil {
		t.Fatalf( "an icon outside the list was accepted" )
	}
}
