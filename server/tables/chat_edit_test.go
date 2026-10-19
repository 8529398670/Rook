package tables

import (
	errors "errors"
	testing "testing"

	game "rook/server/game"
)

func TestOnlyTheSenderEditsOrDeletesAChatLine( t *testing.T ) {
	hub , store , _ := testHub( t )
	ana := testUser( t , store , "Ana" )
	ben := testUser( t , store , "Ben" )
	id := soloTable( t , hub , ana )

	if err := hub.Chat( id , ana , "hello" ); err != nil { t.Fatal( err ) }
	if err := hub.Chat( id , ben , "watching" ); err != nil { t.Fatal( err ) }
	if err := hub.GuestChat( id , "Guest" , "hi all" ); err != nil { t.Fatal( err ) }
	view , _ := hub.View( id , Viewer{ User: ana } , ViewOptions{} )
	mine , bens , guest := view.Chat[ 0 ] , view.Chat[ 1 ] , view.Chat[ 2 ]
	if mine.UserID != ana.ID || bens.UserID != ben.ID || guest.UserID != 0 {
		t.Fatalf( "senders not recorded: %+v" , view.Chat )
	}

	if err := hub.EditChat( id , ana , false , bens.Seq , "nope" ); !errors.Is( err , ErrNotYourMessage ) {
		t.Fatalf( "edited someone else's line: %v" , err )
	}
	if err := hub.DeleteChat( id , ana , false , guest.Seq ); !errors.Is( err , ErrNotYourMessage ) {
		t.Fatalf( "deleted a guest's line: %v" , err )
	}
	if err := hub.EditChat( id , ana , false , 999 , "nope" ); !errors.Is( err , ErrNoSuchMessage ) {
		t.Fatalf( "edited a missing line: %v" , err )
	}
	if err := hub.EditChat( id , ana , false , mine.Seq , "   " ); !errors.Is( err , ErrBadChat ) {
		t.Fatalf( "blank edit: %v" , err )
	}

	if err := hub.EditChat( id , ana , false , mine.Seq , "hello there" ); err != nil { t.Fatal( err ) }
	view , _ = hub.View( id , Viewer{ User: ana } , ViewOptions{} )
	if view.Chat[ 0 ].Text != "hello there" || view.Chat[ 0 ].Edited == false {
		t.Fatalf( "edit not applied: %+v" , view.Chat[ 0 ] )
	}

	if err := hub.DeleteChat( id , ben , false , bens.Seq ); err != nil { t.Fatal( err ) }
	view , _ = hub.View( id , Viewer{ User: ana } , ViewOptions{} )
	if len( view.Chat ) != 2 || view.Chat[ 0 ].Seq != mine.Seq || view.Chat[ 1 ].Seq != guest.Seq {
		t.Fatalf( "delete took the wrong line: %+v" , view.Chat )
	}
}

func TestGlobalChatEditAndDelete( t *testing.T ) {
	hub , store , _ := testHub( t )
	ana := testUser( t , store , "Ana" )
	ben := testUser( t , store , "Ben" )

	if err := hub.GlobalChat( ana , "first" ); err != nil { t.Fatal( err ) }
	if err := hub.GlobalChat( ben , "second" ); err != nil { t.Fatal( err ) }
	chat := hub.Global().Chat
	first , second := chat[ 0 ] , chat[ 1 ]

	if err := hub.EditGlobalChat( ben , false , first.Seq , "mine now" ); !errors.Is( err , ErrNotYourMessage ) {
		t.Fatalf( "edited someone else's line: %v" , err )
	}
	if err := hub.DeleteGlobalChat( ana , false , second.Seq ); !errors.Is( err , ErrNotYourMessage ) {
		t.Fatalf( "deleted someone else's line: %v" , err )
	}
	if err := hub.EditGlobalChat( ana , false , first.Seq , "first!" ); err != nil { t.Fatal( err ) }
	if err := hub.DeleteGlobalChat( ben , false , second.Seq ); err != nil { t.Fatal( err ) }

	chat = hub.Global().Chat
	if len( chat ) != 1 || chat[ 0 ].Text != "first!" || chat[ 0 ].Edited == false {
		t.Fatalf( "global chat after edit and delete: %+v" , chat )
	}
}

func TestAModeratorEditsOrDeletesAnyChatLine( t *testing.T ) {
	hub , store , _ := testHub( t )
	ana := testUser( t , store , "Ana" )
	ben := testUser( t , store , "Ben" )
	id := soloTable( t , hub , ana )

	if err := hub.Chat( id , ana , "hello" ); err != nil { t.Fatal( err ) }
	if err := hub.GuestChat( id , "Guest" , "spam" ); err != nil { t.Fatal( err ) }
	view , _ := hub.View( id , Viewer{ User: ana } , ViewOptions{} )
	anas , guest := view.Chat[ 0 ] , view.Chat[ 1 ]

	if err := hub.EditChat( id , ben , true , anas.Seq , "hello, edited" ); err != nil { t.Fatal( err ) }
	if err := hub.DeleteChat( id , ben , true , guest.Seq ); err != nil { t.Fatal( err ) }
	view , _ = hub.View( id , Viewer{ User: ana } , ViewOptions{} )
	if len( view.Chat ) != 1 || view.Chat[ 0 ].Text != "hello, edited" || view.Chat[ 0 ].Edited == false {
		t.Fatalf( "table chat after moderation: %+v" , view.Chat )
	}

	if err := hub.GlobalChat( ana , "first" ); err != nil { t.Fatal( err ) }
	if err := hub.GlobalChat( ana , "second" ); err != nil { t.Fatal( err ) }
	chat := hub.Global().Chat
	if err := hub.EditGlobalChat( ben , true , chat[ 0 ].Seq , "first!" ); err != nil { t.Fatal( err ) }
	if err := hub.DeleteGlobalChat( ben , true , chat[ 1 ].Seq ); err != nil { t.Fatal( err ) }
	chat = hub.Global().Chat
	if len( chat ) != 1 || chat[ 0 ].Text != "first!" || chat[ 0 ].Edited == false {
		t.Fatalf( "global chat after moderation: %+v" , chat )
	}
}

func TestCanUndoIsForPlayersAtTheirOwnTable( t *testing.T ) {
	hub , store , _ := testHub( t )
	ana := testUser( t , store , "Ana" )
	ben := testUser( t , store , "Ben" )
	id := soloTable( t , hub , ana )

	rewind := func( viewer Viewer ) ( result bool ) {
		view , err := hub.View( id , viewer , ViewOptions{} )
		if err != nil { t.Fatal( err ) }
		result = view.CanRewind
		return
	}
	if rewind( Viewer{ User: ana } ) {
		t.Fatalf( "a plain player can rewind" )
	}
	ana.CanUndo , ben.CanUndo = true , true
	if rewind( Viewer{ User: ana } ) == false || hub.IsSeated( id , ana.ID ) == false {
		t.Fatalf( "a seated player with can_undo cannot rewind" )
	}
	// Watching someone else's table is not enough.
	if rewind( Viewer{ User: ben } ) || hub.IsSeated( id , ben.ID ) {
		t.Fatalf( "a watcher with can_undo can rewind" )
	}
	// Admins can, anywhere.
	if rewind( Viewer{ User: ben , Admin: true } ) == false {
		t.Fatalf( "an admin cannot rewind" )
	}

	view , _ := hub.View( id , Viewer{ User: ana } , ViewOptions{} )
	if view.Game == nil || view.Game.Phase == game.PhaseGameOver || view.Moves == 0 {
		t.Fatalf( "rewind view has no history: moves %d" , view.Moves )
	}
}
