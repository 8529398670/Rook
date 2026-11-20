// Package routes contains one file per group of endpoints, plus this file,
// which is the only place that knows the full URL map.
//
// The API here stays deliberately small: it covers logging in, managing who
// can log in, issuing the API keys that let non-browser callers in, and
// reading the UI's text. Everything else an app does belongs in a new file in
// this package (server side) and in static/js/ (browser side) -- see
// references/architecture.md for where the line sits and why.
package routes

import (
	bytes "bytes"
	json "encoding/json"
	errors "errors"
	fs "io/fs"
	log "log"

	fiber "github.com/gofiber/fiber/v3"

	apidoc "rook/server/apidoc"
	botnames "rook/server/botnames"
	config "rook/server/config"
	db "rook/server/db"
	livereload "rook/server/livereload"
	language "rook/server/language"
	logs "rook/server/logs"
	models "rook/server/models"
	security "rook/server/security"
	static "rook/server/static"
	stream "rook/server/stream"
	tables "rook/server/tables"
)

// Handlers carries the dependencies every route group needs. Passing this in
// rather than reaching for package-level globals is what makes the handlers
// testable and keeps the wiring visible in main.go.
type Handlers struct {
	Store    *db.Store
	Config   *config.Config
	Guard    *security.Guard
	Language *language.Language
	Static   *static.Server
	Hub      *tables.Hub
	// Backgrounds keeps the table pictures (models/backgrounds.go).
	Backgrounds *models.Backgrounds
	// FileBotNames is cfg.BotNamesFile's names, most popular first, or empty
	// when there is no such file. See randomBotNames.
	FileBotNames []string
	// Version is the running build, set by main.go. Every API reply says it
	// (X-App-Version) so an open page can notice an upgrade.
	Version string
	// LiveReload is set by main.go when server.live_reload says open pages
	// hear about frontend changes. Nil, the route does not exist.
	LiveReload *livereload.Watcher
	// Logs is the activity log (see server/logs), and LogStream tells an open
	// Activity log screen that more has been written. main.go sets both.
	Logs      *logs.Logger
	LogStream *stream.Hub
	// OpenAPI is config/api.json and APIDoc is config/api.md, set by
	// main.go; /docs/api renders APIDoc.
	OpenAPI []byte
	APIDoc  []byte
	// uiEventBudget caps what one account's page may add to the log. See
	// RecordUIEvents.
	uiEventBudget eventBudget
}

func New( store *db.Store , cfg *config.Config , guard *security.Guard , lang *language.Language , static_server *static.Server , hub *tables.Hub ) ( handlers *Handlers ) {
	handlers = &Handlers{
		Store:    store,
		Config:   cfg,
		Guard:    guard,
		Language: lang,
		Static:   static_server,
		Hub:      hub,
		Backgrounds: models.NewBackgrounds( store , cfg.StorageDir , cfg.SecretKey ),
	}
	return
}

// LoadBotNames reads bot_names_file from disk. When that is the default
// path and nothing is there, embedded -- the copy compiled into the binary --
// stands in, the same disk-first rule as the frontend. A path someone set
// themselves never falls back, so a typo shows in the log instead of quietly
// giving biblical names.
func ( handlers *Handlers ) LoadBotNames( embedded []byte ) {
	path := handlers.Config.BotNamesFile
	if path == "" { return }
	names , err := botnames.Load( path )
	source := path
	if errors.Is( err , fs.ErrNotExist ) && handlers.Config.BotNamesFileIsDefault && len( embedded ) > 0 {
		names , err = botnames.Parse( bytes.NewReader( embedded ) )
		source = "embedded " + path
	}
	if err != nil {
		log.Printf( "[bots] no names from %s: %v" , source , err )
		return
	}
	handlers.FileBotNames = names
	log.Printf( "[bots] %d names from %s" , len( names ) , source )
}

//go:generate go run ../apidoc/gen

// Register wires every route. The API routes are declared beside their
// handlers, each with its // @route(...) declaration, and mountDeclared
// (routes_generated.go, written by go generate) mounts them, the most
// specific path first, each behind the guards its declaration says:
//
//   - Anyone signed in, by a browser session or an API key, is the default.
//     role=ADMIN is the admins; use=[handlers.RequireOwner] narrows that to
//     the owner (the superadmin, models/superadmin.go), by session or key.
//     role=OWNER is the owner from a browser session only (the activity log).
//   - access=SESSION_ONLY turns an API key away: minting and managing API
//     keys (a leaked key must never leave a fresh key behind), deleting your
//     own account, and the seat and sign-up links. The owner's admin key may
//     create accounts and reissue their login links, and a key whose account
//     switched Sign-in links on may make its own account a one-time link; each
//     such link records the key, and revoking the key voids it
//     (models.VoidLinksMintedByKeyTx).
//   - access=PUBLIC needs no sign-in: logging in, the UI's text, health, and
//     the watch, join and sign-up links, whose credential is the link.
//   - A feature that can be switched off while the server runs has a switch
//     in featureGuards: its routes answer 404 while it is off.
//
// Every signed-in POST carries the session's CSRF token in its body; the
// route table checks it before the handler runs (apidoc.CheckCSRF).
//
// Read routes_generated.go to see the whole surface of this server. Only the
// page a login link opens and the catch-alls are written out here.
func ( handlers *Handlers ) Register( app *fiber.App ) {
	// Resolve the session once, before anything else looks at the request.
	app.Use( handlers.Guard.LoadUser )
	apidoc.CheckCSRF = handlers.Guard.CheckCSRF

	// Every API reply says which build answered it, so a page left open
	// across a deploy finds out on its next call (static/js/fresh.js).
	app.Use( "/api" , func( c fiber.Ctx ) ( err error ) {
		c.Set( "X-App-Version" , handlers.Version )
		err = c.Next()
		return
	} )

	// The page a login link opens. Opening one only shows a page; the link
	// is spent by the POST that page sends when the person confirms. See
	// auth.go.
	app.Get( "/login/*" , handlers.ShowLoginPage )
	// The readable API reference, for an app with no app.repo to link to
	// (api_docs_md.go).
	app.Get( "/docs/*" , handlers.ShowDocsPage )

	handlers.mountDeclared( app )

	// Open pages wait here to hear about a change to the frontend
	// (server.live_reload; server/livereload, static/js/live.js). Not part of
	// the API.
	if handlers.LiveReload != nil {
		app.Get( "/api/live" , handlers.LiveReload.Handler )
	}

	// Any other /api/ path is a JSON 404, not a static page.
	app.Get( "/api/*" , func( c fiber.Ctx ) ( err error ) {
		err = c.Status( fiber.StatusNotFound ).JSON( fiber.Map{ "error": "not found" } )
		return
	} )

	// Catch-all static serving. Must be registered last -- it matches every
	// remaining path, so anything added after this line never runs.
	app.Get( "/*" , handlers.Static.Handler )
}

// featureGuards are the switches of the features that can be off while the
// server runs, by feature name: a route whose op says feature=x answers 404
// while x is off, exactly what an unregistered route would say. The route
// table puts the switch in front of every other guard. apidoc/gen reads
// this map's keys.
func ( handlers *Handlers ) featureGuards() ( guards map[string]fiber.Handler ) {
	guards = map[string]fiber.Handler{
		"api_keys": handlers.RequireAPIKeys,
		"logs":     handlers.RequireLogs,
	}
	return
}

// cfg is the running configuration, under the name the template's handlers
// read it by.
func ( handlers *Handlers ) cfg() ( cfg *config.Config ) {
	cfg = handlers.Config
	return
}

// badRequest / notFound keep error bodies uniform, so the frontend has one
// shape to handle and no handler accidentally leaks internal detail.
func badRequest( c fiber.Ctx , message string ) ( err error ) {
	err = c.Status( fiber.StatusBadRequest ).JSON( fiber.Map{ "error": message } )
	return
}

func notFound( c fiber.Ctx , message string ) ( err error ) {
	err = c.Status( fiber.StatusNotFound ).JSON( fiber.Map{ "error": message } )
	return
}

func forbidden( c fiber.Ctx , message string ) ( err error ) {
	err = c.Status( fiber.StatusForbidden ).JSON( fiber.Map{ "error": message } )
	return
}

// serverError answers 500 without saying why. A cause, when given, goes to
// the server's log instead, where the operator can see it.
func serverError( c fiber.Ctx , causes ...error ) ( err error ) {
	for _ , cause := range causes {
		if cause != nil {
			log.Printf( "[error] %s %s: %v" , c.Method() , c.Path() , cause )
		}
	}
	err = c.Status( fiber.StatusInternalServerError ).JSON( fiber.Map{ "error": "internal error" } )
	return
}

func conflict( c fiber.Ctx , message string , extra fiber.Map ) ( err error ) {
	body := fiber.Map{ "error": message }
	for name , value := range extra {
		body[ name ] = value
	}
	err = c.Status( fiber.StatusConflict ).JSON( body )
	return
}

// refusal is a rule saying no, kept apart from how it is delivered: a handler
// sends it as a status and an error body. A declared route's handler may also
// return it as its error (apidoc.Sender).
type refusal struct {
	status  int
	message string
	// cause is why a 5xx happened. It goes to the log, never to the caller.
	cause error
	// extra is more for the caller beside the message, such as the current
	// value a 409 was about.
	extra fiber.Map
}

func ( refused *refusal ) Error() ( text string ) {
	text = refused.message
	if refused.cause != nil { text = refused.cause.Error() }
	return
}

func ( refused *refusal ) Send( c fiber.Ctx ) ( err error ) {
	err = refused.send( c )
	return
}

func ( refused *refusal ) Refused() ( status int , message string ) {
	status , message = refused.status , refused.message
	return
}

func ( refused *refusal ) send( c fiber.Ctx ) ( err error ) {
	if refused.status >= 500 {
		err = serverError( c , refused.cause )
		return
	}
	body := fiber.Map{ "error": refused.message }
	for name , value := range refused.extra {
		body[ name ] = value
	}
	err = c.Status( refused.status ).JSON( body )
	return
}

// refuse is a refusal with the caller's mistake in it. internalFailure is
// ours: a 500 whose cause goes to the log.
func refuse( status int , message string , extra ...fiber.Map ) ( refused *refusal ) {
	refused = &refusal{ status: status , message: message }
	if len( extra ) > 0 { refused.extra = extra[ 0 ] }
	return
}

func internalFailure( cause error ) ( refused *refusal ) {
	refused = &refusal{ status: fiber.StatusInternalServerError , message: "internal error" , cause: cause }
	return
}

// jsonObjects is rows as the JSON objects they encode to, for a reply that
// passes on what another package keeps without saying its shape.
func jsonObjects[ T any ]( rows []T ) ( objects []map[string]any , err error ) {
	objects = []map[string]any{}
	encoded , err := json.Marshal( rows )
	if err != nil { return }
	err = json.Unmarshal( encoded , &objects )
	if objects == nil { objects = []map[string]any{} }
	return
}

// actOnRefusal turns an error from a model that checked models.mayActOnTx
// into the answer every account route gives: 404 for a missing account, 403
// with the same wording for the owner rule, 500 for anything else. what is
// the action, worded to follow "cannot": "reissue a login link for".
func actOnRefusal( err error , what string ) ( refused *refusal ) {
	switch {
	case err == nil:
	case errors.Is( err , db.ErrNotFound ):
		refused = refuse( fiber.StatusNotFound , "no such user" )
	case errors.Is( err , models.ErrOwnerProtected ):
		refused = refuse( fiber.StatusForbidden , "nobody can " + what + " the owner account" )
	case errors.Is( err , models.ErrOwnerRequired ):
		refused = refuse( fiber.StatusForbidden , "only the owner can " + what + " an admin account" )
	default:
		refused = internalFailure( err )
	}
	return
}

