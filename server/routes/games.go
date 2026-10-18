package routes

import (
	strconv "strconv"

	fiber "github.com/gofiber/fiber/v3"
)

// The game history: every finished or abandoned game, open to any signed-in
// player. See tables/archive.go for what is kept and when.

const (
	gamesPageDefault = 50
	gamesPageMax     = 200
)

func ( handlers *Handlers ) ListGames( c fiber.Ctx ) ( err error ) {
	offset , offset_err := strconv.Atoi( c.Query( "offset" , "0" ) )
	if offset_err != nil || offset < 0 { offset = 0 }
	limit , limit_err := strconv.Atoi( c.Query( "limit" , "0" ) )
	if limit_err != nil || limit <= 0 { limit = gamesPageDefault }
	if limit > gamesPageMax { limit = gamesPageMax }

	games , list_err := handlers.Hub.Games( offset , limit+1 )
	if list_err != nil {
		err = serverError( c )
		return
	}
	more := len( games ) > limit
	if more { games = games[ :limit ] }
	err = c.JSON( fiber.Map{ "games": games , "more": more } )
	return
}

func ( handlers *Handlers ) GetGame( c fiber.Ctx ) ( err error ) {
	entry , get_err := handlers.Hub.Game( c.Params( "game_id" ) )
	if get_err != nil {
		err = handlers.tableError( c , get_err )
		return
	}
	err = c.JSON( entry )
	return
}
