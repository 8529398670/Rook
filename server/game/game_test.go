package game

import (
	errors "errors"
	rand "math/rand/v2"
	testing "testing"
)

func cards( t *testing.T , ids ...string ) ( result []Card ) {
	t.Helper()
	for _ , id := range ids {
		card , err := ParseCard( id )
		if err != nil { t.Fatalf( "bad card id %q" , id ) }
		result = append( result , card )
	}
	return
}

func card( t *testing.T , id string ) ( result Card ) {
	t.Helper()
	result = cards( t , id )[ 0 ]
	return
}

func seeded( seed uint64 ) ( source *rand.Rand ) {
	source = rand.New( rand.NewPCG( seed , seed^0x9e3779b97f4a7c15 ) )
	return
}

// playingState is a hand already in trick play, with trump named and the
// given hands, so a single rule can be exercised without playing up to it.
func playingState( t *testing.T , options Options , trump Color , hands [4][]string ) ( state *State ) {
	t.Helper()
	state = &State{
		Options:    options,
		Phase:      PhasePlaying,
		HandNumber: 1,
		Bid:        100,
		HighBidder: 0,
		Bidder:     0,
		Trump:      trump,
		Turn:       0,
		Trick:      Trick{ Leader: 0 , Winner: -1 },
		Winner:     -1,
	}
	for seat := 0; seat < 4; seat++ {
		state.Hands[ seat ] = cards( t , hands[ seat ]... )
	}
	return
}

func mustPlay( t *testing.T , state *State , seat int , id string ) {
	t.Helper()
	if err := state.PlayCard( seat , card( t , id ) ); err != nil {
		t.Fatalf( "seat %d playing %s: %v" , seat , id , err )
	}
}

func ids( list []Card ) ( result map[ string ]bool ) {
	result = map[ string ]bool{}
	for _ , item := range list {
		result[ item.ID() ] = true
	}
	return
}

func TestDeckHas57CardsWorth180( t *testing.T ) {
	deck := NewDeck()
	if len( deck ) != 57 {
		t.Fatalf( "deck has %d cards" , len( deck ) )
	}
	seen := map[ string ]bool{}
	total := 0
	for _ , item := range deck {
		if seen[ item.ID() ] { t.Fatalf( "duplicate %s" , item.ID() ) }
		seen[ item.ID() ] = true
		total += item.Points()
		parsed , err := ParseCard( item.ID() )
		if err != nil || parsed != item {
			t.Fatalf( "round trip of %s gave %v %v" , item.ID() , parsed , err )
		}
	}
	if total != TotalPoints {
		t.Fatalf( "counters total %d, want %d" , total , TotalPoints )
	}
	for _ , bad := range []string{ "" , "R" , "R0" , "R15" , "R01" , "X5" , "ROOKS" , "r5" , "R100" } {
		if _ , err := ParseCard( bad ); err == nil {
			t.Errorf( "ParseCard(%q) should fail" , bad )
		}
	}
}

func TestDealGivesThirteenEachAndFiveToTheKitty( t *testing.T ) {
	state , err := NewGame( DefaultOptions() , 0 , seeded( 1 ) )
	if err != nil { t.Fatal( err ) }
	for seat := 0; seat < 4; seat++ {
		if len( state.Hands[ seat ] ) != 13 { t.Fatalf( "seat %d has %d cards" , seat , len( state.Hands[ seat ] ) ) }
	}
	if len( state.Kitty ) != 5 || state.CardsInPlay() != 57 {
		t.Fatalf( "kitty %d, cards in play %d" , len( state.Kitty ) , state.CardsInPlay() )
	}
	if state.Phase != PhaseBidding || state.Turn != 0 || state.Bid != OpeningBid {
		t.Fatalf( "unexpected opening state %+v" , state )
	}
}

func TestBiddingRules( t *testing.T ) {
	state , _ := NewGame( DefaultOptions() , 1 , seeded( 2 ) )

	if err := state.PlaceBid( 0 , 70 ); !errors.Is( err , ErrNotYourTurn ) {
		t.Fatalf( "out of turn bid: %v" , err )
	}
	if err := state.PlaceBid( 1 , 65 ); !errors.Is( err , ErrBidTooLow ) {
		t.Fatalf( "bid equal to opening: %v" , err )
	}
	if err := state.PlaceBid( 1 , 72 ); !errors.Is( err , ErrBidStep ) {
		t.Fatalf( "bid not multiple of five: %v" , err )
	}
	if err := state.PlaceBid( 1 , 185 ); !errors.Is( err , ErrBidTooHigh ) {
		t.Fatalf( "bid over 180: %v" , err )
	}
	if err := state.PlaceBid( 1 , 70 ); err != nil { t.Fatal( err ) }
	if err := state.Pass( 2 ); err != nil { t.Fatal( err ) }
	if err := state.PlaceBid( 3 , 80 ); err != nil { t.Fatal( err ) }
	if err := state.PlaceBid( 0 , 85 ); err != nil { t.Fatal( err ) }
	// Seat 2 passed, so after seat 1 comes seat 3.
	if err := state.Pass( 1 ); err != nil { t.Fatal( err ) }
	if state.Turn != 3 { t.Fatalf( "passed seat not skipped; turn is %d" , state.Turn ) }
	if err := state.Pass( 3 ); err != nil { t.Fatal( err ) }

	if state.Phase != PhaseKitty || state.Bidder != 0 || state.Bid != 85 {
		t.Fatalf( "seat 0 should hold the bid at 85: phase %s bidder %d bid %d" , state.Phase , state.Bidder , state.Bid )
	}
	if len( state.Hands[ 0 ] ) != 18 || len( state.Kitty ) != 0 || len( state.PickedUp ) != 5 {
		t.Fatalf( "kitty not picked up: hand %d kitty %d" , len( state.Hands[ 0 ] ) , len( state.Kitty ) )
	}
}

// The dealer sits just before the first bidder, so they bid last; every view
// carries the auction so far, in order, until the bidder starts playing.
func TestDealerBidsLastAndTheAuctionIsShown( t *testing.T ) {
	state , _ := NewGame( DefaultOptions() , 1 , seeded( 2 ) )
	if state.Dealer() != 0 || state.Turn != 1 {
		t.Fatalf( "dealer %d turn %d, want dealer 0 and seat 1 to open" , state.Dealer() , state.Turn )
	}
	if view := state.ViewFor( 2 ); view.Dealer != 0 || len( view.Auction ) != 0 {
		t.Fatalf( "before any bid: dealer %d auction %+v" , view.Dealer , view.Auction )
	}
	state.PlaceBid( 1 , 70 )
	state.Pass( 2 )
	state.PlaceBid( 3 , 75 )
	state.Pass( 0 )
	state.Pass( 1 )
	want := []AuctionEntry{ { 1 , 70 } , { 2 , 0 } , { 3 , 75 } , { 0 , 0 } , { 1 , 0 } }
	got := state.ViewFor( -1 ).Auction
	if state.Phase != PhaseKitty || len( got ) != len( want ) {
		t.Fatalf( "phase %s auction %+v" , state.Phase , got )
	}
	for index := range want {
		if got[ index ] != want[ index ] { t.Fatalf( "auction %+v, want %+v" , got , want ) }
	}
	hand := state.Hands[ 3 ]
	if err := state.Bury( 3 , hand[ :5 ] , Red ); err != nil { t.Fatal( err ) }
	if auction := state.ViewFor( 3 ).Auction; len( auction ) != 0 {
		t.Fatalf( "auction still shown during play: %+v" , auction )
	}
}

// Play to is one of the offered targets or any number someone types, from a
// single hand (0) up to MaxPlayTo.
func TestPlayToTakesACustomTarget( t *testing.T ) {
	options := DefaultOptions()
	for _ , good := range []int{ 0 , 100 , 250 , 1000 , MaxPlayTo } {
		options.PlayTo = good
		if err := options.Validate(); err != nil { t.Fatalf( "play to %d refused: %v" , good , err ) }
	}
	for _ , bad := range []int{ -100 , MaxPlayTo + 1 } {
		options.PlayTo = bad
		if err := options.Validate(); !errors.Is( err , ErrBadOptions ) { t.Fatalf( "play to %d allowed: %v" , bad , err ) }
	}
}

func TestLastPlayerIsStuckAtTheOpeningBid( t *testing.T ) {
	state , _ := NewGame( DefaultOptions() , 2 , seeded( 3 ) )
	for _ , seat := range []int{ 2 , 3 , 0 } {
		if err := state.Pass( seat ); err != nil { t.Fatal( err ) }
	}
	if state.Phase != PhaseKitty || state.Bidder != 1 || state.Bid != OpeningBid {
		t.Fatalf( "seat 1 should be stuck at 65: bidder %d bid %d" , state.Bidder , state.Bid )
	}
}

func TestBiddingEverythingEndsTheAuction( t *testing.T ) {
	state , _ := NewGame( DefaultOptions() , 0 , seeded( 4 ) )
	if err := state.PlaceBid( 0 , 180 ); err != nil { t.Fatal( err ) }
	if state.Phase != PhaseKitty || state.Bidder != 0 {
		t.Fatalf( "180 should win outright, phase %s" , state.Phase )
	}
}

func TestBury( t *testing.T ) {
	state , _ := NewGame( DefaultOptions() , 0 , seeded( 5 ) )
	state.PlaceBid( 0 , 180 )
	hand := state.Hands[ 0 ]

	if err := state.Bury( 1 , hand[ :5 ] , Red ); !errors.Is( err , ErrNotYourTurn ) {
		t.Fatalf( "non-bidder burying: %v" , err )
	}
	if err := state.Bury( 0 , hand[ :4 ] , Red ); !errors.Is( err , ErrMustBuryFive ) {
		t.Fatalf( "four cards: %v" , err )
	}
	if err := state.Bury( 0 , []Card{ hand[ 0 ] , hand[ 0 ] , hand[ 1 ] , hand[ 2 ] , hand[ 3 ] } , Red ); !errors.Is( err , ErrDuplicateCard ) {
		t.Fatalf( "duplicate: %v" , err )
	}
	if err := state.Bury( 0 , hand[ :5 ] , RookBird ); !errors.Is( err , ErrBadTrump ) {
		t.Fatalf( "rook as trump: %v" , err )
	}
	foreign := state.Hands[ 1 ][ 0 ]
	if err := state.Bury( 0 , []Card{ foreign , hand[ 1 ] , hand[ 2 ] , hand[ 3 ] , hand[ 4 ] } , Red ); !errors.Is( err , ErrNotInHand ) {
		t.Fatalf( "someone else's card: %v" , err )
	}
	buried := append( []Card( nil ) , hand[ :5 ]... )
	if err := state.Bury( 0 , buried , Green ); err != nil { t.Fatal( err ) }
	if len( state.Hands[ 0 ] ) != 13 || len( state.Kitty ) != 5 || state.Trump != Green || state.Phase != PhasePlaying || state.Turn != 0 {
		t.Fatalf( "bad state after burying: hand %d trump %v phase %s" , len( state.Hands[ 0 ] ) , state.Trump , state.Phase )
	}
	if state.CardsInPlay() != 57 { t.Fatalf( "cards in play %d" , state.CardsInPlay() ) }
}

func TestLegalCards( t *testing.T ) {
	type scenario struct {
		name    string
		options func( *Options )
		trump   Color
		led     string // played by seat 3 before seat 0 acts; "" = seat 0 leads
		first   bool   // whether this is the hand's first trick
		hand    []string
		want    []string
	}
	scenarios := []scenario{
		{ name: "must follow color" , trump: Green , led: "R5" , hand: []string{ "R7" , "R1" , "B3" , "G9" , "ROOK" } , want: []string{ "R7" , "R1" } },
		{ name: "void may play anything" , trump: Green , led: "Y5" , hand: []string{ "R7" , "G9" , "ROOK" } , want: []string{ "R7" , "G9" , "ROOK" } },
		{ name: "rook follows trump" , trump: Green , led: "G5" , hand: []string{ "R7" , "G9" , "ROOK" } , want: []string{ "G9" , "ROOK" } },
		{ name: "rook alone must follow trump" , trump: Green , led: "G5" , hand: []string{ "R7" , "ROOK" } , want: []string{ "ROOK" } },
		{ name: "rook led is trump led" , trump: Green , led: "ROOK" , hand: []string{ "R7" , "G9" } , want: []string{ "G9" } },
		{ name: "rook does not follow other colors" , trump: Green , led: "R5" , hand: []string{ "R7" , "ROOK" } , want: []string{ "R7" } },
		{ name: "rook anytime" , options: func( o *Options ) { o.RookAnytime = true } , trump: Green , led: "R5" , hand: []string{ "R7" , "ROOK" , "B2" } , want: []string{ "R7" , "ROOK" } },
		{ name: "no rook on first trick when following" , options: func( o *Options ) { o.RookFirstTrick = false } , trump: Green , led: "G5" , first: true , hand: []string{ "R7" , "ROOK" } , want: []string{ "R7" } },
		{ name: "no rook on first trick when leading" , options: func( o *Options ) { o.RookFirstTrick = false } , trump: Green , first: true , hand: []string{ "R7" , "ROOK" } , want: []string{ "R7" } },
		{ name: "rook allowed after first trick" , options: func( o *Options ) { o.RookFirstTrick = false } , trump: Green , led: "G5" , hand: []string{ "R7" , "ROOK" } , want: []string{ "ROOK" } },
		{ name: "rook anytime yields to first-trick ban" , options: func( o *Options ) { o.RookFirstTrick = false; o.RookAnytime = true } , trump: Green , led: "R5" , first: true , hand: []string{ "R7" , "ROOK" } , want: []string{ "R7" } },
		{ name: "only the rook left" , options: func( o *Options ) { o.RookFirstTrick = false } , trump: Green , first: true , hand: []string{ "ROOK" } , want: []string{ "ROOK" } },
		{ name: "leading is free" , trump: Green , hand: []string{ "R7" , "ROOK" , "B1" } , want: []string{ "R7" , "ROOK" , "B1" } },
	}

	for _ , item := range scenarios {
		t.Run( item.name , func( t *testing.T ) {
			options := DefaultOptions()
			if item.options != nil { item.options( &options ) }
			state := playingState( t , options , item.trump , [4][]string{ item.hand , {} , {} , {} } )
			if item.first == false {
				state.Completed = []Trick{ { Leader: 0 , Winner: 0 } }
			}
			if item.led != "" {
				state.Trick.Leader = 3
				state.Trick.Plays = []Play{ { Seat: 3 , Card: card( t , item.led ) } }
			}
			got := ids( state.LegalCards( 0 ) )
			want := ids( cards( t , item.want... ) )
			if len( got ) != len( want ) {
				t.Fatalf( "got %v want %v" , got , want )
			}
			for id := range want {
				if got[ id ] == false { t.Fatalf( "got %v want %v" , got , want ) }
			}
		} )
	}
}

func TestIllegalPlaysAreRefusedWithAReason( t *testing.T ) {
	options := DefaultOptions()
	options.RookFirstTrick = false
	state := playingState( t , options , Green , [4][]string{ { "R7" , "ROOK" } , { "R9" , "B2" } , {} , {} } )
	if err := state.PlayCard( 0 , card( t , "ROOK" ) ); !errors.Is( err , ErrRookFirstTrick ) {
		t.Fatalf( "rook on first trick: %v" , err )
	}
	if err := state.PlayCard( 1 , card( t , "R9" ) ); !errors.Is( err , ErrNotYourTurn ) {
		t.Fatalf( "out of turn: %v" , err )
	}
	if err := state.PlayCard( 0 , card( t , "Y3" ) ); !errors.Is( err , ErrNotInHand ) {
		t.Fatalf( "card not held: %v" , err )
	}
	mustPlay( t , state , 0 , "R7" )
	if err := state.PlayCard( 1 , card( t , "B2" ) ); !errors.Is( err , ErrMustFollowSuit ) {
		t.Fatalf( "not following: %v" , err )
	}
}

func trickWinner( t *testing.T , options Options , trump Color , leader int , plays ...string ) ( winner int ) {
	t.Helper()
	var hands [4][]string
	for index , id := range plays {
		seat := ( leader + index ) % 4
		hands[ seat ] = []string{ id , "Y2" } // a spare card so the hand is not over
	}
	state := playingState( t , options , trump , hands )
	state.Completed = []Trick{ { Leader: 0 , Winner: 0 } }
	state.Turn = leader
	state.Trick.Leader = leader
	for index , id := range plays {
		mustPlay( t , state , ( leader+index )%4 , id )
	}
	winner = state.Completed[ len( state.Completed )-1 ].Winner
	return
}

func TestTrickWinner( t *testing.T ) {
	high := DefaultOptions()
	low := DefaultOptions()
	low.RookRank = RookLow
	half := DefaultOptions()
	half.RookRank = RookTenHalf

	type scenario struct {
		name    string
		options Options
		plays   []string // seats 0..3 in order
		want    int
	}
	scenarios := []scenario{
		{ "highest of color led" , high , []string{ "R5" , "R9" , "R13" , "B1" } , 2 },
		{ "the 1 beats the 14" , high , []string{ "R14" , "R1" , "R13" , "R2" } , 1 },
		{ "any trump beats the color led" , high , []string{ "R1" , "G2" , "R14" , "R13" } , 1 },
		{ "highest trump wins" , high , []string{ "R1" , "G2" , "G11" , "R13" } , 2 },
		// The original version compared instead of assigning here, so a
		// trick led with the bird was scored as if no color had been led.
		{ "rook led is trump led" , high , []string{ "ROOK" , "G1" , "G14" , "R1" } , 0 },
		{ "low rook led loses to any trump" , low , []string{ "ROOK" , "G2" , "R1" , "B1" } , 1 },
		{ "low rook still beats non-trump" , low , []string{ "R1" , "ROOK" , "R14" , "B1" } , 1 },
		{ "ten-and-a-half beats the 10" , half , []string{ "G10" , "ROOK" , "R1" , "B1" } , 1 },
		{ "ten-and-a-half loses to the 11" , half , []string{ "G11" , "ROOK" , "R1" , "B1" } , 0 },
		{ "high rook beats the 1 of trump" , high , []string{ "G1" , "ROOK" , "R1" , "B1" } , 1 },
	}
	for _ , item := range scenarios {
		t.Run( item.name , func( t *testing.T ) {
			if got := trickWinner( t , item.options , Green , 0 , item.plays... ); got != item.want {
				t.Fatalf( "winner %d, want %d" , got , item.want )
			}
		} )
	}
}

// lastTrickState is twelve tricks in with the given running counters, one
// card left per seat, and a kitty worth kitty_points buried.
func lastTrickState( t *testing.T , options Options , bid int , bidder int , taken [2]int , last [4]string , kitty []string ) ( state *State ) {
	t.Helper()
	var hands [4][]string
	for seat := 0; seat < 4; seat++ {
		hands[ seat ] = []string{ last[ seat ] }
	}
	state = playingState( t , options , Green , hands )
	state.Bid = bid
	state.Bidder = bidder
	state.HighBidder = bidder
	state.Taken = taken
	state.Kitty = cards( t , kitty... )
	for index := 0; index < 12; index++ {
		state.Completed = append( state.Completed , Trick{ Leader: 0 , Winner: index % 4 } )
	}
	return
}

func TestKittyGoesToTheTeamTakingTheLastTrick( t *testing.T ) {
	// Seat 1 (team 1) takes the last trick with the 1 of trump and so the
	// kitty's 25 points. The original version always gave it to team 2.
	state := lastTrickState( t , DefaultOptions() , 100 , 0 , [2]int{ 90 , 50 } ,
		[4]string{ "R2" , "G1" , "R3" , "R4" } , []string{ "B10" , "B5" , "B10" , "B2" , "B3" } )
	for seat , id := range []string{ "R2" , "G1" , "R3" , "R4" } {
		mustPlay( t , state , seat , id )
	}
	result := state.History[ 0 ]
	if result.KittyTeam != 1 || result.KittyPoints != 25 {
		t.Fatalf( "kitty went to team %d for %d" , result.KittyTeam , result.KittyPoints )
	}
	if result.Taken != [2]int{ 90 , 90 } {
		t.Fatalf( "taken %v, want [90 90] (15 for the G1 trick, 25 kitty)" , result.Taken )
	}
}

func TestGoingSet( t *testing.T ) {
	// Team 0 bid 120 and takes only 100: it loses 120. Team 1 scores its own
	// 80 -- the original version credited team 1 with team 2's points here.
	state := lastTrickState( t , DefaultOptions() , 120 , 2 , [2]int{ 100 , 80 } ,
		[4]string{ "R2" , "R6" , "R3" , "R4" } , []string{ "B2" , "B3" , "B4" , "B6" , "B7" } )
	for seat , id := range []string{ "R2" , "R6" , "R3" , "R4" } {
		mustPlay( t , state , seat , id )
	}
	result := state.History[ 0 ]
	if result.Made || result.Scored != [2]int{ -120 , 80 } || state.Scores != [2]int{ -120 , 80 } {
		t.Fatalf( "made %v scored %v totals %v" , result.Made , result.Scored , state.Scores )
	}
	if state.Phase != PhaseHandOver {
		t.Fatalf( "phase %s, want hand_over" , state.Phase )
	}
}

func TestMakingTheBid( t *testing.T ) {
	state := lastTrickState( t , DefaultOptions() , 120 , 1 , [2]int{ 50 , 130 } ,
		[4]string{ "R2" , "R6" , "R3" , "R4" } , []string{ "B2" , "B3" , "B4" , "B6" , "B7" } )
	for seat , id := range []string{ "R2" , "R6" , "R3" , "R4" } {
		mustPlay( t , state , seat , id )
	}
	if result := state.History[ 0 ]; result.Made == false || result.Scored != [2]int{ 50 , 130 } {
		t.Fatalf( "made %v scored %v" , result.Made , result.Scored )
	}
}

func TestGameEnd( t *testing.T ) {
	type scenario struct {
		name       string
		play_to    int
		before     [2]int
		bidder     int
		taken      [2]int
		want_phase Phase
		want       int
	}
	scenarios := []scenario{
		{ "not there yet" , 500 , [2]int{ 300 , 300 } , 0 , [2]int{ 100 , 80 } , PhaseHandOver , -1 },
		{ "team 1 reaches it" , 300 , [2]int{ 100 , 250 } , 1 , [2]int{ 60 , 120 } , PhaseGameOver , 1 },
		// Both cross on the same hand: the higher total wins.
		{ "higher total wins" , 300 , [2]int{ 250 , 280 } , 0 , [2]int{ 100 , 80 } , PhaseGameOver , 1 },
		// Dead level: the team that took the bid wins.
		{ "tie goes to the bidders" , 300 , [2]int{ 210 , 220 } , 0 , [2]int{ 100 , 90 } , PhaseGameOver , 0 },
		{ "single hand" , 0 , [2]int{ 0 , 0 } , 0 , [2]int{ 70 , 110 } , PhaseGameOver , 1 },
	}
	for _ , item := range scenarios {
		t.Run( item.name , func( t *testing.T ) {
			options := DefaultOptions()
			options.PlayTo = item.play_to
			state := lastTrickState( t , options , 65 , item.bidder , item.taken ,
				[4]string{ "R2" , "R6" , "R3" , "R4" } , []string{ "B2" , "B3" , "B4" , "B6" , "B7" } )
			state.Scores = item.before
			for seat , id := range []string{ "R2" , "R6" , "R3" , "R4" } {
				mustPlay( t , state , seat , id )
			}
			if state.Phase != item.want_phase || state.Winner != item.want {
				t.Fatalf( "phase %s winner %d scores %v" , state.Phase , state.Winner , state.Scores )
			}
		} )
	}
}

func TestContinueDealsTheNextHandWhenEveryoneIsReady( t *testing.T ) {
	state := lastTrickState( t , DefaultOptions() , 65 , 0 , [2]int{ 100 , 80 } ,
		[4]string{ "R2" , "R6" , "R3" , "R4" } , []string{ "B2" , "B3" , "B4" , "B6" , "B7" } )
	for seat , id := range []string{ "R2" , "R6" , "R3" , "R4" } {
		mustPlay( t , state , seat , id )
	}
	for seat := 0; seat < 3; seat++ {
		dealt , err := state.Continue( seat , ShuffledDeck( seeded( 9 ) ) )
		if err != nil || dealt { t.Fatalf( "seat %d: dealt %v err %v" , seat , dealt , err ) }
	}
	if _ , err := state.Continue( 0 , nil ); !errors.Is( err , ErrAlreadyReady ) {
		t.Fatalf( "double continue: %v" , err )
	}
	dealt , err := state.Continue( 3 , ShuffledDeck( seeded( 9 ) ) )
	if err != nil || dealt == false { t.Fatalf( "dealt %v err %v" , dealt , err ) }
	if state.HandNumber != 2 || state.BidStarter != 1 || state.Turn != 1 || state.Phase != PhaseBidding {
		t.Fatalf( "hand %d starter %d turn %d phase %s" , state.HandNumber , state.BidStarter , state.Turn , state.Phase )
	}
	if state.Dealer() != 0 || len( state.ViewFor( 0 ).Auction ) != 0 {
		t.Fatalf( "next hand: dealer %d auction %+v" , state.Dealer() , state.ViewFor( 0 ).Auction )
	}
}

func TestViewHidesOtherHandsAndTheKitty( t *testing.T ) {
	state , _ := NewGame( DefaultOptions() , 0 , seeded( 11 ) )
	view := state.ViewFor( 2 )
	if len( view.Hand ) != 13 || view.Seats[ 0 ].Cards != 13 || len( view.PickedUp ) != 0 {
		t.Fatalf( "view %+v" , view )
	}
	spectator := state.ViewFor( -1 )
	if len( spectator.Hand ) != 0 || len( spectator.Legal ) != 0 {
		t.Fatalf( "spectator sees cards" )
	}
	state.PlaceBid( 0 , 180 )
	if bidder := state.ViewFor( 0 ); len( bidder.PickedUp ) != 5 || len( bidder.Hand ) != 18 {
		t.Fatalf( "bidder should see the kitty: %d picked up" , len( bidder.PickedUp ) )
	}
	if other := state.ViewFor( 1 ); len( other.PickedUp ) != 0 {
		t.Fatalf( "another seat saw the kitty" )
	}
}

// TestBotGamesKeepEveryInvariant plays thousands of complete games between
// bots across every option combination and every mix of levels, checking
// after every single action that no card was created or lost, that only
// legal moves were accepted, and that every hand's counters add up to 180.
// Expert and Master search, so only the first expert_games games seat them.
func TestBotGamesKeepEveryInvariant( t *testing.T ) {
	games , expert_games := 2000 , 12
	// The levels that do not search, for the games past expert_games.
	cheap := []Level{ LevelEasy , LevelNormal , LevelSkilled , LevelHard }
	if testing.Short() { games , expert_games = 200 , 4 }

	rook_ranks := []RookRank{ RookHigh , RookTenHalf , RookLow }
	play_to := []int{ 0 , 100 , 250 , 300 , 500 }
	bids_made := 0
	hands_played := 0
	bid_total := 0

	for game_index := 0; game_index < games; game_index++ {
		source := seeded( uint64( game_index ) + 100 )
		options := Options{
			RookRank:       rook_ranks[ game_index%3 ],
			RookAnytime:    game_index%2 == 0,
			RookFirstTrick: game_index%5 != 0,
			PlayTo:         play_to[ game_index%len( play_to ) ],
		}
		state , err := NewGame( options , game_index%4 , source )
		if err != nil { t.Fatal( err ) }
		var levels [4]Level
		for seat := range levels {
			if game_index < expert_games {
				levels[ seat ] = Levels[ ( game_index+seat )%len( Levels ) ]
			} else {
				levels[ seat ] = cheap[ ( game_index/3+seat )%len( cheap ) ]
			}
		}

		for steps := 0; state.Phase != PhaseGameOver; steps++ {
			if steps > 20000 { t.Fatalf( "game %d never ended" , game_index ) }
			if state.HandNumber > 60 { break } // two careful bot teams can trade sets for a long time

			if state.Phase == PhaseHandOver {
				for seat := 0; seat < 4; seat++ {
					if _ , err := state.Continue( seat , ShuffledDeck( source ) ); err != nil { t.Fatal( err ) }
				}
				continue
			}

			seat := state.Turn
			if state.Phase == PhasePlaying {
				choice := state.BotPlay( seat , levels[ seat ] )
				if containsCard( state.LegalCards( seat ) , choice ) == false {
					t.Fatalf( "game %d: bot chose illegal %s" , game_index , choice.ID() )
				}
			}
			moved , err := state.BotMove( seat , levels[ seat ] )
			if err != nil || moved == false {
				t.Fatalf( "game %d seat %d phase %s: moved %v err %v" , game_index , seat , state.Phase , moved , err )
			}
			if count := state.CardsInPlay(); count != 57 {
				t.Fatalf( "game %d: %d cards in play after %s" , game_index , count , state.Phase )
			}
			view := state.ViewFor( -1 )
			for team := 0; team < 2; team++ {
				if seats := view.Seats[ team ].Taken + view.Seats[ team+2 ].Taken; seats != view.Taken[ team ] {
					t.Fatalf( "game %d team %d after %s: seats took %d, team took %d" , game_index , team , state.Phase , seats , view.Taken[ team ] )
				}
			}
			if state.Phase == PhaseHandOver || state.Phase == PhaseGameOver {
				result := state.History[ len( state.History )-1 ]
				if result.Taken[ 0 ]+result.Taken[ 1 ] != TotalPoints {
					t.Fatalf( "game %d: hand counters %v do not add to 180" , game_index , result.Taken )
				}
				if result.TricksWon[ 0 ]+result.TricksWon[ 1 ] != HandSize {
					t.Fatalf( "game %d: %v tricks" , game_index , result.TricksWon )
				}
				hands_played++
				bid_total += result.Bid
				if result.Made { bids_made++ }
			}
		}

		var totals [2]int
		for _ , result := range state.History {
			totals[ 0 ] += result.Scored[ 0 ]
			totals[ 1 ] += result.Scored[ 1 ]
		}
		if totals != state.Scores {
			t.Fatalf( "game %d: history sums to %v but scores are %v" , game_index , totals , state.Scores )
		}
	}
	t.Logf( "%d hands, average winning bid %d, bidders made it %d%% of the time" , hands_played , bid_total/hands_played , bids_made*100/hands_played )
}
