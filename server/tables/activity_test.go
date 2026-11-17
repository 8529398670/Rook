package tables

import (
	testing "testing"
	time "time"
)

func TestSomeoneOnlineGoesIdleAfterNoActivity( t *testing.T ) {
	hub , store , _ := testHub( t )
	ana := testUser( t , store , "Ana" )
	if presence := hub.PresenceOf( ana.ID ); presence.Online || presence.Idle {
		t.Fatalf( "offline person: %+v" , presence )
	}

	subscriber := hub.SubscribeGlobal( ana.ID )
	defer hub.UnsubscribeGlobal( subscriber )
	if presence := hub.PresenceOf( ana.ID ); presence.Online == false || presence.Idle {
		t.Fatalf( "just opened the app: %+v" , presence )
	}

	// Last active ten minutes ago.
	hub.global_mu.Lock()
	hub.active_at[ ana.ID ] = time.Now().Add( -10 * time.Minute )
	hub.global_mu.Unlock()
	presence := hub.PresenceOf( ana.ID )
	if presence.Idle == false || presence.IdleSeconds < 599 || presence.IdleSince.IsZero() {
		t.Fatalf( "ten minutes without activity: %+v" , presence )
	}

	hub.Touch( ana.ID )
	if presence := hub.PresenceOf( ana.ID ); presence.Idle {
		t.Fatalf( "after activity: %+v" , presence )
	}
}
