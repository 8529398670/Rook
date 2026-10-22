package tables

import (
	errors "errors"
	slices "slices"
	testing "testing"
	time "time"

	game "rook/server/game"
	models "rook/server/models"
)

func TestReshuffleIsAMoveUndoCanStepOver( t *testing.T ) {
	hub , store , _ := testHub( t )
	ana := testUser( t , store , "Ana" )
	ben := testUser( t , store , "Ben" )
	id , err := hub.Create( ana , "Two" , game.DefaultOptions() )
	if err != nil { t.Fatal( err ) }
	if err = hub.Sit( id , ben , 1 ); err != nil { t.Fatal( err ) }
	for seat := 2; seat < 4; seat++ {
		if err = hub.AddBot( id , ana , seat , "" ); err != nil { t.Fatal( err ) }
	}
	// Ana deals, so Ben (seat 1) opens the bidding.
	if err = hub.Start( id , ana , DealerHost ); err != nil { t.Fatal( err ) }
	if err = hub.Bid( id , ben , 70 ); err != nil { t.Fatal( err ) }
	deadline := time.Now().Add( 5 * time.Second )
	for {
		view , _ := hub.View( id , Viewer{ User: ana } , ViewOptions{} )
		if view.Game.Turn == 0 && view.Game.Phase == game.PhaseBidding { break }
		if time.Now().After( deadline ) { t.Fatalf( "the bots never passed the bidding to Ana" ) }
		time.Sleep( 2 * time.Millisecond )
	}
	before := gameJSON( t , hub , id )
	view , _ := hub.View( id , Viewer{ User: ana } , ViewOptions{} )
	if view.CanReshuffle {
		t.Fatalf( "a player without can_reshuffle is offered it" )
	}
	ana.CanReshuffle = true
	view , _ = hub.View( id , Viewer{ User: ana } , ViewOptions{} )
	if view.CanReshuffle == false {
		t.Fatalf( "Ana has not bid yet and may reshuffle" )
	}
	hand := slices.Clone( view.Game.Hand )

	if err = hub.Reshuffle( id , ana ); err != nil { t.Fatal( err ) }
	view , _ = hub.View( id , Viewer{ User: ana } , ViewOptions{} )
	if view.Game.HandNumber != 1 || view.Game.Turn != 1 || len( view.Game.Auction ) != 0 || slices.Equal( view.Game.Hand , hand ) {
		t.Fatalf( "after the reshuffle: hand %d turn %d auction %+v" , view.Game.HandNumber , view.Game.Turn , view.Game.Auction )
	}
	record , err := hub.Record( id )
	if err != nil { t.Fatal( err ) }
	if len( record.Hands ) != 1 || len( record.Hands[ 0 ].Bids ) != 0 {
		t.Fatalf( "the record kept bids from before the reshuffle: %+v" , record.Hands )
	}
	after := gameJSON( t , hub , id )

	// Undo goes back to the old cards and their bids; redo deals the same
	// reshuffled hands again.
	if err = hub.Undo( id ); err != nil { t.Fatal( err ) }
	if gameJSON( t , hub , id ) != before {
		t.Fatalf( "undo did not restore the hand before the reshuffle" )
	}
	if err = hub.Redo( id ); err != nil { t.Fatal( err ) }
	if gameJSON( t , hub , id ) != after {
		t.Fatalf( "redo did not deal the reshuffled hands" )
	}
	if err = hub.Resume( id ); err != nil { t.Fatal( err ) }

	// Once Ben bids on the new cards he may not reshuffle them.
	if err = hub.Bid( id , ben , 70 ); err != nil { t.Fatal( err ) }
	if err = hub.Reshuffle( id , ben ); !errors.Is( err , game.ErrAlreadyBid ) {
		t.Fatalf( "reshuffle after bidding: %v" , err )
	}
}

func TestRandomBotLook( t *testing.T ) {
	hub , store , _ := testHub( t )
	ana := testUser( t , store , "Ana" )
	id , err := hub.Create( ana , "Looks" , game.DefaultOptions() )
	if err != nil { t.Fatal( err ) }
	look := BotLook{ Names: []string{ "Pip" , "Fig" } , Icons: models.BotIcons }
	if err = hub.AddBotLooking( id , ana , 1 , "" , look ); err != nil { t.Fatal( err ) }
	if err = hub.AddBotLooking( id , ana , 2 , "" , look ); err != nil { t.Fatal( err ) }
	if err = hub.AddBot( id , ana , 3 , "" ); err != nil { t.Fatal( err ) }
	view , _ := hub.View( id , Viewer{ User: ana } , ViewOptions{} )
	first , second , plain := view.Seats[ 1 ] , view.Seats[ 2 ] , view.Seats[ 3 ]
	if first.Name == second.Name || slices.Contains( look.Names , first.Name ) == false || slices.Contains( look.Names , second.Name ) == false {
		t.Fatalf( "names %q and %q" , first.Name , second.Name )
	}
	for _ , seat := range []SeatInfo{ first , second } {
		if slices.Contains( models.BotIcons , seat.Icon ) == false || seat.Color == "" || models.ValidColor( seat.Color ) == false {
			t.Fatalf( "look %q %q" , seat.Icon , seat.Color )
		}
	}
	if plain.Name != "" || plain.Icon != "" || plain.Color != "" {
		t.Fatalf( "a plain bot was dressed: %+v" , plain )
	}
}

func TestShuffleBotLook( t *testing.T ) {
	hub , store , _ := testHub( t )
	ana := testUser( t , store , "Ana" )
	ben := testUser( t , store , "Ben" )
	id , err := hub.Create( ana , "Shuffle" , game.DefaultOptions() )
	if err != nil { t.Fatal( err ) }
	if err = hub.AddBot( id , ana , 1 , "" ); err != nil { t.Fatal( err ) }
	if err = hub.RenameSeat( id , ana , false , 1 , "Boss" ); err != nil { t.Fatal( err ) }
	look := BotLook{ Names: []string{ "Pip" , "Fig" } , Icons: models.BotIcons }
	if err = hub.ShuffleBot( id , ben , false , 1 , look ); errors.Is( err , ErrNotHost ) == false { t.Fatalf( "non-host shuffle: %v" , err ) }
	if err = hub.ShuffleBot( id , ana , false , 0 , look ); errors.Is( err , ErrNotABot ) == false { t.Fatalf( "shuffle a person: %v" , err ) }
	before := ""
	for range 5 {
		if err = hub.ShuffleBot( id , ana , false , 1 , look ); err != nil { t.Fatal( err ) }
		view , _ := hub.View( id , Viewer{ User: ana } , ViewOptions{} )
		bot := view.Seats[ 1 ]
		if slices.Contains( look.Names , bot.Name ) == false || bot.Renamed || bot.Name == before {
			t.Fatalf( "name %q after %q (renamed %v)" , bot.Name , before , bot.Renamed )
		}
		if slices.Contains( models.BotIcons , bot.Icon ) == false || models.ValidColor( bot.Color ) == false {
			t.Fatalf( "look %q %q" , bot.Icon , bot.Color )
		}
		before = bot.Name
	}
}

func TestSetBotLook( t *testing.T ) {
	hub , store , _ := testHub( t )
	ana := testUser( t , store , "Ana" )
	ben := testUser( t , store , "Ben" )
	id , err := hub.Create( ana , "Look" , game.DefaultOptions() )
	if err != nil { t.Fatal( err ) }
	if err = hub.AddBot( id , ana , 1 , "" ); err != nil { t.Fatal( err ) }
	icon := models.BotIcons[ 0 ]
	if err = hub.SetBotLook( id , ben , false , 1 , icon , "teal" ); errors.Is( err , ErrNotHost ) == false { t.Fatalf( "non-host: %v" , err ) }
	if err = hub.SetBotLook( id , ana , false , 0 , icon , "teal" ); errors.Is( err , ErrNotABot ) == false { t.Fatalf( "a person: %v" , err ) }
	if err = hub.SetBotLook( id , ana , false , 1 , "nope" , "teal" ); errors.Is( err , ErrBadLook ) == false { t.Fatalf( "bad icon: %v" , err ) }
	if err = hub.SetBotLook( id , ana , false , 1 , icon , "h400" ); errors.Is( err , ErrBadLook ) == false { t.Fatalf( "bad color: %v" , err ) }
	if err = hub.SetBotLook( id , ana , false , 1 , icon , "h200" ); err != nil { t.Fatal( err ) }
	view , _ := hub.View( id , Viewer{ User: ana } , ViewOptions{} )
	if view.Seats[ 1 ].Icon != icon || view.Seats[ 1 ].Color != "h200" { t.Fatalf( "look %+v" , view.Seats[ 1 ] ) }
	// Back to the robot, keeping a color.
	if err = hub.SetBotLook( id , ana , false , 1 , "" , "plum" ); err != nil { t.Fatal( err ) }
	view , _ = hub.View( id , Viewer{ User: ana } , ViewOptions{} )
	if view.Seats[ 1 ].Icon != "" || view.Seats[ 1 ].Color != "plum" { t.Fatalf( "look %+v" , view.Seats[ 1 ] ) }
}
