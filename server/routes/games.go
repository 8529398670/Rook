package routes

import (
	strconv "strconv"

	fiber "github.com/gofiber/fiber/v3"

	apidoc "rook/server/apidoc"
)

// The game history: every finished or abandoned game, open to any signed-in
// player; only the owner may delete one. See tables/archive.go for what is
// kept and when.

const (
	gamesPageDefault = 50
	gamesPageMax     = 200
)

// @route("GET /api/games", summary="List games", tag="Games",
// )
func ( handlers *Handlers ) ListGames( c fiber.Ctx , in *apidoc.None ) ( err error ) {
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

// @route("GET /api/games/{game_id}", summary="Get game", tag="Games",
// )
func ( handlers *Handlers ) GetGame( c fiber.Ctx , in *getGameIn ) ( err error ) {
	entry , get_err := handlers.Hub.Game( c.Params( "game_id" ) )
	if get_err != nil {
		err = handlers.tableError( c , get_err )
		return
	}
	err = c.JSON( entry )
	return
}

// DeleteGame removes a game from the history. Registered behind RequireOwner.
//
// @route("POST /api/games/{game_id}/delete", summary="Delete game", tag="Games", use=[handlers.RequireOwner],
// )
func ( handlers *Handlers ) DeleteGame( c fiber.Ctx , in *deleteGameIn ) ( err error ) {
	if delete_err := handlers.Hub.DeleteGame( c.Params( "game_id" ) ); delete_err != nil {
		err = handlers.tableError( c , delete_err )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}
