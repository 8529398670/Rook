package tables

import (
	errors "errors"
	testing "testing"
	time "time"

	game "rook/server/game"
)

func TestTakeOverABotWithHostApproval( t *testing.T ) {
	hub , store , _ := testHub( t )
	host := testUser( t , store , "Ana" )
	guest := testUser( t , store , "Ben" )
	other := testUser( t , store , "Cy" )
	id := soloTable( t , hub , host )
	playUntilMoves( t , hub , id , host , 12 )

	if err := hub.RequestSeat( id , guest , 0 ); !errors.Is( err , ErrNotABot ) {
		t.Fatalf( "asking for a person's seat: %v" , err )
	}
	if err := hub.RequestSeat( id , host , 2 ); !errors.Is( err , ErrAlreadySeated ) {
		t.Fatalf( "a seated player asking: %v" , err )
	}
	if err := hub.RequestSeat( id , guest , 2 ); err != nil { t.Fatal( err ) }

	asking , _ := hub.View( id , Viewer{ User: guest } , ViewOptions{ Perspective: 2 } )
	if asking.MyRequest != 2 || len( asking.Requests ) != 0 || asking.You != -1 {
		t.Fatalf( "requester view: my_request %d requests %d you %d" , asking.MyRequest , len( asking.Requests ) , asking.You )
	}
	// Waiting to take over a bot, they must not see its cards -- or anyone's.
	for seat := 0; seat < 4; seat++ {
		peek , _ := hub.View( id , Viewer{ User: guest } , ViewOptions{ Perspective: seat , AllHands: true } )
		if peek.HandsHidden == false || len( peek.Game.Hand ) != 0 || peek.Game.Hands != nil {
			t.Fatalf( "requester saw seat %d's cards" , seat )
		}
	}
	hosting , _ := hub.View( id , Viewer{ User: host } , ViewOptions{} )
	if len( hosting.Requests ) != 1 || hosting.Requests[ 0 ].Seat != 2 || hosting.Requests[ 0 ].Name != "Ben" {
		t.Fatalf( "host should see the request, got %+v" , hosting.Requests )
	}
	if err := hub.AnswerRequest( id , other , false , guest.ID , true ); !errors.Is( err , ErrNotHost ) {
		t.Fatalf( "only the host answers: %v" , err )
	}

	if err := hub.AnswerRequest( id , host , false , guest.ID , true ); err != nil { t.Fatal( err ) }
	seated , _ := hub.View( id , Viewer{ User: guest } , ViewOptions{} )
	if seated.You != 2 || seated.MyRequest != -1 || seated.Seats[ 2 ].Kind != KindHuman || seated.Status != StatusPlaying {
		t.Fatalf( "after approval: you %d my_request %d kind %s status %s" , seated.You , seated.MyRequest , seated.Seats[ 2 ].Kind , seated.Status )
	}
	if len( seated.Game.Hand ) == 0 {
		t.Fatalf( "the new player should hold the bot's cards" )
	}

	// The bot is gone for good: when it is seat 2's turn, nobody moves it.
	waitFor( t , hub , id , host , func( view TableView ) bool {
		g := view.Game
		if g.Turn == 0 {
			switch g.Phase {
			case game.PhaseBidding:
				hub.Pass( id , host )
			case game.PhaseKitty:
				hub.Bury( id , host , g.Hand[ :5 ] , "red" )
			case game.PhasePlaying:
				hub.Play( id , host , g.Legal[ 0 ] )
			}
		}
		return g.Turn == 2 && ( g.Phase == game.PhaseBidding || g.Phase == game.PhaseKitty || g.Phase == game.PhasePlaying )
	} )
	before , _ := hub.View( id , Viewer{ User: guest } , ViewOptions{} )
	time.Sleep( 30 * time.Millisecond )
	after , _ := hub.View( id , Viewer{ User: guest } , ViewOptions{} )
	if after.Version != before.Version {
		t.Fatalf( "something moved for the person who took over" )
	}
}

func TestDeclineCancelAndPrune( t *testing.T ) {
	hub , store , _ := testHub( t )
	host := testUser( t , store , "Ana" )
	guest := testUser( t , store , "Ben" )
	id , _ := hub.Create( host , "Cards" , game.DefaultOptions() )
	for seat := 1; seat < 4; seat++ {
		hub.AddBot( id , host , seat , "" )
	}

	hub.RequestSeat( id , guest , 1 )
	if err := hub.AnswerRequest( id , host , false , guest.ID , false ); err != nil { t.Fatal( err ) }
	if view , _ := hub.View( id , Viewer{ User: guest } , ViewOptions{} ); view.MyRequest != -1 || view.You != -1 {
		t.Fatalf( "declined: my_request %d you %d" , view.MyRequest , view.You )
	}

	hub.RequestSeat( id , guest , 1 )
	if err := hub.CancelRequest( id , guest ); err != nil { t.Fatal( err ) }
	if err := hub.CancelRequest( id , guest ); !errors.Is( err , ErrNoRequest ) {
		t.Fatalf( "cancelling twice: %v" , err )
	}

	// Asking again moves the request rather than adding a second one.
	hub.RequestSeat( id , guest , 1 )
	hub.RequestSeat( id , guest , 3 )
	if view , _ := hub.View( id , Viewer{ User: host } , ViewOptions{} ); len( view.Requests ) != 1 || view.Requests[ 0 ].Seat != 3 {
		t.Fatalf( "re-asking: %+v" , view.Requests )
	}
	// Once the bot is removed there is nothing left to take over.
	hub.RemoveBot( id , host , 3 )
	if view , _ := hub.View( id , Viewer{ User: guest } , ViewOptions{} ); view.MyRequest != -1 {
		t.Fatalf( "a request for a removed bot should be dropped" )
	}

	// An admin may answer for the host.
	hub.RequestSeat( id , guest , 2 )
	if err := hub.AnswerRequest( id , guest , true , guest.ID , true ); err != nil { t.Fatal( err ) }
	if view , _ := hub.View( id , Viewer{ User: guest } , ViewOptions{} ); view.You != 2 {
		t.Fatalf( "admin approval should seat them, got %d" , view.You )
	}
}

func TestJoinLinkAsksForABotSeat( t *testing.T ) {
	hub , store , _ := testHub( t )
	host := testUser( t , store , "Ana" )
	late := testUser( t , store , "Cy" )
	id := soloTable( t , hub , host )
	view , _ := hub.View( id , Viewer{ User: host } , ViewOptions{} )

	if _ , err := hub.Join( view.JoinCode , late ); err != nil { t.Fatal( err ) }
	joined , _ := hub.View( id , Viewer{ User: late } , ViewOptions{} )
	if joined.You != -1 || joined.MyRequest != 1 {
		t.Fatalf( "a full table with bots should leave a request: you %d my_request %d" , joined.You , joined.MyRequest )
	}
	if _ , err := hub.Join( view.JoinCode , late ); err != nil { t.Fatal( err ) }
	if hosting , _ := hub.View( id , Viewer{ User: host } , ViewOptions{} ); len( hosting.Requests ) != 1 {
		t.Fatalf( "joining twice should not ask twice" )
	}
}
