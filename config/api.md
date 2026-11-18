# Rook API

Every route this server answers, apart from the pages at /login/* and /join/*. Generated from the code by scripts/api_docs.sh: an app's own routes get their docs the same way, and a test fails until they do.

This file is generated: do not edit it. Change the docs next to the handlers and run `scripts/api_docs.sh`.

## Contents

- [Calling the API](#calling-the-api)
- [Errors](#errors)
- [Account](#account)
- [Sign-in](#sign-in)
- [API keys](#api-keys)
- [Admin](#admin)
- [Owner](#owner)
- [Monitoring](#monitoring)
- [Direct messages](#direct-messages)
- [Games](#games)
- [Global chat](#global-chat)
- [Join](#join)
- [Lobby](#lobby)
- [Sign-up](#sign-up)
- [Sign-up links](#sign-up-links)
- [Tables](#tables)
- [Watch](#watch)
- [Schemas](#schemas)

## Calling the API

| Way in | How |
|---|---|
| Browser session | A browser session. Its POSTs carry csrf_token in the body. The cookie's name is server.cookie_name, with the __Host- prefix while server.secure_cookies is on. |
| API key | An API key, as Authorization: Bearer <id>.<secret>. |

## Errors

An error reply is a JSON object whose `error` says what went wrong.

| Status | Meaning | Body |
|---|---|---|
| 400 | The request was refused as it stands. | [`Error`](#error) |
| 401 | Not signed in. | [`Error`](#error) |
| 403 | Not allowed, or a bad csrf_token. | [`Error`](#error) |
| 404 | No such thing, or this feature is switched off. | [`Error`](#error) |
| 409 | The record has moved on: an undo too late, or a stale edit. | [`Error`](#error) |
| 429 | Too many requests. | [`Error`](#error) |
| 503 | In maintenance mode. Retry-After says when to try again. | [`Maintenance`](#maintenance) |

## Account

| Method | Path | Summary | Who may call |
|---|---|---|---|
| POST | [`/api/account/auto-play-level`](#post-apiaccountauto-play-level) | Set auto play level | browser session or API key, anyone signed in |
| POST | [`/api/account/avatar`](#post-apiaccountavatar) | Set avatar | browser session or API key, anyone signed in |
| GET | [`/api/account/backgrounds`](#get-apiaccountbackgrounds) | Your table pictures | browser session or API key, anyone signed in |
| POST | [`/api/account/backgrounds`](#post-apiaccountbackgrounds) | Upload a table picture | browser session or API key, anyone signed in |
| POST | [`/api/account/backgrounds/{background_id}/delete`](#post-apiaccountbackgroundsbackground_iddelete) | Delete a table picture | browser session or API key, anyone signed in |
| POST | [`/api/account/bot-level`](#post-apiaccountbot-level) | Set bot level | browser session or API key, anyone signed in |
| POST | [`/api/account/delete`](#post-apiaccountdelete) | Delete my account | browser session only, anyone signed in |
| POST | [`/api/account/hint-level`](#post-apiaccounthint-level) | Set hint level | browser session or API key, anyone signed in |
| POST | [`/api/account/login-link`](#post-apiaccountlogin-link) | Create my login link | browser session only, anyone signed in |
| POST | [`/api/account/partner-bot-outbids`](#post-apiaccountpartner-bot-outbids) | Set partner bot outbids | browser session or API key, anyone signed in |
| POST | [`/api/account/pin`](#post-apiaccountpin) | Set pin | browser session or API key, anyone signed in |
| POST | [`/api/account/rename`](#post-apiaccountrename) | Rename account | browser session or API key, anyone signed in |
| GET | [`/api/account/sessions`](#get-apiaccountsessions) | Your signed-in devices | browser session or API key, anyone signed in |
| POST | [`/api/account/sessions/revoke-others`](#post-apiaccountsessionsrevoke-others) | Sign out every other device | browser session or API key, anyone signed in |
| POST | [`/api/account/sessions/{session_id}/label`](#post-apiaccountsessionssession_idlabel) | Name one of your devices | browser session or API key, anyone signed in |
| POST | [`/api/account/sessions/{session_id}/revoke`](#post-apiaccountsessionssession_idrevoke) | Sign out one of your devices | browser session or API key, anyone signed in |
| POST | [`/api/account/theme`](#post-apiaccounttheme) | Pick your theme | browser session or API key, anyone signed in |
| POST | [`/api/account/themes`](#post-apiaccountthemes) | Replace your own themes | browser session or API key, anyone signed in |
| GET | [`/api/backgrounds/{background_id}`](#get-apibackgroundsbackground_id) | A table picture | browser session or API key, anyone signed in |
| GET | [`/api/language`](#get-apilanguage) | The UI's text | anyone |
| POST | [`/api/logout`](#post-apilogout) | Logout | browser session or API key, anyone signed in |
| POST | [`/api/logs/events`](#post-apilogsevents) | Record what happened on the page | browser session or API key, anyone signed in |
| GET | [`/api/me`](#get-apime) | Get me | browser session or API key, anyone signed in |
| GET | [`/api/profile/options`](#get-apiprofileoptions) | Get profile options | browser session or API key, anyone signed in |
| GET | [`/api/team`](#get-apiteam) | List team | browser session or API key, anyone signed in |

### POST /api/account/auto-play-level

Set auto play level

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `level` | string | no |  |  |

**Reply 200**: Success

### POST /api/account/avatar

Set avatar

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `color` | string | no |  |  |
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `icon` | string | no |  |  |

**Reply 200**: Success

### GET /api/account/backgrounds

Your table pictures

The pictures you may put on the game table in a theme: your own, then the ones an admin shared for the default themes.

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 401, 429, 503 |

**Reply 200**: Success

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `backgrounds` | array of [`Background`](#background) | yes |  | Your own pictures, then the shared ones. |
| `limit` | integer | yes |  | How many of your own you may keep. |
| `max_side` | integer | yes |  | The long side, in pixels, a picture is shrunk to. |
| `shared_limit` | integer | yes |  | How many shared ones there may be. |

### POST /api/account/backgrounds

Upload a table picture

The body is the picture itself: JPEG, PNG or WebP, under the request size limit. It is stored as a JPEG at most 2560 pixels on its long side, with its metadata removed. Answers the Background; use its id as a theme's table image.

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `X-CSRF-Token` | header | string | no |  | From GET /api/me. Browser sessions only. |

**Request body**

Text, as `image/jpeg`.

**Reply 201**: Success

An empty object.

### POST /api/account/backgrounds/{background_id}/delete

Delete a table picture

Delete one of your own pictures (an admin may delete shared ones too). Your themes that used it, and for a shared one the default themes, go back to the look underneath.

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 404, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `background_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `ok` | boolean | yes |  |  |

### POST /api/account/bot-level

Set bot level

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `level` | string | no |  |  |

**Reply 200**: Success

### POST /api/account/delete

Delete my account

|  |  |
|---|---|
| Who may call | browser session only, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/account/hint-level

Set hint level

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `level` | string | no |  |  |

**Reply 200**: Success

### POST /api/account/login-link

Create my login link

|  |  |
|---|---|
| Who may call | browser session only, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/account/partner-bot-outbids

Set partner bot outbids

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `on` | boolean | no |  |  |

**Reply 200**: Success

### POST /api/account/pin

Set pin

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `pin` | string | no |  |  |

**Reply 200**: Success

### POST /api/account/rename

Rename account

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `display_name` | string | no |  |  |

**Reply 200**: Success

### GET /api/account/sessions

Your signed-in devices

Where your account is signed in, a page at a time: id, when it started and was last seen, the browser, and the name you gave it. Through an API key no device is current. next is the offset of the following page, or null on the last one.

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `limit` | query | integer | no | 1 to 1000; default `200` | Rows to return. Default 200. |
| `offset` | query | integer | no | at least 0 | Rows to skip: the next value of the previous page. |

**Reply 200**: Success

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `next` | integer or null | yes |  | Offset of the next page, or null on the last one. |
| `sessions` | array of [`Session`](#session) | yes |  |  |
| `total` | integer | yes |  |  |

### POST /api/account/sessions/revoke-others

Sign out every other device

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `ok` | boolean | yes |  |  |
| `removed` | integer | yes |  |  |

### POST /api/account/sessions/{session_id}/label

Name one of your devices

Give one of your devices a name (at most 80 characters; empty clears it). expect, when given, is the name you think it has now.

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 404, 409, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `session_id` | path | string | yes |  | The device's id, from GET /api/account/sessions or GET /api/admin/users/{user_id}/sessions (list_my_sessions or admin_list_user_sessions over MCP). |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `expect` | string or null | no |  | The name you think it has. Answers 409 if it has another. |
| `label` | string | yes | at most 80 characters | At most 80 characters. |

**Reply 200**: Success

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `label` | string | yes |  |  |
| `ok` | boolean | yes |  |  |

### POST /api/account/sessions/{session_id}/revoke

Sign out one of your devices

End one of your signed-in sessions; that browser has to sign in again.

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 404, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `session_id` | path | string | yes |  | The device's id, from GET /api/account/sessions or GET /api/admin/users/{user_id}/sessions (list_my_sessions or admin_list_user_sessions over MCP). |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `ok` | boolean | yes |  |  |

### POST /api/account/theme

Pick your theme

Switch to another theme from your list (or the defaults), by its id.

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `theme_id` | string | yes |  | A theme's id. Empty for the first in the list. |

**Reply 200**: Success

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `ok` | boolean | yes |  |  |
| `theme_id` | string | yes |  |  |

### POST /api/account/themes

Replace your own themes

Replace your whole list of color themes (1 to 40) and pick the one you use. themes: null goes back to following the app's defaults. A theme is the Theme schema in the API reference (config/api.json): id, name, mode light|dark, accent, second, and optional colors as #rrggbb.

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `builtins` | integer | no |  | Which built-in themes this list has seen: the highest since in Theme.builtIn (static/js/theme.js) when it was saved. Built-in themes added after that join the list the browser shows. Left out: 0, the first ones. |
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `theme_id` | string | no |  | The theme you use. |
| `themes` | array of [`Theme`](#theme) or null | no |  | Your whole list, 1 to 40 themes. null goes back to following the app's defaults. |

**Reply 200**: Success

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `ok` | boolean | yes |  |  |
| `theme_id` | string | yes |  |  |
| `themes` | array of [`Theme`](#theme) or null | yes |  | What you now see; null while the built-in themes apply. |
| `themes_builtins` | integer | yes |  | Which built-in themes that list has seen (see builtins on POST /api/account/themes). |
| `themes_custom` | boolean | yes |  |  |

### GET /api/backgrounds/{background_id}

A table picture

The picture's JPEG, for its owner or, for a shared one, anyone signed in.

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 404, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `background_id` | path | string | yes |  |  |

**Reply 200**: Success

Text, as `image/jpeg`.

### GET /api/language

The UI's text

|  |  |
|---|---|
| Who may call | anyone |
| Errors | 429, 503 |

**Reply 200**: Success

An empty object.

### POST /api/logout

Logout

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/logs/events

Record what happened on the page

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 404, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `events` | array of object | yes |  |  |
| `tab` | string | yes |  |  |

**Reply 200**: Success

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `accepted` | integer | yes |  |  |

### GET /api/me

Get me

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 401, 429, 503 |

**Reply 200**: Success

### GET /api/profile/options

Get profile options

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 401, 429, 503 |

**Reply 200**: Success

### GET /api/team

List team

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 401, 429, 503 |

**Reply 200**: Success

## Sign-in

| Method | Path | Summary | Who may call |
|---|---|---|---|
| POST | [`/api/login`](#post-apilogin) | Redeem login link | anyone |

### POST /api/login

Redeem login link

|  |  |
|---|---|
| Who may call | anyone |
| Errors | 400, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `credential` | string | no |  |  |

**Reply 200**: Success

## API keys

| Method | Path | Summary | Who may call |
|---|---|---|---|
| GET | [`/api/keys`](#get-apikeys) | List my API keys | browser session only, anyone signed in |
| POST | [`/api/keys`](#post-apikeys) | Create my API key | browser session only, anyone signed in |
| POST | [`/api/keys/{key_id}/revoke`](#post-apikeyskey_idrevoke) | Revoke my API key | browser session only, anyone signed in |

### GET /api/keys

List my API keys

|  |  |
|---|---|
| Who may call | browser session only, anyone signed in |
| Errors | 401, 403, 404, 429, 503 |

**Reply 200**: Success

### POST /api/keys

Create my API key

|  |  |
|---|---|
| Who may call | browser session only, anyone signed in |
| Errors | 400, 401, 403, 404, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `expires_in_days` | integer or null | no |  |  |
| `name` | string | no |  |  |
| `role` | string | no |  |  |

**Reply 200**: Success

### POST /api/keys/{key_id}/revoke

Revoke my API key

|  |  |
|---|---|
| Who may call | browser session only, anyone signed in |
| Errors | 400, 401, 403, 404, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `key_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

## Admin

| Method | Path | Summary | Who may call |
|---|---|---|---|
| POST | [`/api/admin/backgrounds`](#post-apiadminbackgrounds) | Upload a shared table picture | browser session or API key, admins |
| GET | [`/api/admin/keys`](#get-apiadminkeys) | List all API keys | browser session only, admins |
| POST | [`/api/admin/keys/{key_id}/delete`](#post-apiadminkeyskey_iddelete) | Delete any API key | browser session only, admins |
| POST | [`/api/admin/keys/{key_id}/revoke`](#post-apiadminkeyskey_idrevoke) | Revoke any API key | browser session only, admins |
| GET | [`/api/admin/settings`](#get-apiadminsettings) | Get settings | browser session or API key, admins |
| POST | [`/api/admin/settings`](#post-apiadminsettings) | Save settings | browser session or API key, admins |
| GET | [`/api/admin/theme-defaults`](#get-apiadmintheme-defaults) | Default themes | browser session or API key, admins |
| POST | [`/api/admin/theme-defaults`](#post-apiadmintheme-defaults) | Replace the default themes | browser session or API key, admins |
| GET | [`/api/admin/users`](#get-apiadminusers) | List users | browser session or API key, admins |
| POST | [`/api/admin/users`](#post-apiadminusers) | Create user | browser session only, admins |
| POST | [`/api/admin/users/{user_id}/avatar`](#post-apiadminusersuser_idavatar) | Admin set avatar | browser session or API key, admins |
| POST | [`/api/admin/users/{user_id}/can-hint`](#post-apiadminusersuser_idcan-hint) | Set user can hint | browser session or API key, admins |
| POST | [`/api/admin/users/{user_id}/can-invite`](#post-apiadminusersuser_idcan-invite) | Set user can invite | browser session or API key, admins |
| POST | [`/api/admin/users/{user_id}/can-reshuffle`](#post-apiadminusersuser_idcan-reshuffle) | Set user can reshuffle | browser session or API key, admins |
| POST | [`/api/admin/users/{user_id}/can-undo`](#post-apiadminusersuser_idcan-undo) | Set user can undo | browser session or API key, admins |
| POST | [`/api/admin/users/{user_id}/delete`](#post-apiadminusersuser_iddelete) | Delete user | browser session or API key, admins |
| POST | [`/api/admin/users/{user_id}/disabled`](#post-apiadminusersuser_iddisabled) | Set user disabled | browser session or API key, admins |
| POST | [`/api/admin/users/{user_id}/reissue-login`](#post-apiadminusersuser_idreissue-login) | Reissue login | browser session only, admins |
| POST | [`/api/admin/users/{user_id}/rename`](#post-apiadminusersuser_idrename) | Admin rename user | browser session or API key, admins |
| POST | [`/api/admin/users/{user_id}/revoke-access`](#post-apiadminusersuser_idrevoke-access) | Revoke access | browser session or API key, admins |
| POST | [`/api/admin/users/{user_id}/role`](#post-apiadminusersuser_idrole) | Admin set role | browser session or API key, admins |
| GET | [`/api/admin/users/{user_id}/sessions`](#get-apiadminusersuser_idsessions) | An account's signed-in devices | browser session or API key, admins |
| POST | [`/api/admin/users/{user_id}/sessions/{session_id}/revoke`](#post-apiadminusersuser_idsessionssession_idrevoke) | Sign out one of an account's devices | browser session or API key, admins |

### POST /api/admin/backgrounds

Upload a shared table picture

Like POST /api/account/backgrounds, but everyone signed in may see and use the picture: for the default themes.

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `X-CSRF-Token` | header | string | no |  | From GET /api/me. Browser sessions only. |

**Request body**

Text, as `image/jpeg`.

**Reply 201**: Success

An empty object.

### GET /api/admin/keys

List all API keys

|  |  |
|---|---|
| Who may call | browser session only, admins |
| Errors | 401, 403, 404, 429, 503 |

**Reply 200**: Success

### POST /api/admin/keys/{key_id}/delete

Delete any API key

|  |  |
|---|---|
| Who may call | browser session only, admins |
| Errors | 400, 401, 403, 404, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `key_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/admin/keys/{key_id}/revoke

Revoke any API key

|  |  |
|---|---|
| Who may call | browser session only, admins |
| Errors | 400, 401, 403, 404, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `key_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### GET /api/admin/settings

Get settings

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 401, 403, 429, 503 |

**Reply 200**: Success

### POST /api/admin/settings

Save settings

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `allow_join_signup` | boolean | yes |  |  |
| `bot_names` | string | no |  |  |
| `bot_names_top` | integer | no |  |  |
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `global_chat_off` | boolean | yes |  |  |
| `new_user_auto_play_level` | string | no |  |  |
| `new_user_bot_level` | string | no |  |  |
| `new_user_hint_level` | string | no |  |  |
| `new_user_no_hint` | boolean | yes |  |  |
| `new_user_no_invite` | boolean | yes |  |  |
| `new_user_no_led_glow` | boolean | yes |  |  |
| `new_user_no_player_scores` | boolean | yes |  |  |
| `new_user_no_reshuffle` | boolean | yes |  |  |
| `new_user_no_undo` | boolean | yes |  |  |
| `new_user_no_win_glow` | boolean | yes |  |  |
| `observers_see_all` | boolean | yes |  |  |
| `rook_anytime` | boolean | yes |  |  |
| `rook_first_trick_off` | boolean | yes |  |  |
| `rook_rank` | string | no |  |  |
| `table_chat_off` | boolean | yes |  |  |
| `trick_pause_ms` | integer | no |  |  |

**Reply 200**: Success

### GET /api/admin/theme-defaults

Default themes

The theme list everyone without their own list sees. themes is null while the built-in ones apply.

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 401, 403, 404, 429, 503 |

**Reply 200**: Success

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `builtins` | integer | yes |  | Which built-in themes that list has seen (see builtins on POST /api/account/themes). |
| `themes` | array of [`Theme`](#theme) or null | yes |  | null while the built-in themes apply. |

### POST /api/admin/theme-defaults

Replace the default themes

Replace the theme list everyone without their own list sees, 1 to 40 themes in the Theme schema of the API reference. null puts the built-in themes back.

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 400, 401, 403, 404, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `builtins` | integer | no |  | Which built-in themes this list has seen: the highest since in Theme.builtIn (static/js/theme.js) when it was saved. Built-in themes added after that join the list the browser shows. Left out: 0, the first ones. |
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `themes` | array of [`Theme`](#theme) or null | no |  | 1 to 40 themes. null goes back to the built-in ones. |

**Reply 200**: Success

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `builtins` | integer | yes |  | Which built-in themes that list has seen (see builtins on POST /api/account/themes). |
| `ok` | boolean | yes |  |  |
| `themes` | array of [`Theme`](#theme) or null | yes |  |  |

### GET /api/admin/users

List users

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 401, 403, 429, 503 |

**Reply 200**: Success

### POST /api/admin/users

Create user

|  |  |
|---|---|
| Who may call | browser session only, admins |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `display_name` | string | no |  |  |
| `role` | string | no |  |  |

**Reply 200**: Success

### POST /api/admin/users/{user_id}/avatar

Admin set avatar

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `color` | string | no |  |  |
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `icon` | string | no |  |  |

**Reply 200**: Success

### POST /api/admin/users/{user_id}/can-hint

Set user can hint

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `can_hint` | boolean | no |  |  |
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/admin/users/{user_id}/can-invite

Set user can invite

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `can_invite` | boolean | no |  |  |
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/admin/users/{user_id}/can-reshuffle

Set user can reshuffle

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `can_reshuffle` | boolean | no |  |  |
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/admin/users/{user_id}/can-undo

Set user can undo

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `can_undo` | boolean | no |  |  |
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/admin/users/{user_id}/delete

Delete user

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/admin/users/{user_id}/disabled

Set user disabled

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `disabled` | boolean | no |  |  |

**Reply 200**: Success

### POST /api/admin/users/{user_id}/reissue-login

Reissue login

|  |  |
|---|---|
| Who may call | browser session only, admins |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/admin/users/{user_id}/rename

Admin rename user

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `display_name` | string | no |  |  |

**Reply 200**: Success

### POST /api/admin/users/{user_id}/revoke-access

Revoke access

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/admin/users/{user_id}/role

Admin set role

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `role` | string | no |  |  |

**Reply 200**: Success

### GET /api/admin/users/{user_id}/sessions

An account's signed-in devices

Where an account is signed in: every session, with when it started and was last seen. Addresses are shown to the owner only, and only the owner sees the owner's.

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 400, 401, 403, 404, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | integer | yes | at least 1 | The account's id, from GET /api/admin/users. |
| `limit` | query | integer | no | 1 to 1000; default `200` | Rows to return. Default 200. |
| `offset` | query | integer | no | at least 0 | Rows to skip: the next value of the previous page. |

**Reply 200**: Success

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `next` | integer or null | yes |  | Offset of the next page, or null on the last one. |
| `sessions` | array of [`Session`](#session) | yes |  |  |
| `total` | integer | yes |  |  |

### POST /api/admin/users/{user_id}/sessions/{session_id}/revoke

Sign out one of an account's devices

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 400, 401, 403, 404, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | integer | yes | at least 1 | The account's id, from GET /api/admin/users. |
| `session_id` | path | string | yes |  | The device's id, from GET /api/account/sessions or GET /api/admin/users/{user_id}/sessions (list_my_sessions or admin_list_user_sessions over MCP). |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `ok` | boolean | yes |  |  |

## Owner

| Method | Path | Summary | Who may call |
|---|---|---|---|
| GET | [`/api/owner/logs/days`](#get-apiownerlogsdays) | The days on record | browser session only, the owner |
| GET | [`/api/owner/logs/days/{date}`](#get-apiownerlogsdaysdate) | One day's entries | browser session only, the owner |
| POST | [`/api/owner/logs/days/{date}/delete`](#post-apiownerlogsdaysdatedelete) | Delete one day | browser session only, the owner |
| GET | [`/api/owner/logs/days/{date}/download`](#get-apiownerlogsdaysdatedownload) | One day's entries as a file | browser session only, the owner |
| GET | [`/api/owner/logs/ip-names`](#get-apiownerlogsip-names) | What the owner calls addresses | browser session only, the owner |
| POST | [`/api/owner/logs/ip-names`](#post-apiownerlogsip-names) | Name an address or a range, or rename one | browser session only, the owner |
| POST | [`/api/owner/logs/ip-names/delete`](#post-apiownerlogsip-namesdelete) | Take a name off an address or a range | browser session only, the owner |
| POST | [`/api/owner/logs/prune`](#post-apiownerlogsprune) | Apply the retention rules now | browser session only, the owner |
| GET | [`/api/owner/logs/stream`](#get-apiownerlogsstream) | New log entries as they happen | browser session only, the owner |

### GET /api/owner/logs/days

The days on record

|  |  |
|---|---|
| Who may call | browser session only, the owner |
| Errors | 400, 404, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `q` | query | string | no |  | The Logs screen's search. |

**Reply 200**: Success

[`LogDays`](#logdays).

### GET /api/owner/logs/days/{date}

One day's entries

|  |  |
|---|---|
| Who may call | browser session only, the owner |
| Errors | 400, 404, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `date` | path | string | yes |  | The day's date, from GET /api/owner/logs/days. |
| `q` | query | string | no |  | The Logs screen's search. |
| `before` | query | integer or null | no | at least 0 |  |
| `limit` | query | integer | no | 1 to 1000; default `200` | Rows to return. Default 200. |

**Reply 200**: Success

[`LogDay`](#logday).

### POST /api/owner/logs/days/{date}/delete

Delete one day

|  |  |
|---|---|
| Who may call | browser session only, the owner |
| Errors | 400, 403, 404, 409, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `date` | path | string | yes |  | The day's date, from GET /api/owner/logs/days. |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `deleted` | boolean | yes |  |  |

### GET /api/owner/logs/days/{date}/download

One day's entries as a file

|  |  |
|---|---|
| Who may call | browser session only, the owner |
| Errors | 400, 404, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `date` | path | string | yes |  | The day's date, from GET /api/owner/logs/days. |

**Reply 200**: Success

Text, as `application/x-ndjson`.

### GET /api/owner/logs/ip-names

What the owner calls addresses

|  |  |
|---|---|
| Who may call | browser session only, the owner |
| Errors | 403, 404, 429, 503 |

**Reply 200**: Success

[`IPNames`](#ipnames).

### POST /api/owner/logs/ip-names

Name an address or a range, or rename one

|  |  |
|---|---|
| Who may call | browser session only, the owner |
| Errors | 400, 403, 404, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `name` | string | yes | 1 to 80 characters |  |
| `target` | string | yes | e.g. `10.0.0.0/24` | An address, or a range such as 10.0.0.0/24. |

**Reply 200**: Success

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `name` | string | yes |  |  |
| `target` | string | yes |  |  |

### POST /api/owner/logs/ip-names/delete

Take a name off an address or a range

|  |  |
|---|---|
| Who may call | browser session only, the owner |
| Errors | 400, 403, 404, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `target` | string | yes | e.g. `10.0.0.0/24` | An address, or a range such as 10.0.0.0/24. |

**Reply 200**: Success

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `deleted` | boolean | yes |  |  |

### POST /api/owner/logs/prune

Apply the retention rules now

|  |  |
|---|---|
| Who may call | browser session only, the owner |
| Errors | 400, 403, 404, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

[`PruneResult`](#pruneresult).

### GET /api/owner/logs/stream

New log entries as they happen

|  |  |
|---|---|
| Who may call | browser session only, the owner |
| Errors | 400, 404, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `q` | query | string | no |  | The Logs screen's search. |

**Reply 200**: Success

Text, as `text/event-stream`.

## Monitoring

| Method | Path | Summary | Who may call |
|---|---|---|---|
| GET | [`/api/api.json`](#get-apiapijson) | This document | browser session or API key, anyone signed in |
| GET | [`/api/api.md`](#get-apiapimd) | The API reference as Markdown tables for people | browser session or API key, anyone signed in |
| GET | [`/api/health`](#get-apihealth) | Is the process up | anyone |

### GET /api/api.json

This document

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 401, 404, 429, 503 |

**Reply 200**: Success

An empty object.

### GET /api/api.md

The API reference as Markdown tables for people

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 401, 404, 429, 503 |

**Reply 200**: Success

Text, as `text/markdown`.

### GET /api/health

Is the process up

|  |  |
|---|---|
| Who may call | anyone |
| Errors | 429, 503 |

**Reply 200**: Success

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `ok` | boolean | yes |  |  |

## Direct messages

| Method | Path | Summary | Who may call |
|---|---|---|---|
| POST | [`/api/dm/{user_id}`](#post-apidmuser_id) | Send direct message | browser session or API key, anyone signed in |
| POST | [`/api/dm/{user_id}/close`](#post-apidmuser_idclose) | Close direct message | browser session or API key, anyone signed in |
| POST | [`/api/dm/{user_id}/open`](#post-apidmuser_idopen) | Open direct message | browser session or API key, anyone signed in |
| POST | [`/api/dm/{user_id}/read`](#post-apidmuser_idread) | Mark direct messages read | browser session or API key, anyone signed in |
| POST | [`/api/dm/{user_id}/{seq}/delete`](#post-apidmuser_idseqdelete) | Delete direct message | browser session or API key, anyone signed in |
| POST | [`/api/dm/{user_id}/{seq}/edit`](#post-apidmuser_idseqedit) | Edit direct message | browser session or API key, anyone signed in |
| POST | [`/api/invite`](#post-apiinvite) | Invite player | browser session or API key, anyone signed in |

### POST /api/dm/{user_id}

Send direct message

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `text` | string | no |  |  |

**Reply 200**: Success

### POST /api/dm/{user_id}/close

Close direct message

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/dm/{user_id}/open

Open direct message

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/dm/{user_id}/read

Mark direct messages read

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `seq` | integer | no |  |  |

**Reply 200**: Success

### POST /api/dm/{user_id}/{seq}/delete

Delete direct message

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | string | yes |  |  |
| `seq` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/dm/{user_id}/{seq}/edit

Edit direct message

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | string | yes |  |  |
| `seq` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `text` | string | no |  |  |

**Reply 200**: Success

### POST /api/invite

Invite player

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `as` | string | no |  |  |
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `name` | string | no |  |  |
| `table_id` | string | no |  |  |
| `user_id` | integer | no |  |  |

**Reply 200**: Success

## Games

| Method | Path | Summary | Who may call |
|---|---|---|---|
| GET | [`/api/games`](#get-apigames) | List games | browser session or API key, anyone signed in |
| GET | [`/api/games/{game_id}`](#get-apigamesgame_id) | Get game | browser session or API key, anyone signed in |
| POST | [`/api/games/{game_id}/delete`](#post-apigamesgame_iddelete) | Delete game | browser session or API key, anyone signed in |

### GET /api/games

List games

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 401, 429, 503 |

**Reply 200**: Success

### GET /api/games/{game_id}

Get game

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `game_id` | path | string | yes |  |  |

**Reply 200**: Success

### POST /api/games/{game_id}/delete

Delete game

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `game_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

## Global chat

| Method | Path | Summary | Who may call |
|---|---|---|---|
| POST | [`/api/activity`](#post-apiactivity) | Report activity | browser session or API key, anyone signed in |
| GET | [`/api/global`](#get-apiglobal) | Get global | browser session or API key, anyone signed in |
| POST | [`/api/global/chat`](#post-apiglobalchat) | Global chat | browser session or API key, anyone signed in |
| POST | [`/api/global/chat/{seq}/delete`](#post-apiglobalchatseqdelete) | Delete global chat | browser session or API key, anyone signed in |
| POST | [`/api/global/chat/{seq}/edit`](#post-apiglobalchatseqedit) | Edit global chat | browser session or API key, anyone signed in |
| GET | [`/api/global/events`](#get-apiglobalevents) | Global events | browser session or API key, anyone signed in |
| GET | [`/api/players/{user_id}/presence`](#get-apiplayersuser_idpresence) | Player presence | browser session or API key, anyone signed in |

### POST /api/activity

Report activity

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### GET /api/global

Get global

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 401, 429, 503 |

**Reply 200**: Success

### POST /api/global/chat

Global chat

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `text` | string | no |  |  |

**Reply 200**: Success

### POST /api/global/chat/{seq}/delete

Delete global chat

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `seq` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/global/chat/{seq}/edit

Edit global chat

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `seq` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `text` | string | no |  |  |

**Reply 200**: Success

### GET /api/global/events

Global events

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 401, 429, 503 |

**Reply 200**: Success

### GET /api/players/{user_id}/presence

Player presence

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `user_id` | path | string | yes |  |  |

**Reply 200**: Success

## Join

| Method | Path | Summary | Who may call |
|---|---|---|---|
| GET | [`/api/join/{code}`](#get-apijoincode) | Get join | anyone |
| POST | [`/api/join/{code}`](#post-apijoincode) | Join table | browser session or API key, anyone signed in |
| POST | [`/api/join/{code}/signup`](#post-apijoincodesignup) | Join signup | anyone |

### GET /api/join/{code}

Get join

|  |  |
|---|---|
| Who may call | anyone |
| Errors | 400, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `code` | path | string | yes |  |  |

**Reply 200**: Success

### POST /api/join/{code}

Join table

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `code` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/join/{code}/signup

Join signup

|  |  |
|---|---|
| Who may call | anyone |
| Errors | 400, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `code` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `display_name` | string | no |  |  |

**Reply 200**: Success

## Lobby

| Method | Path | Summary | Who may call |
|---|---|---|---|
| GET | [`/api/lobby/events`](#get-apilobbyevents) | Lobby events | browser session or API key, anyone signed in |

### GET /api/lobby/events

Lobby events

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 401, 429, 503 |

**Reply 200**: Success

## Sign-up

| Method | Path | Summary | Who may call |
|---|---|---|---|
| POST | [`/api/signup/claim`](#post-apisignupclaim) | Signup claim | anyone |
| POST | [`/api/signup/create`](#post-apisignupcreate) | Signup create | anyone |
| POST | [`/api/signup/info`](#post-apisignupinfo) | Signup info | anyone |
| POST | [`/api/signup/match`](#post-apisignupmatch) | Signup match | anyone |

### POST /api/signup/claim

Signup claim

|  |  |
|---|---|
| Who may call | anyone |
| Errors | 400, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `credential` | string | no |  |  |
| `display_name` | string | no |  |  |
| `pin` | string | no |  |  |
| `user_id` | integer | no |  |  |

**Reply 200**: Success

### POST /api/signup/create

Signup create

|  |  |
|---|---|
| Who may call | anyone |
| Errors | 400, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `credential` | string | no |  |  |
| `display_name` | string | no |  |  |
| `pin` | string | no |  |  |
| `user_id` | integer | no |  |  |

**Reply 200**: Success

### POST /api/signup/info

Signup info

|  |  |
|---|---|
| Who may call | anyone |
| Errors | 400, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `credential` | string | no |  |  |
| `display_name` | string | no |  |  |
| `pin` | string | no |  |  |
| `user_id` | integer | no |  |  |

**Reply 200**: Success

### POST /api/signup/match

Signup match

|  |  |
|---|---|
| Who may call | anyone |
| Errors | 400, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `credential` | string | no |  |  |
| `display_name` | string | no |  |  |
| `pin` | string | no |  |  |
| `user_id` | integer | no |  |  |

**Reply 200**: Success

## Sign-up links

| Method | Path | Summary | Who may call |
|---|---|---|---|
| GET | [`/api/signup-links`](#get-apisignup-links) | List signup links | browser session only, anyone signed in |
| POST | [`/api/signup-links`](#post-apisignup-links) | Create signup link | browser session only, anyone signed in |
| POST | [`/api/signup-links/{link_id}/delete`](#post-apisignup-linkslink_iddelete) | Delete signup link | browser session only, anyone signed in |
| POST | [`/api/signup-links/{link_id}/revoke`](#post-apisignup-linkslink_idrevoke) | Revoke signup link | browser session only, anyone signed in |

### GET /api/signup-links

List signup links

|  |  |
|---|---|
| Who may call | browser session only, anyone signed in |
| Errors | 401, 403, 429, 503 |

**Reply 200**: Success

### POST /api/signup-links

Create signup link

|  |  |
|---|---|
| Who may call | browser session only, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `days` | integer | no |  |  |
| `label` | string | no |  |  |
| `max_uses` | integer | no |  |  |

**Reply 200**: Success

### POST /api/signup-links/{link_id}/delete

Delete signup link

|  |  |
|---|---|
| Who may call | browser session only, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `link_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/signup-links/{link_id}/revoke

Revoke signup link

|  |  |
|---|---|
| Who may call | browser session only, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `link_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

## Tables

| Method | Path | Summary | Who may call |
|---|---|---|---|
| GET | [`/api/tables`](#get-apitables) | List tables | browser session or API key, anyone signed in |
| POST | [`/api/tables`](#post-apitables) | Create table | browser session or API key, anyone signed in |
| GET | [`/api/tables/{table_id}`](#get-apitablestable_id) | Get table | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/assign`](#post-apitablestable_idassign) | Assign seat | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/auto-play`](#post-apitablestable_idauto-play) | Set auto play | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/bid`](#post-apitablestable_idbid) | Bid | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/bot`](#post-apitablestable_idbot) | Add bot | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/bot/level`](#post-apitablestable_idbotlevel) | Set seat bot level | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/bot/remove`](#post-apitablestable_idbotremove) | Remove bot | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/bury`](#post-apitablestable_idbury) | Bury | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/chat`](#post-apitablestable_idchat) | Chat at table | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/chat/{seq}/delete`](#post-apitablestable_idchatseqdelete) | Delete chat at table | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/chat/{seq}/edit`](#post-apitablestable_idchatseqedit) | Edit chat at table | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/continue`](#post-apitablestable_idcontinue) | Continue hand | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/delete`](#post-apitablestable_iddelete) | Delete table | browser session or API key, anyone signed in |
| GET | [`/api/tables/{table_id}/events`](#get-apitablestable_idevents) | Table events | browser session or API key, anyone signed in |
| GET | [`/api/tables/{table_id}/hint`](#get-apitablestable_idhint) | Get hint | browser session or API key, anyone signed in |
| GET | [`/api/tables/{table_id}/history`](#get-apitablestable_idhistory) | Get history | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/invite`](#post-apitablestable_idinvite) | Invite to seat | browser session only, anyone signed in |
| POST | [`/api/tables/{table_id}/leave`](#post-apitablestable_idleave) | Leave table | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/observers`](#post-apitablestable_idobservers) | Set table observers | browser session or API key, admins |
| POST | [`/api/tables/{table_id}/play`](#post-apitablestable_idplay) | Play card | browser session or API key, anyone signed in |
| GET | [`/api/tables/{table_id}/record`](#get-apitablestable_idrecord) | Get record | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/redo`](#post-apitablestable_idredo) | Redo | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/reset`](#post-apitablestable_idreset) | Reset to | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/reshuffle`](#post-apitablestable_idreshuffle) | Reshuffle | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/restart`](#post-apitablestable_idrestart) | Restart table | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/resume`](#post-apitablestable_idresume) | Resume | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/seat/look`](#post-apitablestable_idseatlook) | Set seat bot look | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/seat/remove`](#post-apitablestable_idseatremove) | Remove player | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/seat/rename`](#post-apitablestable_idseatrename) | Rename seat | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/seat/shuffle`](#post-apitablestable_idseatshuffle) | Shuffle seat bot | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/seat/unshuffle`](#post-apitablestable_idseatunshuffle) | Unshuffle seat bot | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/sit`](#post-apitablestable_idsit) | Sit at table | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/start`](#post-apitablestable_idstart) | Start table | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/takeover`](#post-apitablestable_idtakeover) | Request takeover | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/takeover/answer`](#post-apitablestable_idtakeoveranswer) | Answer takeover | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/takeover/cancel`](#post-apitablestable_idtakeovercancel) | Cancel takeover | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/trick-pause`](#post-apitablestable_idtrick-pause) | Set table trick pause | browser session or API key, anyone signed in |
| POST | [`/api/tables/{table_id}/undo`](#post-apitablestable_idundo) | Undo | browser session or API key, anyone signed in |

### GET /api/tables

List tables

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 401, 429, 503 |

**Reply 200**: Success

### POST /api/tables

Create table

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `name` | string | no |  |  |
| `options` | object | no |  |  |
| `options.play_to` | integer or null | no |  |  |
| `options.rook_anytime` | boolean | no |  |  |
| `options.rook_first_trick` | boolean | no |  |  |
| `options.rook_rank` | string | no |  |  |

**Reply 200**: Success

### GET /api/tables/{table_id}

Get table

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/assign

Assign seat

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `seat` | integer | no |  |  |
| `user_id` | integer | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/auto-play

Set auto play

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `on` | boolean | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/bid

Bid

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `amount` | integer | no |  |  |
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/tables/{table_id}/bot

Add bot

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `level` | string | no |  |  |
| `seat` | integer | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/bot/level

Set seat bot level

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `level` | string | no |  |  |
| `seat` | integer | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/bot/remove

Remove bot

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `seat` | integer | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/bury

Bury

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `cards` | array of string | no |  |  |
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `trump` | string | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/chat

Chat at table

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `text` | string | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/chat/{seq}/delete

Delete chat at table

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |
| `seq` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/tables/{table_id}/chat/{seq}/edit

Edit chat at table

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |
| `seq` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `text` | string | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/continue

Continue hand

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/tables/{table_id}/delete

Delete table

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### GET /api/tables/{table_id}/events

Table events

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Reply 200**: Success

### GET /api/tables/{table_id}/hint

Get hint

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Reply 200**: Success

### GET /api/tables/{table_id}/history

Get history

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/invite

Invite to seat

|  |  |
|---|---|
| Who may call | browser session only, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `display_name` | string | no |  |  |
| `seat` | integer | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/leave

Leave table

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/tables/{table_id}/observers

Set table observers

|  |  |
|---|---|
| Who may call | browser session or API key, admins |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `see_all` | boolean | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/play

Play card

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `card` | string | no |  |  |
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### GET /api/tables/{table_id}/record

Get record

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/redo

Redo

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/tables/{table_id}/reset

Reset to

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `to` | integer | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/reshuffle

Reshuffle

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/tables/{table_id}/restart

Restart table

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/tables/{table_id}/resume

Resume

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/tables/{table_id}/seat/look

Set seat bot look

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `color` | string | no |  |  |
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `icon` | string | no |  |  |
| `seat` | integer | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/seat/remove

Remove player

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `seat` | integer | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/seat/rename

Rename seat

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `name` | string | no |  |  |
| `seat` | integer | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/seat/shuffle

Shuffle seat bot

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `seat` | integer | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/seat/unshuffle

Unshuffle seat bot

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `seat` | integer | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/sit

Sit at table

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `seat` | integer | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/start

Start table

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `dealer` | integer or null | no |  |  |
| `random_dealer` | boolean | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/takeover

Request takeover

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `seat` | integer | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/takeover/answer

Answer takeover

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `accept` | boolean | no |  |  |
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `user_id` | integer | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/takeover/cancel

Cancel takeover

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

### POST /api/tables/{table_id}/trick-pause

Set table trick pause

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |
| `ms` | integer | no |  |  |

**Reply 200**: Success

### POST /api/tables/{table_id}/undo

Undo

|  |  |
|---|---|
| Who may call | browser session or API key, anyone signed in |
| Errors | 400, 401, 403, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `table_id` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  | From GET /api/me. Browser sessions only. |

**Reply 200**: Success

## Watch

| Method | Path | Summary | Who may call |
|---|---|---|---|
| GET | [`/api/watch/{code}`](#get-apiwatchcode) | Get watch | anyone |
| POST | [`/api/watch/{code}/chat`](#post-apiwatchcodechat) | Watch chat | anyone |
| GET | [`/api/watch/{code}/events`](#get-apiwatchcodeevents) | Watch events | anyone |
| GET | [`/api/watch/{code}/record`](#get-apiwatchcoderecord) | Get watch record | anyone |

### GET /api/watch/{code}

Get watch

|  |  |
|---|---|
| Who may call | anyone |
| Errors | 400, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `code` | path | string | yes |  |  |

**Reply 200**: Success

### POST /api/watch/{code}/chat

Watch chat

|  |  |
|---|---|
| Who may call | anyone |
| Errors | 400, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `code` | path | string | yes |  |  |

**Request body**

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `csrf_token` | string | no |  |  |
| `name` | string | no |  |  |
| `text` | string | no |  |  |

**Reply 200**: Success

### GET /api/watch/{code}/events

Watch events

|  |  |
|---|---|
| Who may call | anyone |
| Errors | 400, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `code` | path | string | yes |  |  |

**Reply 200**: Success

### GET /api/watch/{code}/record

Get watch record

|  |  |
|---|---|
| Who may call | anyone |
| Errors | 400, 429, 503 |

**Parameters**

| Name | In | Type | Required | Limits | Description |
|---|---|---|---|---|---|
| `code` | path | string | yes |  |  |

**Reply 200**: Success

## Schemas

The shapes the routes above refer to by name.

### Background

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `created_at` | string (date-time) | yes |  |  |
| `height` | integer | yes |  |  |
| `id` | string | yes |  |  |
| `mine` | boolean | yes |  | You uploaded it, so you may delete it. |
| `shared` | boolean | yes |  | Uploaded by an admin for the default themes: everyone may use it. |
| `size` | integer | yes |  | Bytes. |
| `width` | integer | yes |  |  |

### Error

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `error` | string | yes |  |  |
| `request_id` | string | no |  | On a 5xx: the reference that finds this failure in the log. |

### IPNames

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `names` | array of object | yes |  |  |
| `names[].created_at` | string (date-time) | yes |  |  |
| `names[].name` | string | yes |  |  |
| `names[].target` | string | yes |  |  |

### LogDay

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `date` | string | yes |  |  |
| `entries` | array of object | yes |  |  |
| `next` | integer or null | yes |  |  |

### LogDays

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `days` | array of object | yes |  |  |
| `dropped` | integer | yes |  |  |
| `settings` | object | yes |  |  |
| `total_bytes` | integer | yes |  |  |

### Maintenance

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `error` | string | yes |  |  |
| `maintenance` | string | yes | one of `read_only`, `full` |  |

### PruneResult

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `bytes_after` | integer | yes |  |  |
| `bytes_before` | integer | yes |  |  |
| `compressed` | integer | yes |  |  |
| `deleted` | integer | yes |  |  |

### Session

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `created_at` | string (date-time) | yes |  |  |
| `current` | boolean | yes |  |  |
| `expires_at` | string (date-time) | yes |  |  |
| `id` | string | yes |  |  |
| `ip` | string | no |  |  |
| `ip_name` | string | no |  | What the owner calls this address on the Logs screen. The owner's own list only. |
| `label` | string | no |  | The name you gave this device. |
| `last_seen_at` | string (date-time) | no |  |  |
| `user_agent` | string | no |  |  |

### Theme

| Field | Type | Required | Limits | Description |
|---|---|---|---|---|
| `accent` | string | yes |  | The main color: what you can press. |
| `background` | string | no |  | The page. Left out: the mode's own. |
| `button` | object or null | no |  | How the theme's buttons look. A choice left out is the default (rounded, normal, raised, normal). |
| `button.case` | string | no | one of `normal`, `upper` |  |
| `button.depth` | string | no | one of `flat`, `raised`, `glossy` |  |
| `button.shape` | string | no | one of `square`, `soft`, `rounded`, `pill` |  |
| `button.weight` | string | no | one of `normal`, `medium`, `bold` |  |
| `dark` | object or null | no |  | A light theme's colors for dark mode, kept while it is light. Switching the theme to dark swaps them in. A light theme's light (or a dark theme's dark) is dropped. |
| `dark.accent` | string | yes |  | The main color: what you can press. |
| `dark.background` | string | no |  | The page. Left out: the mode's own. |
| `dark.overrides` | map of string | no |  | Worked-out colors replaced by name: text, accent, accentFg, accentText, second, secondFg, secondText, link, navLink, background, surface, nested, chrome, info, infoBorder, border, ledRing (the card that led the trick), winRing (the card that won it), the game screen's table (the felt), tableText, tablePanel (seat names, the auction), tablePanelText, tableBar (the bar across the top), tableBarText, tray (the tray and hand along the bottom), trayText, us (your team) and them, and for each button variant V (Secondary, Success, Danger, Warning, Info, Light, Dark) btnV, btnVFg and btnVText. Leaving one out puts the readable color back. |
| `dark.second` | string | yes |  | Says where you are: active nav item, headings, info. |
| `dark.surface` | string | no |  | Cards. Left out: the mode's own. |
| `id` | string | yes |  | Lower-case letters, digits, - and _, up to 32. |
| `light` | object or null | no |  | A dark theme's colors for light mode, kept while it is dark. Switching the theme to light swaps them in. |
| `light.accent` | string | yes |  | The main color: what you can press. |
| `light.background` | string | no |  | The page. Left out: the mode's own. |
| `light.overrides` | map of string | no |  | Worked-out colors replaced by name: text, accent, accentFg, accentText, second, secondFg, secondText, link, navLink, background, surface, nested, chrome, info, infoBorder, border, ledRing (the card that led the trick), winRing (the card that won it), the game screen's table (the felt), tableText, tablePanel (seat names, the auction), tablePanelText, tableBar (the bar across the top), tableBarText, tray (the tray and hand along the bottom), trayText, us (your team) and them, and for each button variant V (Secondary, Success, Danger, Warning, Info, Light, Dark) btnV, btnVFg and btnVText. Leaving one out puts the readable color back. |
| `light.second` | string | yes |  | Says where you are: active nav item, headings, info. |
| `light.surface` | string | no |  | Cards. Left out: the mode's own. |
| `mode` | string | yes | one of `light`, `dark` |  |
| `name` | string | yes |  | 1 to 60 characters. |
| `overrides` | map of string | no |  | Worked-out colors replaced by name: text, accent, accentFg, accentText, second, secondFg, secondText, link, navLink, background, surface, nested, chrome, info, infoBorder, border, ledRing (the card that led the trick), winRing (the card that won it), the game screen's table (the felt), tableText, tablePanel (seat names, the auction), tablePanelText, tableBar (the bar across the top), tableBarText, tray (the tray and hand along the bottom), trayText, us (your team) and them, and for each button variant V (Secondary, Success, Danger, Warning, Info, Light, Dark) btnV, btnVFg and btnVText. Leaving one out puts the readable color back. |
| `second` | string | yes |  | Says where you are: active nav item, headings, info. |
| `surface` | string | no |  | Cards. Left out: the mode's own. |
| `table` | object or null | no |  | How the game table looks. Left out: light maple in light mode, green baize in dark. |
| `table.dim` | integer | no |  | How much the picture is darkened, 0 to 70 percent. |
| `table.image` | string | no |  | With look image: a picture's id from GET /api/account/backgrounds. |
| `table.look` | string | no |  | maple, baize, blue, red, walnut, slate, plain, or image for a picture. Left out: maple in light mode, baize in dark. |
| `table.x` | integer or null | no |  | The picture's focal point across, 0 to 100 percent: what stays in view when a screen crops it. Left out: 50. |
| `table.y` | integer or null | no |  | The picture's focal point down, 0 to 100 percent. Left out: 50. |
