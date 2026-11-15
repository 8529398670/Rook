package tables

import (
	testing "testing"

	models "rook/server/models"
)

// TestPartnerBotOutbidsFollowsThePerson: a bot may keep raising its partner
// after the opponents pass only when that partner is a person who switched
// it on, and a change on their account reaches the seat at once.
func TestPartnerBotOutbidsFollowsThePerson( t *testing.T ) {
	hub , store , _ := testHub( t )
	person := testUser( t , store , "Ana" )
	id := soloTable( t , hub , person )

	outbids := func( seat int ) ( result bool ) {
		table := hub.table( id )
		table.mu.Lock()
		defer table.mu.Unlock()
		result = table.outbidsPartner( seat )
		return
	}
	if outbids( 2 ) { t.Fatalf( "on by default: the bot partner should stop bidding" ) }

	if err := models.SetUserPartnerBotOutbids( store , person.ID , true ); err != nil { t.Fatal( err ) }
	updated , err := models.GetUser( store , person.ID )
	if err != nil { t.Fatal( err ) }
	hub.RefreshUser( updated )
	if outbids( 2 ) == false { t.Fatalf( "switched off on the account, but the seat did not follow" ) }
	// Seats 1 and 3 partner each other, both bots: the rule always holds.
	if outbids( 1 ) || outbids( 3 ) { t.Fatalf( "a bot partnered with a bot may outbid it" ) }
	// Ana's own seat is a person, not a bot the server bids for.
	if outbids( 0 ) { t.Fatalf( "the person's own seat counted as a bot" ) }
}
