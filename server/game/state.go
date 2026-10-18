package game

import (
	rand "math/rand/v2"
)

// Phase is where a game is. A table that has not dealt yet has no State at
// all -- seating people is the tables package's job, not the rules'.
type Phase string

const (
	PhaseBidding  Phase = "bidding"
	PhaseKitty    Phase = "kitty"
	PhasePlaying  Phase = "playing"
	PhaseHandOver Phase = "hand_over"
	PhaseGameOver Phase = "game_over"
)

// Seats are numbered 0-3 clockwise. Partners sit opposite, so seats 0 and 2
// are team 0 and seats 1 and 3 are team 1.
func TeamOf( seat int ) ( team int ) {
	team = seat % 2
	return
}

func nextSeat( seat int ) ( result int ) {
	result = ( seat + 1 ) % 4
	return
}

type Play struct {
	Seat int  `json:"seat"`
	Card Card `json:"card"`
}

type Trick struct {
	Leader int    `json:"leader"`
	Plays  []Play `json:"plays"`
	Winner int    `json:"winner"`
	Points int    `json:"points"`
}

// HandResult is one line of the score sheet.
type HandResult struct {
	Hand        int     `json:"hand"`
	Bidder      int     `json:"bidder"`
	Bid         int     `json:"bid"`
	Trump       Color   `json:"trump"`
	Taken       [2]int  `json:"taken"`       // counters each team captured, kitty included
	Scored      [2]int  `json:"scored"`      // what each team's total moved by
	Totals      [2]int  `json:"totals"`      // running totals after this hand
	TricksWon   [2]int  `json:"tricks_won"`
	Made        bool    `json:"made"`
	Kitty       []Card  `json:"kitty"`       // the buried cards, revealed once the hand is over
	KittyPoints int     `json:"kitty_points"`
	KittyTeam   int     `json:"kitty_team"`  // who won the last trick and with it the kitty
}

// Event is one line of the game log. It carries facts, not sentences: the
// frontend turns {kind: "bid", seat: 2, amount: 85} into text through
// language.yaml, the same way it does error codes.
type Event struct {
	Seq    int    `json:"seq"`
	Kind   string `json:"kind"`
	Seat   int    `json:"seat"`
	Amount int    `json:"amount,omitempty"`
	Card   string `json:"card,omitempty"`
	Color  string `json:"color,omitempty"`
	Team   int    `json:"team"`
}

const maxEvents = 60

type State struct {
	Options    Options `json:"options"`
	Phase      Phase   `json:"phase"`
	HandNumber int     `json:"hand_number"`
	// BidStarter opens the auction and rotates one seat clockwise each hand.
	BidStarter int `json:"bid_starter"`
	Turn       int `json:"turn"`

	Hands [4][]Card `json:"hands"`
	// Kitty is the five dealt cards until someone wins the auction, and the
	// five cards they buried after that.
	Kitty []Card `json:"kitty"`
	// PickedUp remembers which cards came out of the kitty, so the bidder's
	// screen can mark them. It is a copy, never counted as cards in play.
	PickedUp []Card `json:"picked_up,omitempty"`

	Bid        int       `json:"bid"`
	HighBidder int       `json:"high_bidder"` // -1 while nobody has bid
	Passed     [4]bool   `json:"passed"`
	LastBid    [4]int    `json:"last_bid"` // 0 = has not bid this hand
	Bidder     int       `json:"bidder"`   // who won the auction; -1 during it
	Trump      Color     `json:"trump"`

	Trick     Trick   `json:"trick"`
	Completed []Trick `json:"completed"`
	Taken     [2]int  `json:"taken"`
	TricksWon [2]int  `json:"tricks_won"`

	Scores  [2]int       `json:"scores"`
	History []HandResult `json:"history"`
	Ready   [4]bool      `json:"ready"`
	Winner  int          `json:"winner"` // winning team once the game is over, else -1

	Events   []Event `json:"events"`
	EventSeq int     `json:"event_seq"`
}

// NewGame deals the first hand from a freshly shuffled deck. first_bidder is
// usually seat 0; tests vary it.
func NewGame( options Options , first_bidder int , source *rand.Rand ) ( state *State , err error ) {
	state , err = NewGameWithDeck( options , first_bidder , ShuffledDeck( source ) )
	return
}

// NewGameWithDeck deals the first hand from a deck in a known order, which is
// how a recorded game is replayed.
func NewGameWithDeck( options Options , first_bidder int , deck []Card ) ( state *State , err error ) {
	if err = options.Validate(); err != nil { return }
	if ValidDeck( deck ) == false {
		err = ErrBadDeck
		return
	}
	state = &State{
		Options:    options,
		BidStarter: ( ( first_bidder % 4 ) + 4 ) % 4,
		Winner:     -1,
		Trump:      NoColor,
	}
	state.deal( deck )
	return
}

func ( state *State ) deal( deck []Card ) {
	state.HandNumber++
	for seat := 0; seat < 4; seat++ {
		hand := make( []Card , HandSize )
		copy( hand , deck[ seat*HandSize : ( seat+1 )*HandSize ] )
		state.Hands[ seat ] = hand
	}
	state.Kitty = append( []Card( nil ) , deck[ 4*HandSize: ]... )
	state.PickedUp = nil

	state.Phase = PhaseBidding
	state.Turn = state.BidStarter
	state.Bid = OpeningBid
	state.HighBidder = -1
	state.Passed = [4]bool{}
	state.LastBid = [4]int{}
	state.Bidder = -1
	state.Trump = NoColor
	state.Trick = Trick{ Leader: -1 , Winner: -1 }
	state.Completed = nil
	state.Taken = [2]int{}
	state.TricksWon = [2]int{}
	state.Ready = [4]bool{}

	state.log( Event{ Kind: "deal" , Seat: state.BidStarter , Amount: state.HandNumber } )
}

func ( state *State ) log( event Event ) {
	state.EventSeq++
	event.Seq = state.EventSeq
	state.Events = append( state.Events , event )
	if len( state.Events ) > maxEvents {
		state.Events = append( []Event( nil ) , state.Events[ len( state.Events )-maxEvents: ]... )
	}
}

// Dealer is who dealt this hand: the seat just before whoever opens the
// bidding, since the auction starts on the dealer's left. Both move one seat
// clockwise each hand, so it is worked out rather than stored.
func ( state *State ) Dealer() ( seat int ) {
	seat = ( state.BidStarter + 3 ) % 4
	return
}

// IsTurn reports whether seat is the one being waited on. During the hand
// summary every seat that has not confirmed is being waited on.
func ( state *State ) IsTurn( seat int ) ( result bool ) {
	switch state.Phase {
	case PhaseBidding , PhaseKitty , PhasePlaying:
		result = state.Turn == seat
	case PhaseHandOver:
		result = state.Ready[ seat ] == false
	}
	return
}

// suitOf is a card's effective color: the bird belongs to trump.
func ( state *State ) suitOf( card Card ) ( color Color ) {
	color = card.Color
	if card.IsRook() {
		color = state.Trump
	}
	return
}

// CardsInPlay counts every physical card the state holds. It is 57 at every
// step of a game, which the property test checks after each action.
func ( state *State ) CardsInPlay() ( count int ) {
	for seat := 0; seat < 4; seat++ {
		count += len( state.Hands[ seat ] )
	}
	count += len( state.Kitty ) + len( state.Trick.Plays )
	for _ , trick := range state.Completed {
		count += len( trick.Plays )
	}
	return
}
