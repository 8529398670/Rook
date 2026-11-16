package routes

import (
	fiber "github.com/gofiber/fiber/v3"

	apidoc "rook/server/apidoc"
)

// GetOpenAPI hands out config/api.json, which scripts/api_docs.sh writes
// from the routes as each declares itself (the // @route(...) of its
// handler); api_docs_test.go fails while it is out of date. Signed-in callers
// only: it maps the admin surface, and nobody who cannot sign in has a use for
// it.
//
// @route("GET /api/api.json", summary="This document",
//     tag="Monitoring", errors=(404,), reply_type="application/json",
// )
func ( handlers *Handlers ) GetOpenAPI( c fiber.Ctx , in *apidoc.None ) ( err error ) {
	if len( handlers.OpenAPI ) == 0 {
		err = notFound( c , "not found" )
		return
	}
	c.Set( fiber.HeaderContentType , fiber.MIMEApplicationJSON )
	err = c.Send( handlers.OpenAPI )
	return
}

// openAPIURL is where the API reference is read: config/api.json in
// app.repo when that is set, so it opens as a page there, else this server's
// own copy, which needs the same sign-in.
func ( handlers *Handlers ) openAPIURL() ( result string ) {
	result = handlers.Config.RepoFile( "config/api.json" )
	if result == "" {
		result = "/api/api.json"
	}
	return
}
