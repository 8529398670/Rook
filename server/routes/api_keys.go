package routes

import (
	errors "errors"
	strconv "strconv"
	time "time"

	fiber "github.com/gofiber/fiber/v3"

	apidoc "rook/server/apidoc"
	db "rook/server/db"
	logs "rook/server/logs"

	models "rook/server/models"
	security "rook/server/security"
)

// API keys are how anything that is not a browser calls this API: a script, a
// cron job, a CI pipeline, another service. A key carries a role from the same
// set users do, so there is one permission model rather than two -- see
// server/models/api_key.go and references/architecture.md.
//
// Every signed-in user can mint their own keys. An admin can additionally see
// and revoke everyone's, which is what makes "who has standing access to this
// app" an answerable question.
//
// Minting and revoking require a browser session (the group in routes.go adds
// Guard.RequireSession), so a leaked key cannot quietly mint its replacement.

type createAPIKeyRequest struct {
	Name string `json:"name,omitempty"`
	Role string `json:"role,omitempty"`
	// ExpiresInDays is a pointer so that three cases stay distinguishable:
	// absent means "use the configured default", 0 means "never expires", and
	// a number means that many days. With a plain int, "never" and "unset"
	// would be the same request.
	ExpiresInDays *int   `json:"expires_in_days,omitempty"`
	CSRFToken     string `json:"csrf_token,omitempty"`
}

// RequireAPIKeys hides the key-management endpoints when the feature is
// switched off, so an app with no programmatic callers can close the surface
// entirely. The Authorization header is ignored in that state too -- that part
// is enforced in server/security, not here, because it has to hold for every
// route rather than only these.
func ( handlers *Handlers ) RequireAPIKeys( c fiber.Ctx ) ( err error ) {
	if handlers.Config.APIKeysEnabled == false {
		err = notFound( c , "api keys are disabled on this server" )
		return
	}
	err = c.Next()
	return
}

// apiKeyRow is the only shape a key is ever returned in. The secret hash is
// not in it, and nor is the credential -- that exists in one response, from
// CreateAPIKey, and is never recoverable afterwards.
func apiKeyRow( key *models.APIKey , display_name string ) ( row fiber.Map ) {
	row = fiber.Map{
		"id":         key.ID,
		"user_id":    key.UserID,
		"name":       key.Name,
		"role":       key.Role,
		"created_at": key.CreatedAt,
		"revoked":    key.Revoked(),
		"expired":    key.Expired(),
		"live":       key.Live(),
		// may_sign_in: this key may make its own account a login link
		// (POST /api/account/login-link). Its account switches it, from a
		// browser session.
		"may_sign_in": key.MaySignIn,
		// may_mint_keys: this key may make keys for other accounts (portal
		// contract v2, key minting). minted_by_key: the key that made this
		// one, or empty.
		"may_mint_keys": key.MayMintKeys,
		"minted_by_key": key.MintedByKey,
	}
	if key.ExpiresAt != nil {
		row[ "expires_at" ] = key.ExpiresAt
	}
	if key.LastUsedAt != nil {
		row[ "last_used_at" ] = key.LastUsedAt
	}
	if key.RevokedAt != nil {
		row[ "revoked_at" ] = key.RevokedAt
	}
	if display_name != "" {
		row[ "display_name" ] = display_name
	}
	return
}

// ListMyAPIKeys answers "what has standing access as me". Revoked and expired
// keys are included, marked as such, because a key that used to exist is part
// of that answer.
//
// @route("GET /api/keys", summary="List my API keys", tag="API keys", feature="api_keys", access=SESSION_ONLY,
// )
func ( handlers *Handlers ) ListMyAPIKeys( c fiber.Ctx , in *apidoc.None ) ( err error ) {
	user := security.UserFrom( c )
	keys , list_err := models.ListAPIKeysForUser( handlers.Store , user.ID )
	if list_err != nil {
		err = serverError( c )
		return
	}
	rows := []fiber.Map{}
	for _ , key := range keys {
		rows = append( rows , apiKeyRow( key , "" ) )
	}
	err = c.JSON( rows )
	return
}

// CreateMyAPIKey mints a key for the signed-in user and returns the credential
// once. The default role is "user" even for an admin: an admin-scoped key can
// list and disable accounts and revoke anyone's access, so it should be asked
// for rather than handed out by omission.
//
// The ceiling that a key cannot outrank its owner lives in
// models.IssueAPIKey, not here, so the control socket and the CLI get it too.
//
// @route("POST /api/keys", summary="Create my API key", tag="API keys", feature="api_keys", access=SESSION_ONLY,
// )
func ( handlers *Handlers ) CreateMyAPIKey( c fiber.Ctx , in *createMyAPIKeyIn ) ( err error ) {
	body := in.createAPIKeyRequest
	if models.ValidAPIKeyName( body.Name ) == false {
		err = badRequest( c , "name must be 1-80 characters" )
		return
	}

	role := body.Role
	if role == "" {
		role = models.RoleUser
	}
	if models.ValidRole( role ) == false {
		err = badRequest( c , "role must be admin, user or viewer" )
		return
	}

	ttl , ok := models.APIKeyTTLFromDays( body.ExpiresInDays , handlers.Config.APIKeyTTL )
	if ok == false {
		err = badRequest( c , "expires_in_days must be 0 (never) or up to 3650" )
		return
	}

	user := security.UserFrom( c )
	credential , key , issue_err := models.IssueAPIKey( handlers.Store , user.ID , body.Name , role , ttl )
	if issue_err != nil {
		switch {
		case errors.Is( issue_err , models.ErrAPIKeyRole ):
			err = forbidden( c , "you cannot create an API key with more permission than your own account" )
		case errors.Is( issue_err , models.ErrAPIKeyLimit ):
			err = badRequest( c , "you already have the maximum number of live API keys -- revoke one first" )
		case errors.Is( issue_err , models.ErrAPIKeyDisabled ):
			// Not reachable through a live session today, since a disabled
			// account cannot hold one. Mapped anyway so that a future path
			// into this handler fails as a refusal rather than a 500.
			err = forbidden( c , "this account is disabled" )
		default:
			err = serverError( c )
		}
		return
	}

	// The credential appears here and nowhere else, ever.
	row := apiKeyRow( key , "" )
	row[ "credential" ] = credential
	err = c.Status( fiber.StatusCreated ).JSON( row )
	return
}

// RevokeMyAPIKey takes effect immediately -- the next request carrying that
// key fails. Someone else's key is reported as "no such api key" rather than
// "forbidden", so the endpoint does not confirm which key ids exist.
//
// @route("POST /api/keys/{key_id}/revoke", summary="Revoke my API key", tag="API keys", feature="api_keys", access=SESSION_ONLY,
// )
func ( handlers *Handlers ) RevokeMyAPIKey( c fiber.Ctx , in *revokeMyAPIKeyIn ) ( err error ) {

	user := security.UserFrom( c )
	key , get_err := models.GetAPIKey( handlers.Store , c.Params( "key_id" ) )
	if get_err != nil || key.UserID != user.ID {
		err = notFound( c , "no such api key" )
		return
	}
	if models.RevokeAPIKey( handlers.Store , key.ID ) != nil {
		err = serverError( c )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}

// SetMyAPIKeySignIn is the Sign-in links switch on one of your own keys: on,
// the key may make your account a one-time login link that works for at most
// 15 minutes (POST /api/account/login-link). Only the key's own account may
// flip it, from a browser session -- not an admin, not the owner, and never
// the key itself. Turning it off voids any link the key made that is still
// unused; browsers those links signed in stay signed in until the key is
// revoked.
//
// @route("POST /api/keys/{key_id}/sign-in", summary="Let my API key make sign-in links", tag="API keys", feature="api_keys", access=SESSION_ONLY,
//     description="may_sign_in true lets this key of yours ask POST /api/account/login-link for a "
//         "one-time link that works for at most 15 minutes; false stops it and voids any unused link it made.",
// )
func ( handlers *Handlers ) SetMyAPIKeySignIn( c fiber.Ctx , in *setMyAPIKeySignInIn ) ( err error ) {
	user := security.UserFrom( c )
	key , set_err := models.SetAPIKeySignIn( handlers.Store , in.KeyID , user.ID , in.MaySignIn )
	switch {
	case set_err == nil:
	case errors.Is( set_err , db.ErrNotFound ) , errors.Is( set_err , models.ErrAPIKeyNotYours ):
		// Someone else's key reads as missing, as in RevokeMyAPIKey.
		err = notFound( c , "no such api key" )
		return
	case errors.Is( set_err , models.ErrAPIKeyDead ):
		err = badRequest( c , "this key is revoked or expired" )
		return
	case errors.Is( set_err , models.ErrServiceAccount ):
		err = serviceAccountRefusal()
		return
	default:
		err = serverError( c )
		return
	}
	handlers.audit( c , logs.KindAuth , "api key sign-in links switched" , fiber.Map{ "key_id": key.ID , "may_sign_in": key.MaySignIn } )
	err = c.JSON( apiKeyRow( key , "" ) )
	return
}

// ListAllAPIKeys is the admin view: every key in the app, with whose it is.
// This is the "who and what has standing access" question, and it is the
// reason admins can see keys they did not create.
//
// @route("GET /api/admin/keys", summary="List all API keys", tag="Admin", feature="api_keys", role=ADMIN, access=SESSION_ONLY, use=[handlers.RequireOwner],
// )
func ( handlers *Handlers ) ListAllAPIKeys( c fiber.Ctx , in *apidoc.None ) ( err error ) {
	keys , list_err := models.ListAPIKeys( handlers.Store )
	if list_err != nil {
		err = serverError( c )
		return
	}
	users , users_err := models.ListUsers( handlers.Store )
	if users_err != nil {
		err = serverError( c )
		return
	}

	// One pass to build the lookup rather than a GetUser per key: a bucket
	// read per row is the shape that quietly becomes slow.
	names := map[ uint64 ]string{}
	for _ , user := range users {
		names[ user.ID ] = user.DisplayName
	}

	rows := []fiber.Map{}
	for _ , key := range keys {
		rows = append( rows , apiKeyRow( key , names[ key.UserID ] ) )
	}
	err = c.JSON( rows )
	return
}

// RevokeAnyAPIKey is how an admin kills a credential belonging to someone
// else -- a laptop that went missing, a CI system being decommissioned, a
// contractor who has finished. Every key it minted goes with it, down the
// chain (models.VoidLinksMintedByKeyTx).
//
// A key the owner allowed to make keys may call it too (key minting), for
// any key but the owner's; any other key gets 403 key_minting_off. Any admin
// may (portal contract v3, item 1), on the owner ladder for the key's
// account: nobody revokes the owner's keys this way, and only the owner
// revokes another admin's.
//
// @route("POST /api/admin/keys/{key_id}/revoke", summary="Revoke any API key", tag="Admin", feature="api_keys", role=ADMIN, use=[handlers.RequireAccountAdmin],
//     description="Stops the key at once, and every key it made, down the chain. Needs a browser session "
//         "or a key with may_mint_keys (any other key gets 403 key_minting_off), which may not revoke the owner's keys. "
//         "Follows the owner ladder: nobody revokes the owner's keys this way, and only the owner revokes another admin's.",
// )
func ( handlers *Handlers ) RevokeAnyAPIKey( c fiber.Ctx , in *revokeAnyAPIKeyIn ) ( err error ) {
	if handlers.keyMintingOff( c ) {
		err = apidoc.Refuse( c , fiber.StatusForbidden , "key_minting_off" , apidoc.CodeKeyMintingOff )
		return
	}
	key , get_err := models.GetAPIKey( handlers.Store , c.Params( "key_id" ) )
	if get_err != nil {
		err = notFound( c , "no such api key" )
		return
	}
	actor := security.ActorFrom( c )
	if actor.ViaAPIKey() && models.IsSuperAdmin( handlers.Store , key.UserID ) {
		err = apidoc.Refuse( c , fiber.StatusForbidden , "an API key cannot revoke the owner's keys" , apidoc.CodeOwnerProtected )
		return
	}
	if refused := actOnRefusal( models.MaySuperActOn( handlers.Store , actor.User.ID , key.UserID ) , "revoke the keys of" ); refused != nil {
		err = refused
		return
	}
	if models.RevokeAPIKey( handlers.Store , key.ID ) != nil {
		err = serverError( c )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}

// DeleteAnyAPIKey removes a key -- anyone's, live or dead -- for good, so it
// drops off the listings too rather than waiting for the purge loop.
//
// @route("POST /api/admin/keys/{key_id}/delete", summary="Delete any API key", tag="Admin", feature="api_keys", role=ADMIN, access=SESSION_ONLY, use=[handlers.RequireOwner],
// )
func ( handlers *Handlers ) DeleteAnyAPIKey( c fiber.Ctx , in *deleteAnyAPIKeyIn ) ( err error ) {

	key , get_err := models.GetAPIKey( handlers.Store , c.Params( "key_id" ) )
	if get_err != nil {
		err = notFound( c , "no such api key" )
		return
	}
	if models.DeleteAPIKey( handlers.Store , key.ID ) != nil {
		err = serverError( c )
		return
	}
	err = c.JSON( fiber.Map{ "ok": true } )
	return
}

// keyMintingOff is true for a request made with an API key that the owner
// has not allowed to make keys (APIKey.MayMintKeys). A browser session is
// never refused here: these routes stay the owner's from the page as before.
func ( handlers *Handlers ) keyMintingOff( c fiber.Ctx ) ( result bool ) {
	actor := security.ActorFrom( c )
	result = actor.ViaAPIKey() && actor.APIKey.MayMintKeys == false
	return
}

type setMyAPIKeyMintingIn struct {
	pathKeyID
	MayMintKeys bool `json:"may_mint_keys"`
}

// SetMyAPIKeyMinting is the Make keys switch (portal contract v2, key
// minting): on, this admin key may make keys for other accounts through
// POST /api/admin/users/{user_id}/keys, the way ProjectPortal hands its
// people keys. Only the owner, from a browser session, on one of the owner's
// own admin-role keys a person made, or on an admin key of a service account
// (ProjectPortal's, portal contract v3). Off leaves the keys it already made
// alone; revoking it ends those.
//
// @route("POST /api/keys/{key_id}/mint-keys", summary="Let my API key make keys", tag="API keys", feature="api_keys", access=SESSION_ONLY,
//     description="may_mint_keys true lets this admin key of the owner's make keys for other accounts "
//         "(POST /api/admin/users/{user_id}/keys); false stops it. Owner only, on the owner's own admin keys "
//         "or a service account's.",
// )
func ( handlers *Handlers ) SetMyAPIKeyMinting( c fiber.Ctx , in *setMyAPIKeyMintingIn ) ( err error ) {
	user := security.UserFrom( c )
	key , set_err := models.SetAPIKeyMinting( handlers.Store , in.KeyID , user.ID , in.MayMintKeys )
	switch {
	case set_err == nil:
	case errors.Is( set_err , db.ErrNotFound ) , errors.Is( set_err , models.ErrAPIKeyNotYours ):
		err = notFound( c , "no such api key" )
		return
	case errors.Is( set_err , models.ErrOwnerRequired ):
		err = apidoc.Refuse( c , fiber.StatusForbidden , "only the owner can let a key make keys" , apidoc.CodeOwnerRequired )
		return
	case errors.Is( set_err , models.ErrAPIKeyNotAdmin ) , errors.Is( set_err , models.ErrAPIKeyMintsNot ):
		err = badRequest( c , set_err.Error() )
		return
	case errors.Is( set_err , models.ErrAPIKeyDead ):
		err = badRequest( c , "this key is revoked or expired" )
		return
	default:
		err = serverError( c , set_err )
		return
	}
	handlers.audit( c , logs.KindAuth , "api key minting switched" , fiber.Map{ "key_id": key.ID , "may_mint_keys": key.MayMintKeys } )
	err = c.JSON( apiKeyRow( key , "" ) )
	return
}

type mintKeyRequest struct {
	Name          string `json:"name,omitempty"`
	Role          string `json:"role,omitempty"`
	ExpiresInDays int    `json:"expires_in_days,omitempty"`
}

type mintKeyForUserIn struct {
	pathUserID
	mintKeyRequest
}

type listUserAPIKeysIn struct {
	pathUserID
}

// mintTarget is the account in the path, for the two key-minting routes.
func ( handlers *Handlers ) mintTarget( c fiber.Ctx ) ( user *models.User , err error ) {
	user_id , parse_err := strconv.ParseUint( c.Params( "user_id" ) , 10 , 64 )
	if parse_err != nil {
		err = badRequest( c , "invalid user id" )
		return
	}
	if user , parse_err = models.GetUser( handlers.Store , user_id ); parse_err != nil {
		user = nil
		err = notFound( c , "no such user" )
	}
	return
}

// MintKeyForUser makes a key for an account (portal contract v2, key
// minting). Called with a key, that key needs may_mint_keys (403
// key_minting_off otherwise) and becomes the new key's minted_by_key, so
// revoking it revokes this one. The role is capped at the account's own; the
// owner's account never gets one this way. Any admin may, on the owner
// ladder (RequireAccountAdmin; portal contract v3, item 1).
//
// @route("POST /api/admin/users/{user_id}/keys", summary="Make a key for a user", tag="Admin", feature="api_keys", role=ADMIN, use=[handlers.RequireAccountAdmin], status=201,
//     description="name 1-80, role admin, user or viewer (capped at the account's role), expires_in_days "
//         "1-365 (required). Answers with the key row and its credential, shown once. A key calling needs "
//         "may_mint_keys (403 key_minting_off). Never for the owner's account.",
// )
func ( handlers *Handlers ) MintKeyForUser( c fiber.Ctx , in *mintKeyForUserIn ) ( err error ) {
	if handlers.keyMintingOff( c ) {
		err = apidoc.Refuse( c , fiber.StatusForbidden , "key_minting_off" , apidoc.CodeKeyMintingOff )
		return
	}
	body := in.mintKeyRequest
	actor := security.ActorFrom( c )
	if models.ValidAPIKeyName( body.Name ) == false {
		err = badRequest( c , "name must be 1-80 characters" )
		return
	}
	if models.ValidRole( body.Role ) == false {
		err = badRequest( c , "role must be admin, user or viewer" )
		return
	}
	if body.ExpiresInDays < 1 || body.ExpiresInDays > 365 {
		err = badRequest( c , "expires_in_days must be 1-365" )
		return
	}
	target , target_err := handlers.mintTarget( c )
	if target == nil {
		err = target_err
		return
	}
	ttl := time.Duration( body.ExpiresInDays ) * 24 * time.Hour
	credential , key , issue_err := models.MintAPIKey( handlers.Store , actor.User.ID , actor.KeyID() , target.ID , body.Name , body.Role , ttl )
	switch {
	case issue_err == nil:
	case errors.Is( issue_err , models.ErrOwnerProtected ):
		err = apidoc.Refuse( c , fiber.StatusForbidden , "the owner's account cannot be given a key this way" , apidoc.CodeOwnerProtected )
		return
	case errors.Is( issue_err , models.ErrOwnerRequired ):
		err = apidoc.Refuse( c , fiber.StatusForbidden , issue_err.Error() , apidoc.CodeOwnerRequired )
		return
	case errors.Is( issue_err , models.ErrAPIKeyLimit ):
		err = badRequest( c , "that account already has the maximum number of live API keys" )
		return
	case errors.Is( issue_err , models.ErrAPIKeyDisabled ):
		err = forbidden( c , "that account is disabled" )
		return
	default:
		err = serverError( c , issue_err )
		return
	}
	handlers.audit( c , logs.KindAdmin , "api key minted" , fiber.Map{ "key_id": key.ID , "user_id": target.ID , "minted_by_key": key.MintedByKey } )
	row := apiKeyRow( key , target.DisplayName )
	row[ "credential" ] = credential
	err = c.Status( fiber.StatusCreated ).JSON( row )
	return
}

// ListUserAPIKeys is one account's keys, for the portal's Manage screen
// (portal contract v2, key minting). A key calling needs may_mint_keys. Any
// admin may, on the owner ladder (RequireAccountAdmin; portal contract v3,
// item 1): nobody lists the owner's keys this way, and only the owner lists
// another admin's.
//
// @route("GET /api/admin/users/{user_id}/keys", summary="List a user's API keys", tag="Admin", feature="api_keys", role=ADMIN, use=[handlers.RequireAccountAdmin],
//     description="{keys: [key rows]}; each row has minted_by_key (the key that made it, or empty). "
//         "A key calling needs may_mint_keys (403 key_minting_off). Follows the owner ladder: "
//         "nobody lists the owner's keys this way, and only the owner lists another admin's.",
// )
func ( handlers *Handlers ) ListUserAPIKeys( c fiber.Ctx , in *listUserAPIKeysIn ) ( err error ) {
	if handlers.keyMintingOff( c ) {
		err = apidoc.Refuse( c , fiber.StatusForbidden , "key_minting_off" , apidoc.CodeKeyMintingOff )
		return
	}
	target , target_err := handlers.mintTarget( c )
	if target == nil {
		err = target_err
		return
	}
	keys , list_err := models.ListAPIKeysForUser( handlers.Store , target.ID )
	if list_err != nil {
		err = serverError( c , list_err )
		return
	}
	rows := []fiber.Map{}
	for _ , key := range keys {
		rows = append( rows , apiKeyRow( key , target.DisplayName ) )
	}
	err = c.JSON( fiber.Map{ "keys": rows } )
	return
}

type rotatedKeyReply struct {
	Credential string `json:"credential" jsonschema_description:"The key's new credential, shown in this response only. The old one stopped working."`
}

func ( rotatedKeyReply ) APIName() ( name string ) { name = "RotatedKey"; return }

// rotateSelf is a key giving itself a new secret (portal contract v3): an
// outside tool rotates its own key on a schedule, with nobody involved.
//
// @route("POST /api/keys/self/rotate", summary="Rotate the calling key",
//     tag="API keys", access=KEY_ONLY, feature="api_keys",
//     description="Gives the calling key a new secret and answers its new credential, shown only now. The old "
//         "secret stops working at once. The key's id, name, role, grants, lineage and expiry stay as they are, so "
//         "the keys and links it made keep working; rotating never extends the expiry. A viewer key may call it.",
// )
func ( handlers *Handlers ) rotateSelf( c fiber.Ctx , in *apidoc.None ) ( reply *rotatedKeyReply , err error ) {
	actor := security.ActorFrom( c )
	credential , key , rotate_err := models.RotateAPIKey( handlers.Store , actor.KeyID() )
	if rotate_err != nil {
		err = refuse( fiber.StatusBadRequest , rotate_err.Error() )
		return
	}
	handlers.audit( c , logs.KindAuth , "api key rotated" , fiber.Map{ "key_id": key.ID } )
	reply = &rotatedKeyReply{ Credential: credential }
	return
}

// rotateMyAPIKey is the owner rotating one of the owner's own keys from a
// browser session. Someone else's key is "no such api key", as everywhere.
//
// @route("POST /api/keys/{key_id}/rotate", summary="Rotate one of your keys",
//     tag="API keys", access=SESSION_ONLY, feature="api_keys", errors=(404,),
//     description="The owner only: gives one of the owner's own keys a new secret and answers its new credential, "
//         "shown only now. The old secret stops working at once; nothing else about the key changes.",
// )
func ( handlers *Handlers ) rotateMyAPIKey( c fiber.Ctx , in *pathKeyID ) ( reply *rotatedKeyReply , err error ) {
	user := security.UserFrom( c )
	key , get_err := models.GetAPIKey( handlers.Store , in.KeyID )
	if get_err != nil || key.UserID != user.ID {
		err = refuse( fiber.StatusNotFound , "no such api key" )
		return
	}
	if models.IsSuperAdmin( handlers.Store , user.ID ) == false {
		err = refuse( fiber.StatusForbidden , "only the owner rotates a key from here; any key rotates itself" ).coded( apidoc.CodeOwnerRequired )
		return
	}
	credential , _ , rotate_err := models.RotateAPIKey( handlers.Store , key.ID )
	if rotate_err != nil {
		err = refuse( fiber.StatusBadRequest , rotate_err.Error() )
		return
	}
	handlers.audit( c , logs.KindAuth , "api key rotated" , fiber.Map{ "key_id": key.ID } )
	reply = &rotatedKeyReply{ Credential: credential }
	return
}
