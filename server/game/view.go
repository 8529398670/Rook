package game

import (
	sort "sort"
)

// View is one seat's window onto the game: everything that seat is allowed
// to know and nothing else. It is the only shape of game state that ever
// leaves the server, so redaction happens in exactly one place.
type View struct {
	Phase      Phase   `json:"phase"`
	HandNumber int     `json:"hand_number"`
	Options    Options `json:"options"`
	You        int     `json:"you"` // -1 for a spectator
	Turn       int     `json:"turn"`
	BidStarter int     `json:"bid_starter"`
	Dealer     int     `json:"dealer"`

	Hand  []string `json:"hand"`  // sorted for display
	Legal []string `json:"legal"` // empty unless it is your turn to play
	// PickedUp marks which of your cards came from the kitty, while you are
	// choosing what to bury.
	PickedUp []string `json:"picked_up"`

	Bid        int    `json:"bid"`
	HighBidder int    `json:"high_bidder"`
	MinBid     int    `json:"min_bid"`
	MaxBid     int    `json:"max_bid"`
	Bidder     int    `json:"bidder"`
	Trump      string `json:"trump"`
	// Auction is this hand's bids in the order they were made, while it is
	// being bid for and while the winner buries. Empty the rest of the time.
	Auction []AuctionEntry `json:"auction"`

	Seats     [4]SeatView `json:"seats"`
	Trick     TrickView   `json:"trick"`
	LastTrick *TrickView  `json:"last_trick"`
	TrickNumber int       `json:"trick_number"` // 1-13 during play
	TricksWon [2]int      `json:"tricks_won"`
	Taken     [2]int      `json:"taken"` // counters in tricks won so far (kitty not yet counted)

	Scores  [2]int       `json:"scores"`
	History []HandResult `json:"history"` // Card and Color marshal to their wire names
	Ready   [4]bool          `json:"ready"`
	Winner  int              `json:"winner"`
	Events  []Event          `json:"events"`
	// Hands is every seat's hand, filled in only for someone watching who is
	// allowed to see them all (see the tables package). Nil otherwise.
	Hands [][]string `json:"hands,omitempty"`
}

// AuctionEntry is one turn of the bidding: a bid, or a pass (amount 0).
type AuctionEntry struct {
	Seat   int `json:"seat"`
	Amount int `json:"amount"`
}

type SeatView struct {
	Cards   int  `json:"cards"`
	Passed  bool `json:"passed"`
	LastBid int  `json:"last_bid"`
	Taken   int  `json:"taken"` // counters in the tricks this seat won this hand
}

type PlayView struct {
	Seat int    `json:"seat"`
	Card string `json:"card"`
}

type TrickView struct {
	Leader int        `json:"leader"`
	Plays  []PlayView `json:"plays"`
	Winner int        `json:"winner"`
	Points int        `json:"points"`
}

const viewEvents = 30

// ViewFor builds the snapshot for seat (or -1 for someone watching).
func ( state *State ) ViewFor( seat int ) ( view View ) {
	view = View{
		Phase:      state.Phase,
		HandNumber: state.HandNumber,
		Options:    state.Options,
		You:        seat,
		Turn:       state.Turn,
		BidStarter: state.BidStarter,
		Dealer:     state.Dealer(),
		Hand:       []string{},
		Legal:      []string{},
		PickedUp:   []string{},
		Bid:        state.Bid,
		HighBidder: state.HighBidder,
		MinBid:     state.MinNextBid(),
		MaxBid:     MaxBid,
		Bidder:     state.Bidder,
		Auction:    state.auction(),
		TricksWon:  state.TricksWon,
		Taken:      state.Taken,
		Scores:     state.Scores,
		// The buried kitty is only revealed once its hand is over, inside the
		// score sheet. Until then it is in no view at all.
		Ready:      state.Ready,
		Winner:     state.Winner,
		History:    append( []HandResult{} , state.History... ),
		Trick:      trickView( state.Trick ),
	}
	if state.Trump != NoColor {
		view.Trump = state.Trump.String()
	}
	if state.Phase == PhasePlaying {
		view.TrickNumber = len( state.Completed ) + 1
	}

	if seat >= 0 && seat < 4 {
		view.Hand = cardIDs( state.SortedHand( seat ) )
		view.Legal = cardIDs( state.LegalCards( seat ) )
		if state.Phase == PhaseKitty && state.Bidder == seat {
			view.PickedUp = cardIDs( state.PickedUp )
		}
	}

	taken := state.seatTaken()
	for index := 0; index < 4; index++ {
		view.Seats[ index ] = SeatView{
			Cards:   len( state.Hands[ index ] ),
			Passed:  state.Passed[ index ],
			LastBid: state.LastBid[ index ],
			Taken:   taken[ index ],
		}
	}

	if count := len( state.Completed ); count > 0 {
		last := trickView( state.Completed[ count-1 ] )
		view.LastTrick = &last
	}

	events := state.Events
	if len( events ) > viewEvents {
		events = events[ len( events )-viewEvents: ]
	}
	view.Events = append( []Event{} , events... )
	return
}

// auction reads this hand's bids back out of the event log. The log always
// holds a whole auction: nothing but bids and passes is logged between the
// deal and the bid being won, and a full auction is far shorter than
// maxEvents.
func ( state *State ) auction() ( entries []AuctionEntry ) {
	entries = []AuctionEntry{}
	if state.Phase != PhaseBidding && state.Phase != PhaseKitty { return }
	start := 0
	for index := len( state.Events ) - 1; index >= 0; index-- {
		if kind := state.Events[ index ].Kind; kind == "deal" || kind == "reshuffle" {
			start = index + 1
			break
		}
	}
	for _ , event := range state.Events[ start: ] {
		switch event.Kind {
		case "bid":
			entries = append( entries , AuctionEntry{ Seat: event.Seat , Amount: event.Amount } )
		case "pass":
			entries = append( entries , AuctionEntry{ Seat: event.Seat } )
		}
	}
	return
}

// seatTaken is the counters in the tricks each seat has won this hand. Once
// the hand is over the kitty joins whoever took the last trick, as it does in
// finishHand, so a team's two seats always add up to its Taken. Its points
// are on the score sheet by then, so this gives nothing away.
func ( state *State ) seatTaken() ( taken [4]int ) {
	for _ , trick := range state.Completed {
		taken[ trick.Winner ] += trick.Points
	}
	over := state.Phase == PhaseHandOver || state.Phase == PhaseGameOver
	if over && len( state.Completed ) == HandSize {
		last := state.Completed[ len( state.Completed )-1 ]
		for _ , card := range state.Kitty {
			taken[ last.Winner ] += card.Points()
		}
	}
	return
}

func trickView( trick Trick ) ( view TrickView ) {
	view = TrickView{ Leader: trick.Leader , Winner: trick.Winner , Points: trick.Points , Plays: []PlayView{} }
	for _ , play := range trick.Plays {
		view.Plays = append( view.Plays , PlayView{ Seat: play.Seat , Card: play.Card.ID() } )
	}
	return
}

// AllHands is every seat's hand in display order, for a permitted watcher.
func ( state *State ) AllHands() ( hands [][]string ) {
	hands = make( [][]string , 4 )
	for seat := 0; seat < 4; seat++ {
		hands[ seat ] = cardIDs( state.SortedHand( seat ) )
	}
	return
}

// SortedHand groups a hand by color with trump last (the rightmost, where a
// thumb naturally reaches), each color low to high with the 1 at the top, and
// the bird inside trump at its rank. Before trump is named the bird goes last.
func ( state *State ) SortedHand( seat int ) ( hand []Card ) {
	hand = append( []Card( nil ) , state.Hands[ seat ]... )
	group := func( card Card ) ( result int ) {
		color := state.suitOf( card )
		if card.IsRook() && state.Trump == NoColor {
			result = 10
			return
		}
		if color == state.Trump {
			result = 9
			return
		}
		for index , candidate := range Colors {
			if candidate == color { result = index }
		}
		return
	}
	sort.SliceStable( hand , func( i int , j int ) bool {
		group_i , group_j := group( hand[ i ] ) , group( hand[ j ] )
		if group_i != group_j { return group_i < group_j }
		return hand[ i ].rank( state.Options.RookRank ) < hand[ j ].rank( state.Options.RookRank )
	} )
	return
}
