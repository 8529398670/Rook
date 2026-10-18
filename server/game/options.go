package game

// Error is a machine-readable refusal. The engine never produces English:
// the frontend maps each code to a sentence in language.yaml (errors.<code>),
// so rewording a message is a YAML edit and the API stays stable.
type Error string

func ( e Error ) Error() ( result string ) {
	result = string( e )
	return
}

const (
	ErrWrongPhase      Error = "wrong_phase"
	ErrNotYourTurn     Error = "not_your_turn"
	ErrBidTooLow       Error = "bid_too_low"
	ErrBidTooHigh      Error = "bid_too_high"
	ErrBidStep         Error = "bid_not_multiple_of_five"
	ErrBadCard         Error = "bad_card"
	ErrNotInHand       Error = "card_not_in_hand"
	ErrMustFollowSuit  Error = "must_follow_suit"
	ErrRookFirstTrick  Error = "rook_not_on_first_trick"
	ErrMustBuryFive    Error = "must_bury_five"
	ErrDuplicateCard   Error = "duplicate_card"
	ErrBadTrump        Error = "bad_trump"
	ErrBadOptions      Error = "bad_options"
	ErrAlreadyReady    Error = "already_ready"
	ErrBadDeck         Error = "bad_deck"
)

// RookRank is where the bird sits inside trump.
type RookRank string

const (
	RookHigh    RookRank = "high"     // above the 1: the strongest card in the game
	RookTenHalf RookRank = "ten_half" // the official rule: beats the 10, loses to the 11
	RookLow     RookRank = "low"      // below the 2: the weakest trump
)

// Options are chosen by whoever creates the table. The defaults reproduce the
// house rules of the original PHP version of this game.
type Options struct {
	RookRank RookRank `json:"rook_rank"`
	// RookAnytime is the official Hasbro rule: the bird may be played on any
	// trick, even by someone who could follow the color led.
	RookAnytime bool `json:"rook_anytime"`
	// RookFirstTrick false forbids the bird on the hand's first trick.
	RookFirstTrick bool `json:"rook_first_trick"`
	// PlayTo is the winning total; 0 means a single hand decides it.
	PlayTo int `json:"play_to"`
}

var PlayToChoices = []int{ 0 , 100 , 300 , 500 }

func DefaultOptions() ( options Options ) {
	options = Options{
		RookRank:       RookHigh,
		RookAnytime:    false,
		RookFirstTrick: true,
		PlayTo:         500,
	}
	return
}

func ( options Options ) Validate() ( err error ) {
	switch options.RookRank {
	case RookHigh , RookTenHalf , RookLow:
	default:
		err = ErrBadOptions
		return
	}
	for _ , choice := range PlayToChoices {
		if options.PlayTo == choice { return }
	}
	err = ErrBadOptions
	return
}

// Bidding limits. Bids open at 65 with nobody holding them: the first real bid
// must beat it, and a player left alone after three passes takes the hand at
// whatever the bid stands at -- 65 if nobody bid at all.
const (
	OpeningBid = 65
	BidStep    = 5
	MaxBid     = TotalPoints
	HandSize   = 13
	KittySize  = 5
)
