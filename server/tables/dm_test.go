package tables

import (
	errors "errors"
	testing "testing"
)

func TestDirectMessagesReachOnlyThePairAndSurviveARestart( t *testing.T ) {
	hub , store , cfg := testHub( t )
	ana := testUser( t , store , "Ana" )
	ben := testUser( t , store , "Ben" )
	cleo := testUser( t , store , "Cleo" )

	if err := hub.SendDM( ana , ana.ID , "me" ); !errors.Is( err , ErrMessageSelf ) {
		t.Fatalf( "message to self: %v" , err )
	}
	if err := hub.SendDM( ana , 999999 , "hi" ); !errors.Is( err , ErrNoSuchUser ) {
		t.Fatalf( "message to nobody: %v" , err )
	}
	if err := hub.SendDM( ana , ben.ID , "  " ); !errors.Is( err , ErrBadChat ) {
		t.Fatalf( "blank message: %v" , err )
	}
	if err := hub.SendDM( ana , ben.ID , "hi Ben" ); err != nil { t.Fatal( err ) }
	if err := hub.SendDM( ana , ben.ID , "you there?" ); err != nil { t.Fatal( err ) }

	mine := hub.GlobalFor( ana.ID ).DMs
	if len( mine ) != 1 || mine[ 0 ].UserID != ben.ID || mine[ 0 ].Unread != 0 || len( mine[ 0 ].Lines ) != 2 {
		t.Fatalf( "sender's view: %+v" , mine )
	}
	theirs := hub.GlobalFor( ben.ID ).DMs
	if len( theirs ) != 1 || theirs[ 0 ].UserID != ana.ID || theirs[ 0 ].Unread != 2 {
		t.Fatalf( "recipient's view: %+v" , theirs )
	}
	if len( hub.GlobalFor( cleo.ID ).DMs ) != 0 {
		t.Fatal( "a third person sees the conversation" )
	}

	if err := hub.ReadDM( ben , ana.ID , 1 ); err != nil { t.Fatal( err ) }
	if unread := hub.GlobalFor( ben.ID ).DMs[ 0 ].Unread; unread != 1 {
		t.Fatalf( "unread after reading one: %d" , unread )
	}
	if err := hub.EditDM( ben , ana.ID , 1 , "not mine" ); !errors.Is( err , ErrNotYourMessage ) {
		t.Fatalf( "edit someone else's line: %v" , err )
	}
	if err := hub.EditDM( ana , ben.ID , 1 , "hello Ben" ); err != nil { t.Fatal( err ) }
	if err := hub.DeleteDM( ana , ben.ID , 2 ); err != nil { t.Fatal( err ) }

	// Closing the tab hides it for that person only; the next message
	// brings it back.
	if err := hub.CloseDM( ben , ana.ID ); err != nil { t.Fatal( err ) }
	if len( hub.GlobalFor( ben.ID ).DMs ) != 0 || len( hub.GlobalFor( ana.ID ).DMs ) != 1 {
		t.Fatal( "closing a tab" )
	}
	if err := hub.SendDM( ana , ben.ID , "back again" ); err != nil { t.Fatal( err ) }
	if back := hub.GlobalFor( ben.ID ).DMs; len( back ) != 1 || back[ 0 ].Unread != 1 {
		t.Fatalf( "reopened by a message: %+v" , back )
	}
	if err := hub.OpenDM( cleo , ana.ID ); err != nil { t.Fatal( err ) }
	if opened := hub.GlobalFor( cleo.ID ).DMs; len( opened ) != 1 || len( opened[ 0 ].Lines ) != 0 {
		t.Fatalf( "an opened, empty conversation: %+v" , opened )
	}

	hub.Close()
	reloaded , err := New( store , cfg )
	if err != nil { t.Fatal( err ) }
	t.Cleanup( reloaded.Close )
	after := reloaded.GlobalFor( ben.ID ).DMs
	if len( after ) != 1 || len( after[ 0 ].Lines ) != 2 || after[ 0 ].Lines[ 0 ].Text != "hello Ben" || after[ 0 ].Lines[ 0 ].Edited == false || after[ 0 ].Unread != 1 {
		t.Fatalf( "after a restart: %+v" , after )
	}

	reloaded.ForgetUser( ana.ID )
	if len( reloaded.GlobalFor( ben.ID ).DMs ) != 0 {
		t.Fatal( "a deleted account's conversations stay" )
	}
}

func TestInviteHoldsTheRightSeatAndNotifies( t *testing.T ) {
	hub , store , _ := testHub( t )
	ana := testUser( t , store , "Ana" )
	ben := testUser( t , store , "Ben" )
	cleo := testUser( t , store , "Cleo" )

	// No table: a new one is opened, with Ben across from Ana.
	id , err := hub.Invite( ana , false , ben.ID , InviteAsPartner , "" , "Ana & Ben" )
	if err != nil { t.Fatal( err ) }
	table := hub.table( id )
	if table.Seats[ 0 ].UserID != ana.ID || table.Seats[ 2 ].UserID != ben.ID || table.Seats[ 2 ].Pending == false {
		t.Fatalf( "seats: %+v" , table.Seats )
	}
	invites := hub.GlobalFor( ben.ID ).Invites
	if len( invites ) != 1 || invites[ 0 ].FromName != "Ana" || invites[ 0 ].As != InviteAsPartner || invites[ 0 ].TableID != id {
		t.Fatalf( "notice: %+v" , invites )
	}
	if _ , err = hub.Invite( ana , false , ben.ID , InviteAsOpponent , id , "" ); !errors.Is( err , ErrAlreadySeated ) {
		t.Fatalf( "inviting twice: %v" , err )
	}

	// A bot in the opponent's chair gives it up.
	if err = hub.AddBot( id , ana , 1 , "" ); err != nil { t.Fatal( err ) }
	if _ , err = hub.Invite( ana , false , cleo.ID , InviteAsOpponent , id , "" ); err != nil { t.Fatal( err ) }
	if hub.table( id ).Seats[ 3 ].UserID != cleo.ID && hub.table( id ).Seats[ 1 ].UserID != cleo.ID {
		t.Fatalf( "opponent seat: %+v" , hub.table( id ).Seats )
	}
	if as := hub.GlobalFor( cleo.ID ).Invites[ 0 ].As; as != InviteAsOpponent {
		t.Fatalf( "invited as %q" , as )
	}

	// Ben is not the host, and may not invite: he may not hand out seats
	// at Ana's table.
	dee := testUser( t , store , "Dee" )
	guest := *ben
	guest.CanInvite = false
	if _ , err = hub.Invite( &guest , false , dee.ID , InviteAsOpponent , id , "" ); !errors.Is( err , ErrCannotInvite ) {
		t.Fatalf( "a guest inviting: %v" , err )
	}

	// Declining frees the seat and the notice goes.
	if err = hub.Leave( id , ben ); err != nil { t.Fatal( err ) }
	if len( hub.GlobalFor( ben.ID ).Invites ) != 0 || hub.table( id ).Seats[ 2 ].Kind != KindEmpty {
		t.Fatal( "decline" )
	}

	// Arriving at the table answers the invitation too.
	if _ , err = hub.Invite( ana , false , ben.ID , InviteAsPartner , id , "" ); err != nil { t.Fatal( err ) }
	subscriber , err := hub.SubscribeTable( id , ben.ID , false )
	if err != nil { t.Fatal( err ) }
	hub.UnsubscribeTable( id , subscriber )
	if len( hub.GlobalFor( ben.ID ).Invites ) != 0 {
		t.Fatal( "notice stays after arriving" )
	}
}
