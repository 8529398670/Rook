package tables

import (
	errors "errors"
	strconv "strconv"
	testing "testing"
	time "time"

	game "rook/server/game"
	models "rook/server/models"
)

func TestGlobalChatKeepsTheLatestLinesAcrossARestart( t *testing.T ) {
	hub , store , cfg := testHub( t )
	ana := testUser( t , store , "Ana" )
	for index := 0 ; index < maxGlobalLines+5 ; index++ {
		if err := hub.GlobalChat( ana , "line " + strconv.Itoa( index ) ); err != nil { t.Fatal( err ) }
	}
	if err := hub.GlobalChat( ana , "   " ); !errors.Is( err , ErrBadChat ) {
		t.Fatalf( "blank line allowed: %v" , err )
	}
	view := hub.Global()
	if len( view.Chat ) != viewGlobalLines || view.Chat[ len( view.Chat )-1 ].Text != "line 104" || view.Enabled == false {
		t.Fatalf( "unexpected view: %d lines, enabled %v" , len( view.Chat ) , view.Enabled )
	}

	hub.Close()
	reloaded , err := New( store , cfg )
	if err != nil { t.Fatal( err ) }
	t.Cleanup( reloaded.Close )
	if len( reloaded.global_chat ) != maxGlobalLines || reloaded.global_chat[ 0 ].Text != "line 5" {
		t.Fatalf( "history after restart: %d lines, first %q" , len( reloaded.global_chat ) , reloaded.global_chat[ 0 ].Text )
	}
	if err := reloaded.GlobalChat( ana , "after" ); err != nil { t.Fatal( err ) }
	if last := reloaded.global_chat[ len( reloaded.global_chat )-1 ]; last.Seq != 106 {
		t.Fatalf( "sequence restarted: %d" , last.Seq )
	}
}

func TestChatSwitches( t *testing.T ) {
	hub , store , _ := testHub( t )
	ana := testUser( t , store , "Ana" )
	id , _ := hub.Create( ana , "Cards" , game.DefaultOptions() )

	if err := hub.SetSettings( models.AppSettings{ GlobalChatOff: true , TableChatOff: true } ); err != nil { t.Fatal( err ) }
	if err := hub.GlobalChat( ana , "hi" ); !errors.Is( err , ErrChatOff ) {
		t.Fatalf( "global chat while off: %v" , err )
	}
	if err := hub.Chat( id , ana , "hi" ); !errors.Is( err , ErrChatOff ) {
		t.Fatalf( "table chat while off: %v" , err )
	}
	if err := hub.GuestChat( id , "Cleo" , "hi" ); !errors.Is( err , ErrChatOff ) {
		t.Fatalf( "guest chat while off: %v" , err )
	}
	if view , _ := hub.View( id , Viewer{ User: ana } , ViewOptions{} ); view.ChatOff == false {
		t.Fatalf( "table view does not say chat is off" )
	}
	if hub.Global().Enabled {
		t.Fatalf( "global view does not say chat is off" )
	}
}

func TestOnlineList( t *testing.T ) {
	defer func( grace time.Duration ) { onlineGrace = grace }( onlineGrace )
	onlineGrace = 150 * time.Millisecond
	hub , store , _ := testHub( t )
	ana := testUser( t , store , "Ana" )
	ben := testUser( t , store , "Ben" )
	online := func() ( names []string ) {
		for _ , person := range hub.Global().Online { names = append( names , person.DisplayName ) }
		return
	}

	first := hub.SubscribeGlobal( ana.ID )
	second := hub.SubscribeGlobal( ana.ID )
	stream := hub.SubscribeGlobal( ben.ID )
	if names := online(); len( names ) != 2 || names[ 0 ] != "Ana" || names[ 1 ] != "Ben" {
		t.Fatalf( "online: %v" , names )
	}

	// One of two tabs closing leaves Ana online; Ben's reconnect does too.
	hub.UnsubscribeGlobal( first )
	hub.UnsubscribeGlobal( stream )
	if names := online(); len( names ) != 2 {
		t.Fatalf( "away the moment a stream closed: %v" , names )
	}
	time.Sleep( 300 * time.Millisecond )
	if names := online(); len( names ) != 1 || names[ 0 ] != "Ana" {
		t.Fatalf( "after Ben left: %v" , names )
	}

	// Disabled accounts drop off at once.
	models.SetUserDisabled( store , ana.ID , true )
	if names := online(); len( names ) != 0 {
		t.Fatalf( "disabled account still listed: %v" , names )
	}
	hub.UnsubscribeGlobal( second )
}
