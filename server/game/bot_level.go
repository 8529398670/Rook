package game

// Level is how well a computer player plays. Each seat's bot has its own, so
// a table can pair a beginner-friendly partner with sharp opponents. Hints
// and auto play always use BestLevel: a suggestion should be the best move
// the server knows, whatever the bots at the table are set to.
type Level string

const (
	// LevelEasy makes the mistakes of someone just learning: it wins
	// tricks with its biggest card, throws points to whoever is winning,
	// and never thinks about its partner.
	LevelEasy Level = "easy"
	// LevelNormal is the original bot, kept exactly as it was.
	LevelNormal Level = "normal"
	// LevelHard plays the strategy guide: it counts cards, knows which are
	// still out, draws trump as the bidder and feeds a partner's sure
	// tricks.
	LevelHard Level = "hard"
	// LevelExpert plays Hard's rules forward through many possible deals of
	// the cards it cannot see, and picks what does best on average.
	LevelExpert Level = "expert"
)

// Levels is every level, weakest first. Each beats the one below it by a
// clear margin. Measured by the duplicate league in bot_league_test.go
// (ROOK_BOT_LEAGUE=1), points a hand: Normal over Easy +20, Hard over
// Normal +43 (3000 deals each), Expert over Hard +26 (300 deals).
var Levels = []Level{ LevelEasy , LevelNormal , LevelHard , LevelExpert }

// BestLevel is what hints and auto play use.
const BestLevel = LevelExpert

const ErrBadLevel Error = "bad_bot_level"

// ParseLevel accepts the four names, and "" for a seat saved before levels
// existed, which plays as Normal.
func ParseLevel( name string ) ( level Level , ok bool ) {
	level = LevelNormal
	if name == "" {
		ok = true
		return
	}
	for _ , candidate := range Levels {
		if string( candidate ) == name {
			level , ok = candidate , true
			return
		}
	}
	return
}

// orNormal reads an unset level as Normal.
func ( level Level ) orNormal() ( result Level ) {
	result = level
	if result == "" { result = LevelNormal }
	return
}
