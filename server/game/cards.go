// Package game is the Rook rules engine: dealing, bidding, the kitty, trick
// play, and scoring. It is pure -- no I/O, no clock, no globals it mutates --
// so every rule can be unit tested by constructing a State and calling it.
//
// It lives on the server, which is an exception to the template's "keep the
// server thin" rule, and a deliberate one: a hand of cards is hidden
// information. If the browser enforced the rules, the browser would have to
// know every hand, and anyone with dev tools could read them. So the server
// is the only authority, and the frontend renders what it is told -- including
// which cards are legal to play, so the rules are never written twice.
package game

import (
	fmt "fmt"
	rand "math/rand/v2"
	strconv "strconv"
)

// Color is a card's suit. The Rook bird has no color of its own: once trump
// is named it plays as the highest (or lowest, or middle) card of trump.
type Color int

const (
	NoColor Color = iota - 1
	Red
	Black
	Yellow
	Green
	RookBird
)

// Colors is the order hands are sorted in. Red and green sit apart so the two
// colors most often confused are never neighbours in a hand.
var Colors = [4]Color{ Red , Black , Yellow , Green }

var colorNames = map[ Color ]string{ Red: "red" , Black: "black" , Yellow: "yellow" , Green: "green" , RookBird: "rook" }
var colorLetters = map[ Color ]string{ Red: "R" , Black: "B" , Yellow: "Y" , Green: "G" }

func ( color Color ) String() ( result string ) {
	result = colorNames[ color ]
	return
}

func ( color Color ) MarshalText() ( result []byte , err error ) {
	result = []byte( colorNames[ color ] )
	return
}

func ( color *Color ) UnmarshalText( raw []byte ) ( err error ) {
	parsed , ok := ParseColor( string( raw ) )
	if ok == false && len( raw ) > 0 {
		err = fmt.Errorf( "unknown color %q" , raw )
		return
	}
	*color = parsed
	return
}

// ParseColor accepts only the four suits -- the bird is never a valid trump.
func ParseColor( name string ) ( color Color , ok bool ) {
	color = NoColor
	for _ , candidate := range Colors {
		if colorNames[ candidate ] == name {
			color = candidate
			ok = true
			return
		}
	}
	return
}

// Card is one of the 57: four colors numbered 1-14, plus the Rook.
type Card struct {
	Color  Color
	Number int
}

const RookID = "ROOK"

var TheRook = Card{ Color: RookBird , Number: 0 }

func ( card Card ) IsRook() ( result bool ) {
	result = card.Color == RookBird
	return
}

// ID is the compact wire form: "R14", "G1", "ROOK". Short enough that a full
// table snapshot stays small, readable enough to debug from a network tab.
func ( card Card ) ID() ( result string ) {
	if card.IsRook() {
		result = RookID
		return
	}
	result = colorLetters[ card.Color ] + strconv.Itoa( card.Number )
	return
}

func ( card Card ) String() ( result string ) {
	result = card.ID()
	return
}

func ParseCard( id string ) ( card Card , err error ) {
	if id == RookID {
		card = TheRook
		return
	}
	if len( id ) < 2 || len( id ) > 3 {
		err = ErrBadCard
		return
	}
	card.Color = NoColor
	for color , letter := range colorLetters {
		if id[ :1 ] == letter {
			card.Color = color
		}
	}
	number , parse_err := strconv.Atoi( id[ 1: ] )
	if card.Color == NoColor || parse_err != nil || number < 1 || number > 14 || id[ 1 ] == '0' {
		err = ErrBadCard
		return
	}
	card.Number = number
	return
}

func ( card Card ) MarshalText() ( result []byte , err error ) {
	result = []byte( card.ID() )
	return
}

func ( card *Card ) UnmarshalText( raw []byte ) ( err error ) {
	*card , err = ParseCard( string( raw ) )
	return
}

// Points are the "counters". Four colors of 5+10+14+1 is 160, plus the Rook's
// 20 is 180 -- which is why 180 is the highest possible bid.
func ( card Card ) Points() ( result int ) {
	switch {
	case card.IsRook():
		result = 20
	case card.Number == 1:
		result = 15
	case card.Number == 10 || card.Number == 14:
		result = 10
	case card.Number == 5:
		result = 5
	}
	return
}

// TotalPoints is what every hand's counters add up to, kitty included.
const TotalPoints = 180

// rank orders cards within a color, doubled so the official "Rook is 10½"
// rule fits between two integers. The 1 is the highest numbered card.
func ( card Card ) rank( rook_rank RookRank ) ( result int ) {
	if card.IsRook() {
		switch rook_rank {
		case RookLow:
			result = 1
		case RookTenHalf:
			result = 21
		default:
			result = 32
		}
		return
	}
	if card.Number == 1 {
		result = 30
		return
	}
	result = card.Number * 2
	return
}

// NewDeck returns all 57 cards in a fixed order.
func NewDeck() ( deck []Card ) {
	deck = make( []Card , 0 , 57 )
	for _ , color := range Colors {
		for number := 1; number <= 14; number++ {
			deck = append( deck , Card{ Color: color , Number: number } )
		}
	}
	deck = append( deck , TheRook )
	return
}

// ShuffledDeck is a fresh deck in random order. The tables package draws one
// for every deal and records it, which is what makes a game replayable: the
// same deck plus the same moves always reproduces the same position, so an
// admin can step back through a game and forward again.
func ShuffledDeck( source *rand.Rand ) ( deck []Card ) {
	deck = NewDeck()
	Shuffle( deck , source )
	return
}

// ValidDeck reports whether deck is exactly the 57 cards, each once -- the
// check that keeps a stored or replayed deal from inventing cards.
func ValidDeck( deck []Card ) ( ok bool ) {
	if len( deck ) != 57 { return }
	unseen := map[ Card ]bool{}
	for _ , card := range NewDeck() {
		unseen[ card ] = true
	}
	for _ , card := range deck {
		if unseen[ card ] == false { return }
		delete( unseen , card )
	}
	ok = true
	return
}

// Shuffle uses the given source when there is one (tests pass a seeded one so
// a deal is reproducible) and math/rand/v2's randomly seeded global otherwise.
func Shuffle( deck []Card , source *rand.Rand ) {
	swap := func( i int , j int ) { deck[ i ] , deck[ j ] = deck[ j ] , deck[ i ] }
	if source != nil {
		source.Shuffle( len( deck ) , swap )
		return
	}
	rand.Shuffle( len( deck ) , swap )
}

func containsCard( cards []Card , target Card ) ( result bool ) {
	for _ , card := range cards {
		if card == target {
			result = true
			return
		}
	}
	return
}

func removeCard( cards []Card , target Card ) ( result []Card , found bool ) {
	result = make( []Card , 0 , len( cards ) )
	for _ , card := range cards {
		if card == target && found == false {
			found = true
			continue
		}
		result = append( result , card )
	}
	return
}

func cardIDs( cards []Card ) ( result []string ) {
	result = make( []string , 0 , len( cards ) )
	for _ , card := range cards {
		result = append( result , card.ID() )
	}
	return
}
