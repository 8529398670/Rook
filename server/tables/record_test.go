package tables

import (
	testing "testing"
)

func TestRecordGroupsHandsAndTricks( t *testing.T ) {
	hub , store , _ := testHub( t )
	person := testUser( t , store , "Ana" )
	id := soloTable( t , hub , person )
	// Well into the first hand: bidding done and a few tricks played.
	playUntilMoves( t , hub , id , person , 30 )
	hub.Undo( id ) // hold the table so the bots stop under the test

	record , err := hub.Record( id )
	if err != nil { t.Fatal( err ) }
	if len( record.Hands ) != 1 {
		t.Fatalf( "want one hand, got %d" , len( record.Hands ) )
	}
	hand := record.Hands[ 0 ]
	if hand.Hand != 1 || len( hand.Bids ) == 0 || hand.Bidder < 0 || hand.Trump == "" {
		t.Fatalf( "hand header: %+v" , hand )
	}
	if len( hand.Tricks ) == 0 {
		t.Fatalf( "tricks should be recorded" )
	}
	for index , trick := range hand.Tricks {
		if trick.Number != index+1 {
			t.Fatalf( "trick %d numbered %d" , index+1 , trick.Number )
		}
		if trick.Done && ( len( trick.Plays ) != 4 || trick.Winner < 0 ) {
			t.Fatalf( "finished trick %d: %+v" , trick.Number , trick )
		}
		if trick.Done == false && index != len( hand.Tricks )-1 {
			t.Fatalf( "only the last trick may be unfinished" )
		}
	}

	// The admin history groups the same plays into the same tricks.
	history , _ := hub.History( id )
	finished := 0
	for _ , entry := range history.Entries {
		if entry.Kind == ActionPlay && entry.Trick == 0 {
			t.Fatalf( "play %d has no trick" , entry.Index )
		}
		if entry.Winner >= 0 { finished++ }
	}
	done := 0
	for _ , trick := range hand.Tricks {
		if trick.Done { done++ }
	}
	if finished < done {
		t.Fatalf( "history marks %d finished tricks, record has %d" , finished , done )
	}
}
