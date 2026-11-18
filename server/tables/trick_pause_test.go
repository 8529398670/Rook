package tables

import (
	errors "errors"
	testing "testing"

	game "rook/server/game"
	models "rook/server/models"
)

func TestTrickPauseDefaultsAndPerTable( t *testing.T ) {
	hub , store , _ := testHub( t )
	ana := testUser( t , store , "Ana" )
	id := soloTable( t , hub , ana )

	pause := func() ( view TableView ) {
		view , err := hub.View( id , Viewer{ User: ana } , ViewOptions{} )
		if err != nil { t.Fatal( err ) }
		return
	}
	if view := pause(); view.TrickPause != models.DefaultTrickPauseMS || view.TrickPauseTable != 0 {
		t.Fatalf( "fresh table pause %d (own %d)" , view.TrickPause , view.TrickPauseTable )
	}

	// The app-wide default moves every table that has not chosen its own.
	settings := hub.Settings()
	settings.TrickPauseMS = 2500
	if err := hub.SetSettings( settings ); err != nil { t.Fatal( err ) }
	if view := pause(); view.TrickPause != 2500 || view.TrickPauseDefault != 2500 {
		t.Fatalf( "default not followed: %d / %d" , view.TrickPause , view.TrickPauseDefault )
	}

	// A table's own choice wins, and 0 hands it back to the default.
	if err := hub.SetTrickPause( id , ana , 4000 ); err != nil { t.Fatal( err ) }
	if view := pause(); view.TrickPause != 4000 || view.TrickPauseTable != 4000 {
		t.Fatalf( "table pause not used: %d" , view.TrickPause )
	}
	if err := hub.SetTrickPause( id , ana , 0 ); err != nil { t.Fatal( err ) }
	if view := pause(); view.TrickPause != 2500 {
		t.Fatalf( "reset to default gave %d" , view.TrickPause )
	}

	// It is the owner's setting: someone else may not change it, even seated
	// at the table; an admin (nil) may.
	bo := testUser( t , store , "Bo" )
	if err := hub.SetTrickPause( id , bo , 2000 ); !errors.Is( err , ErrNotHost ) {
		t.Fatalf( "a stranger changed the pause: %v" , err )
	}
	open , err := hub.Create( ana , "Open table" , game.DefaultOptions() )
	if err != nil { t.Fatal( err ) }
	if err = hub.Sit( open , bo , 1 ); err != nil { t.Fatal( err ) }
	if err = hub.SetTrickPause( open , bo , 2000 ); !errors.Is( err , ErrNotHost ) {
		t.Fatalf( "a seated non-owner changed the pause: %v" , err )
	}
	if err = hub.SetTrickPause( open , ana , 3000 ); err != nil { t.Fatal( err ) }
	if err := hub.SetTrickPause( id , nil , 2000 ); err != nil { t.Fatal( err ) }

	if err := hub.SetTrickPause( id , ana , 123 ); !errors.Is( err , ErrBadTrickPause ) {
		t.Fatalf( "odd pause accepted: %v" , err )
	}
}
