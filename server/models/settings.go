package models

import (
	errors "errors"

	db "rook/server/db"
	game "rook/server/game"
)

// AppSettings are the few switches an admin flips from the Settings screen
// rather than from config.yaml: they are about how the game is run day to
// day, not how the server is deployed.
type AppSettings struct {
	// AllowJoinSignup lets someone who is not signed in create an account
	// from a table's join link, just by typing a name. Off by default: it is
	// self-service signup for anyone the link reaches.
	AllowJoinSignup bool `json:"allow_join_signup"`
	// ObserversSeeAll lets everyone watching any table see every hand.
	ObserversSeeAll bool `json:"observers_see_all"`
	// TableChatOff and GlobalChatOff switch off the chat at every table and
	// the chat everyone shares. Named for "off" so that a database saved
	// before they existed reads as both on.
	TableChatOff  bool `json:"table_chat_off"`
	GlobalChatOff bool `json:"global_chat_off"`
	// TrickPauseMS is how long a finished trick stays on the table before
	// it is swept to the winner, at every table that has not chosen its
	// own. 0 means DefaultTrickPauseMS, so a database saved before this
	// existed keeps the pause it always had.
	TrickPauseMS int `json:"trick_pause_ms,omitempty"`
	// RookAnytime and RookFirstTrickOff are the bird's house rules, copied
	// into each table when it is created (see game.Options), so a change
	// never alters a game already under way. RookFirstTrickOff is named for
	// "off" so a database saved before it existed keeps the bird allowed on
	// the first trick, as it always was.
	RookAnytime       bool `json:"rook_anytime"`
	RookFirstTrickOff bool `json:"rook_first_trick_off"`
	// RookRank is where the bird sits in trump at every new table, copied
	// in the same way. "" means game.RookHigh, as it always was.
	RookRank game.RookRank `json:"rook_rank,omitempty"`
	// BotNames is where a random bot's name comes from: "" for the built-in
	// list in language.yaml, BotNamesFile for the names file in config.yaml
	// (bot_names_file). BotNamesTop is how many of that file's most popular
	// names are used; 0 means botnames.DefaultTop.
	BotNames    string `json:"bot_names,omitempty"`
	BotNamesTop int    `json:"bot_names_top,omitempty"`
	// NewUser... are what a new account starts with, copied onto it when it
	// is created (see CreateUser), so a change never alters anyone who
	// already has an account. Named for "off", and levels with "" read as
	// DefaultNewUserLevel, so a database saved before they existed gives
	// new players everything: invite, hints, undo and reshuffle allowed,
	// Master everywhere, and both table extras shown.
	NewUserNoInvite       bool   `json:"new_user_no_invite"`
	NewUserNoHint         bool   `json:"new_user_no_hint"`
	NewUserNoUndo         bool   `json:"new_user_no_undo"`
	NewUserNoReshuffle    bool   `json:"new_user_no_reshuffle"`
	NewUserHintLevel      string `json:"new_user_hint_level,omitempty"`
	NewUserBotLevel       string `json:"new_user_bot_level,omitempty"`
	NewUserAutoPlayLevel  string `json:"new_user_auto_play_level,omitempty"`
	NewUserNoPlayerScores bool   `json:"new_user_no_player_scores"`
	NewUserNoLedGlow      bool   `json:"new_user_no_led_glow"`
}

// DefaultNewUserLevel is the level a new player's hints, added bots and
// auto play start on when the owner has not picked another.
const DefaultNewUserLevel = game.LevelMaster

// ValidNewUserLevel accepts "" ("the default") or one of the game's levels.
func ValidNewUserLevel( level string ) ( ok bool ) {
	if level == "" {
		ok = true
		return
	}
	for _ , choice := range game.Levels {
		if string( choice ) == level {
			ok = true
			return
		}
	}
	return
}

func newUserLevel( level string ) ( result string ) {
	result = level
	if result == "" { result = string( DefaultNewUserLevel ) }
	return
}

// NewUserHint, NewUserBot and NewUserAutoPlay are the new-player levels,
// with "" read as the default.
func ( settings AppSettings ) NewUserHint() ( level string ) {
	level = newUserLevel( settings.NewUserHintLevel )
	return
}

func ( settings AppSettings ) NewUserBot() ( level string ) {
	level = newUserLevel( settings.NewUserBotLevel )
	return
}

func ( settings AppSettings ) NewUserAutoPlay() ( level string ) {
	level = newUserLevel( settings.NewUserAutoPlayLevel )
	return
}

// ApplyNewUserDefaults copies the new-player defaults onto an account that
// is being created.
func ( settings AppSettings ) ApplyNewUserDefaults( user *User ) {
	user.CanInvite = settings.NewUserNoInvite == false
	user.CanHint = settings.NewUserNoHint == false
	user.CanUndo = settings.NewUserNoUndo == false
	user.CanReshuffle = settings.NewUserNoReshuffle == false
	user.HintLevel = settings.NewUserHint()
	user.BotLevel = settings.NewUserBot()
	user.AutoPlayLevel = settings.NewUserAutoPlay()
	player_scores := settings.NewUserNoPlayerScores == false
	led_glow := settings.NewUserNoLedGlow == false
	user.PlayerScores = &player_scores
	user.LedGlow = &led_glow
}

// BotNamesFile is the BotNames choice for the names file.
const BotNamesFile = "file"

// MaxBotNamesTop bounds BotNamesTop, well past any names file's length.
const MaxBotNamesTop = 100000

// ValidBotNames accepts the built-in list or the file, and a top of 0 ("the
// default") up to MaxBotNamesTop.
func ValidBotNames( source string , top int ) ( ok bool ) {
	ok = ( source == "" || source == BotNamesFile ) && top >= 0 && top <= MaxBotNamesTop
	return
}

// ValidRookRank accepts "" ("the default") or one of the game's ranks.
func ValidRookRank( rank game.RookRank ) ( ok bool ) {
	switch rank {
	case "" , game.RookHigh , game.RookTenHalf , game.RookLow:
		ok = true
	}
	return
}

// Rank is the app-wide Rook rank, with "" read as the default.
func ( settings AppSettings ) Rank() ( rank game.RookRank ) {
	rank = settings.RookRank
	if rank == "" { rank = game.RookHigh }
	return
}

// DefaultTrickPauseMS is the pause a finished trick has always had.
const DefaultTrickPauseMS = 1100

// TrickPauses are the pauses on offer, in milliseconds: a short list rather
// than any number, so a typo cannot freeze every table for a minute.
var TrickPauses = []int{ 500 , 800 , 1100 , 1500 , 2000 , 2500 , 3000 , 4000 , 5000 }

// ValidTrickPause accepts 0 ("the default") or one of TrickPauses.
func ValidTrickPause( ms int ) ( ok bool ) {
	if ms == 0 {
		ok = true
		return
	}
	for _ , choice := range TrickPauses {
		if choice == ms {
			ok = true
			return
		}
	}
	return
}

// TrickPause is the app-wide pause, with 0 read as the default.
func ( settings AppSettings ) TrickPause() ( ms int ) {
	ms = settings.TrickPauseMS
	if ms == 0 { ms = DefaultTrickPauseMS }
	return
}

var settingsKey = []byte( "app_settings" )

func LoadSettings( store *db.Store ) ( settings AppSettings , err error ) {
	err = store.Get( db.BucketMeta , settingsKey , &settings )
	if errors.Is( err , db.ErrNotFound ) { err = nil }
	return
}

func SaveSettings( store *db.Store , settings AppSettings ) ( err error ) {
	err = store.Put( db.BucketMeta , settingsKey , &settings )
	return
}
