package routes

import (
	json "encoding/json"

	fiber "github.com/gofiber/fiber/v3"
)

// The request types of the routes declared in this package that were mounted
// by hand before the go-webserver typed routes: a route's path values, then
// the JSON body its handler always read. Every body field is optional, as it
// was when the handler decoded the body itself.

type pathCode struct {
	Code string `json:"code" path:"code"`
}

type pathGameID struct {
	GameID string `json:"game_id" path:"game_id"`
}

type pathKeyID struct {
	KeyID string `json:"key_id" path:"key_id"`
}

type pathLinkID struct {
	LinkID string `json:"link_id" path:"link_id"`
}

type pathSeq struct {
	Seq string `json:"seq" path:"seq"`
}

type pathTableID struct {
	TableID string `json:"table_id" path:"table_id"`
}

type pathTableIDSeq struct {
	TableID string `json:"table_id" path:"table_id"`
	Seq string `json:"seq" path:"seq"`
}

type pathUserID struct {
	UserID string `json:"user_id" path:"user_id"`
}

type redeemLoginLinkIn struct {
	loginRequest
}

type getWatchIn struct {
	pathCode
}

type watchEventsIn struct {
	pathCode
}

type getWatchRecordIn struct {
	pathCode
}

type watchChatIn struct {
	pathCode
	watchChatRequest
}

type getJoinIn struct {
	pathCode
}

type joinSignupIn struct {
	pathCode
	joinSignupRequest
}

type signupInfoIn struct {
	signupRequest
}

type signupMatchIn struct {
	signupRequest
}

type signupClaimIn struct {
	signupRequest
}

type signupCreateIn struct {
	signupRequest
}

type renameAccountIn struct {
	renameRequest
}

type setAvatarIn struct {
	avatarRequest
}

type setPinIn struct {
	pinRequest
}

type setBotLevelIn struct {
	botLevelRequest
}

type setAutoPlayLevelIn struct {
	botLevelRequest
}

type setHintLevelIn struct {
	botLevelRequest
}

type setPartnerBotOutbidsIn struct {
	partnerBotOutbidsRequest
}

type joinTableIn struct {
	pathCode
}

type globalChatIn struct {
	globalChatRequest
}

type editGlobalChatIn struct {
	pathSeq
	globalChatRequest
}

type deleteGlobalChatIn struct {
	pathSeq
}

type createSignupLinkIn struct {
	signupLinkRequest
}

type revokeSignupLinkIn struct {
	pathLinkID
}

type deleteSignupLinkIn struct {
	pathLinkID
}

type createMyAPIKeyIn struct {
	createAPIKeyRequest
}

type revokeMyAPIKeyIn struct {
	pathKeyID
}

type createUserIn struct {
	createUserRequest
}

type reissueLoginIn struct {
	pathUserID
}

type setUserDisabledIn struct {
	pathUserID
	setDisabledRequest
}

type revokeAccessIn struct {
	pathUserID
	csrfOnlyRequest
}

type setUserCanInviteIn struct {
	pathUserID
	setCanInviteRequest
}

type setUserCanHintIn struct {
	pathUserID
	setCanHintRequest
}

type setUserCanUndoIn struct {
	pathUserID
	setCanUndoRequest
}

type setUserCanReshuffleIn struct {
	pathUserID
	setCanReshuffleRequest
}

type adminSetRoleIn struct {
	pathUserID
	setRoleRequest
}

type adminRenameUserIn struct {
	pathUserID
	adminRenameRequest
}

type adminSetAvatarIn struct {
	pathUserID
	avatarRequest
}

type deleteUserIn struct {
	pathUserID
	csrfOnlyRequest
}

type saveSettingsIn struct {
	settingsRequest
}

type revokeAnyAPIKeyIn struct {
	pathKeyID
}

type deleteAnyAPIKeyIn struct {
	pathKeyID
}

type createTableIn struct {
	createTableRequest
}

type getTableIn struct {
	pathTableID
}

type tableEventsIn struct {
	pathTableID
}

type sitAtTableIn struct {
	pathTableID
	seatRequest
}

type leaveTableIn struct {
	pathTableID
}

type addBotIn struct {
	pathTableID
	addBotRequest
}

type removeBotIn struct {
	pathTableID
	seatRequest
}

type setSeatBotLevelIn struct {
	pathTableID
	addBotRequest
}

type inviteToSeatIn struct {
	pathTableID
	inviteRequest
}

type assignSeatIn struct {
	pathTableID
	assignRequest
}

type removePlayerIn struct {
	pathTableID
	seatRequest
}

type renameSeatIn struct {
	pathTableID
	renameSeatRequest
}

type shuffleSeatBotIn struct {
	pathTableID
	seatRequest
}

type unshuffleSeatBotIn struct {
	pathTableID
	seatRequest
}

type setSeatBotLookIn struct {
	pathTableID
	botLookRequest
}

type startTableIn struct {
	pathTableID
	startRequest
}

type restartTableIn struct {
	pathTableID
}

type deleteTableIn struct {
	pathTableID
}

type bidIn struct {
	pathTableID
	bidRequest
}

type buryIn struct {
	pathTableID
	buryRequest
}

type playCardIn struct {
	pathTableID
	playRequest
}

type continueHandIn struct {
	pathTableID
}

type reshuffleIn struct {
	pathTableID
}

type setAutoPlayIn struct {
	pathTableID
	autoPlayRequest
}

type chatAtTableIn struct {
	pathTableID
	chatRequest
}

type editChatAtTableIn struct {
	pathTableIDSeq
	chatRequest
}

type deleteChatAtTableIn struct {
	pathTableIDSeq
}

type getHintIn struct {
	pathTableID
}

type getRecordIn struct {
	pathTableID
}

type requestTakeoverIn struct {
	pathTableID
	seatRequest
}

type cancelTakeoverIn struct {
	pathTableID
}

type answerTakeoverIn struct {
	pathTableID
	answerRequest
}

type getHistoryIn struct {
	pathTableID
}

type undoIn struct {
	pathTableID
}

type redoIn struct {
	pathTableID
}

type resetToIn struct {
	pathTableID
	resetRequest
}

type resumeIn struct {
	pathTableID
}

type setTableObserversIn struct {
	pathTableID
	observersRequest
}

type setTableTrickPauseIn struct {
	pathTableID
	trickPauseRequest
}

type getGameIn struct {
	pathGameID
}

type deleteGameIn struct {
	pathGameID
}

// csrfFromBody is the csrf_token of a JSON body. The typed routes take it out
// of the request before the handler sees it, and check it themselves on every
// signed-in route; a public route that checks it only for a signed-in caller
// reads it here.
func csrfFromBody( c fiber.Ctx ) ( token string ) {
	var body struct {
		CSRFToken string `json:"csrf_token"`
	}
	if json.Unmarshal( c.Body() , &body ) == nil { token = body.CSRFToken }
	return
}

// userIDPath is the account a route under /api/admin/users/{user_id} acts on
// (the template's sessions routes take it).
type userIDPath struct {
	UserID uint64 `json:"user_id" path:"user_id" jsonschema:"minimum=1" jsonschema_description:"The account's id, from GET /api/admin/users." error:"invalid user id"`
}

// okReply is the answer of a route that has nothing to say but yes.
type okReply struct {
	Ok bool `json:"ok"`
}
