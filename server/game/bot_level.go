package game

// Level is how well a computer player plays. Each seat's bot has its own, so
// a table can pair a beginner-friendly partner with sharp opponents. Hints,
// and auto play unless the player picks another level, use HelpLevel,
// whatever the bots at the table are set to.
type Level string

const (
	// LevelEasy makes the mistakes of someone just learning: it wins
	// tricks with its biggest card, throws points to whoever is winning,
	// and never thinks about its partner.
	LevelEasy Level = "easy"
	// LevelNormal is the original bot, kept exactly as it was.
	LevelNormal Level = "normal"
	// LevelSkilled plays the strategy guide -- draws trump as the bidder,
	// never leads trump defending, feeds a partner's tricks -- but does not
	// count cards: it judges from its own hand and the trick in front of it.
	LevelSkilled Level = "skilled"
	// LevelHard plays the same guide and counts cards: it knows which top
	// cards are still out and who has run out of a color.
	LevelHard Level = "hard"
	// LevelExpert plays Hard's rules forward through 100 imagined deals of
	// the cards it cannot see, and picks what does best on average.
	LevelExpert Level = "expert"
	// LevelMaster is Expert reading the table: it imagines the deals that
	// best explain the bids, the passes and the trump call (bot_reading.go).
	LevelMaster Level = "master"
)

// Levels is every level, weakest first. Each beats the one below it.
// Measured by the duplicate league in bot_league_test.go (ROOK_BOT_LEAGUE=1),
// points a hand: Normal over Easy +20, Skilled over Normal +22, Hard over
// Skilled +14 (3000 deals each), Expert over Hard +26 and Master over Expert
// +9 (300 deals each, where results move by about 5 from one set to the
// next).
var Levels = []Level{ LevelEasy , LevelNormal , LevelSkilled , LevelHard , LevelExpert , LevelMaster }

// HelpLevel is what hints use, and auto play unless the player picked
// another level in Settings.
const HelpLevel = LevelExpert

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
