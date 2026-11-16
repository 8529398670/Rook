package routes

import (
	fiber "github.com/gofiber/fiber/v3"

	apidoc "rook/server/apidoc"
)

// The readable reference: config/api.md, the same as api.json but as tables
// for people. With app.repo set, the API keys card links to the repository's
// own rendering of it; without it, to this server's /docs/api page
// (static/docs.html), which fetches the Markdown from the route below and
// renders it.

// GetAPIDoc hands out config/api.md, which scripts/api_docs.sh writes beside
// api.json. Signed-in callers only, like api.json.
//
// @route("GET /api/api.md", summary="The API reference as Markdown tables for people",
//     tag="Monitoring", errors=(404,), reply_type="text/markdown",
// )
func ( handlers *Handlers ) GetAPIDoc( c fiber.Ctx , in *apidoc.None ) ( err error ) {
	if len( handlers.APIDoc ) == 0 {
		err = notFound( c , "not found" )
		return
	}
	c.Set( fiber.HeaderContentType , "text/markdown; charset=utf-8" )
	err = c.Send( handlers.APIDoc )
	return
}

// apiDocURL is where the readable API reference is read: config/api.md in
// app.repo when that is set, so the repository's site renders it, else this
// server's own page.
func ( handlers *Handlers ) apiDocURL() ( result string ) {
	result = handlers.Config.RepoFile( "config/api.md" )
	if result == "" {
		result = "/docs/api"
	}
	return
}

// ShowDocsPage is /docs/api: static/docs.html, which reads its own path to
// pick the file and shows an error for any other /docs/ path. The page
// itself is public, like index.html; the Markdown it fetches needs a sign-in.
func ( handlers *Handlers ) ShowDocsPage( c fiber.Ctx ) ( err error ) {
	c.Set( fiber.HeaderReferrerPolicy , "no-referrer" )
	c.Set( "X-Robots-Tag" , "noindex" )
	err = handlers.Static.ServeFile( c , "docs.html" )
	return
}
