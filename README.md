# Rook

The Rook card game, for four players in two teams, played in the browser. Any empty seat can be filled with a computer player at one of four strengths. The server is a single Go binary with the whole frontend compiled in. There's no database server to run and nothing to install on the clients.

- https://github.com/0187773933/Rook
- https://github.com/nfriend/Rook

## Contents

- [Features](#features)
- [Quick start](#quick-start)
- [Building and deploying](#building-and-deploying)
- [How the game is played](#how-the-game-is-played)
- [Computer players](#computer-players)
- [Hints and auto play](#hints-and-auto-play)
- [Shuffling and fairness](#shuffling-and-fairness)
- [Accounts and access](#accounts-and-access)
- [Configuration](#configuration)
- [Changing the wording](#changing-the-wording)
- [Project layout](#project-layout)
- [Development and tests](#development-and-tests)
- [HTTP API](#http-api)

## Features

**At the table**
- Bidding, the kitty and naming trump, trick play, scoring, and games played to 100, 300, 500, a custom target, or a single hand.
- Table options: where the Rook ranks, whether it may be played on any trick, and whether it is barred from the first trick.
- Cards you may not play are dimmed. The server decides what is legal, so the browser never has to know the rules.
- You can play by dragging, by tapping a card twice, or from the keyboard.

**Players and bots**
- Fill any seat with a computer player at one of six levels: Easy, Normal, Skilled, Hard, Expert or Master.
- Each player can set the level the "Add bot" picker starts on.
- A **hint button** (for players an admin allows) and **auto play** both use the strongest bot, Master.
- Someone watching can ask to take over a bot's seat, and the host decides.

**Around the table**
- Table chat, a global chat, a game log, the last trick, and the score sheet.
- Watch links for spectators.
- A history of finished games, with every hand and trick.
- Undo, redo and "step back to any move", for admins and for players allowed to.

**Accounts and the app**
- Accounts with no passwords: you sign in with one-time links. Sign-up links let people join, with an optional PIN on each account. API keys let scripts use the same API.
- Light theme by default, with an opt-in dark theme.
- Adjustable card sizes, and chat docks you can put beside your hand.
- Every sentence on screen comes from [`language.yaml`](language.yaml), including the Rules and Strategy guide.

## Quick start

You need Go 1.26 or newer.

```bash
./run.sh
```

`run.sh` builds this machine's binary into `dist/` and starts it. If a copy started earlier by `run.sh` is still running, it stops that copy first.

On the very first run there are no accounts yet. The server prints a **one-time login link** for the first admin:

```
ONE-TIME LOGIN LINK -- first-run admin "Admin"
  /login/<token>
```

Open `http://localhost:8080/login/<token>` and you are signed in as the owner. From there you can create a table, add bots, and invite people.

> Over plain `http://localhost`, set `secure_cookies: false` (or `SECURE_COOKIES=false`). Browsers silently drop a Secure cookie on http, so the login appears to succeed and then does nothing.

The server prints where it keeps its state on startup (the `[state]` line). By default that is `~/.config/rook/`, holding `app.db`, `secret.key` and an optional `config.yaml`. Back up `app.db` and `secret.key` together: the database is encrypted at rest, and losing the key makes it unreadable.

## Building and deploying

**Portable binaries.** `build.sh` cross-compiles self-contained executables into `dist/`. Each one has the frontend embedded, no libc dependency, and nothing else to install.

```bash
./build.sh
```

That builds every target. `./build.sh linux/arm64` builds just one.

**Docker.** `dockerBuild.sh` builds the image, and `dockerRun.sh` builds it and runs it hardened: a read-only filesystem, dropped capabilities, and state in `./data` next to the script.

```bash
./dockerRun.sh
```

**Account administration from the shell.** The same binary has a `manage` subcommand. It works whether the server is running (it talks to the server over a control socket) or stopped (it opens the database directly).

```bash
./dist/rook_darwin_arm64 manage list-users
```

The commands are:

| Command | What it does |
|---|---|
| `bootstrap-admin -name "Ada"` | create the first admin and a login link |
| `create-login -name "Grace" -role user` | create an account and a login link |
| `reissue-login -user-id 3` | a fresh login link for an existing account |
| `list-users` | id, name, role, status |
| `set-disabled -user-id 3 -disabled=true` | disable or re-enable an account |
| `revoke-access -user-id 3` | end every session, login link and API key |
| `list-keys`, `create-key`, `revoke-key` | manage API keys |
| `paths` | where the app keeps its state |

## How the game is played

The full rules and a strategy guide are in the app: **Rules & strategy** in the lobby, or `#/rules` and `#/strategy`. In short:

**The cards and points**
- 57 cards: red, black, yellow and green, each numbered 1 to 14, plus the Rook.
- The 1 is the highest card of every color.
- The point cards are the 1s (15), 14s (10), 10s (10), 5s (5) and the Rook (20). That makes 180 points in all.

**A hand**
1. Each player gets 13 cards, and 5 go face down as the **kitty**.
2. Players bid in steps of 5. A pass is final.
3. The winner of the bid picks up the kitty, buries any five cards, names trump, and leads the first trick.
4. The Rook belongs to the trump color.
5. You must follow the color led if you can. The highest trump wins a trick, or the highest card of the color led if no trump was played.
6. The buried cards go to whoever wins the **last trick**.

**Scoring**
- If the bidding team takes at least its bid, it scores everything it took.
- Otherwise it is **set**: it loses the amount of its bid.
- The defenders always score what they took.
- The game ends after the hand in which either team reaches the target.

**Table options**, chosen by whoever creates the table:
- **Rook rank:** highest (the default), 10½, or lowest.
- **Rook anytime:** the Rook may be played on any trick.
- **No Rook on trick 1.**
- **Play to:** the target score, or a single hand.

**Keyboard play:**
- Type a card (`r1`, `g10`, `y14`, or `R` for the Rook), then Enter.
- Tab and the arrow keys step through the cards you may play.
- To bury, type five cards, then `t` and a color letter to name trump (`tr` for red).
- Ctrl+A toggles auto play.

## Computer players

Any seated player can fill an empty seat with a bot by pressing **Add bot** and picking a level. The picker starts on your own default, which you set in **Settings → Bots you add**, so pressing **Enter** adds one at that level. The host or an admin can change a bot's level at any time by tapping its avatar. Mid-hand, the bot simply plays its next move at the new level.

### The six levels

| Level | How it plays |
|---|---|
| **Easy** | Like someone in their first few games. It bids timidly, wins tricks with its biggest card, throws whatever is lowest even when that gives points away, never helps its partner, and buries without a plan. About a third of its card choices are a random legal card. |
| **Normal** | The original bot. It uses simple rules: it wins a trick as cheaply as it can, throws points onto a trick its partner is winning, and otherwise plays its least valuable card. |
| **Skilled** | Plays the in-app strategy guide, rule by rule (see below), but does not count cards. It judges only from its own hand and the trick in front of it. |
| **Hard** | The same guide, plus card counting. |
| **Expert** | Hard, plus lookahead. For each card it plays it imagines **100 deals** of the cards it cannot see, plays every choice to the end of the hand, and picks the best average. |
| **Master** | Expert that **reads the table**. Its imagined deals favour the ones that explain the bids, the passes and the trump call. Hints and auto play use it. |

**What Skilled and Hard do (the strategy guide):**
- As the bidder, it draws trump while the other team might still hold some, keeping one trump back for the last trick (which carries the kitty).
- Defending, it never leads trump. It leads low cards from long colors to make the bidder use trump.
- It throws points onto a trick its partner is sure to win, never trumps its partner's trick, and buries to empty short colors.

**What Hard adds (card counting):**
- It remembers every card played, so it knows which card is the top one still out in each color, and when to stop drawing trump.
- It notices when someone fails to follow a color, which shows they are out of it.
- Together these tell it whether a trick is really safe for its side. Skilled, which forgets each trick once it is over, has to guess, so it sometimes throws points onto a trick that gets taken away, or keeps drawing trump after the other team has run out.

**How Expert decides:**
- **Imagined deals.** Each imagined deal is consistent with everything Expert has seen: how many cards each player holds, the colors each player has shown they are out of, and the buried kitty if Expert buried it.
- **Card play.** Every legal card is played forward on the same 100 deals, with all four seats playing Hard, and scored by the real scoring (made or set, the kitty, the last trick).
- **Bidding.** It compares raising against passing on 32 deals. Bidding on 100 deals measured no better, so it stays at the cheaper 32. When playing to a target, it also allows for the score: it is more careful when going set could cost the game, and bolder when the other team is about to win.
- **Naming trump.** It tries each reasonable trump color, with a bury for each, on 100 deals.
- **Speed.** The slowest decision, the opening lead with 13 legal cards, takes about 65 ms (Master about 75 ms). Bots wait `bot_delay_ms` (900 ms by default) before moving anyway, so the search never slows the game. It also runs on a copy of the game outside the table's lock, so nobody else at the table waits on it.

**How Master reads the table:**

Expert treats every deal of the unseen cards that fits the known voids as equally likely. Master, like a strong player, uses what the other players chose:
- Someone who bid up to 120 holds a strong hand, and someone who passed at 75 does not.
- The bidder named the color they were longest and strongest in.

For each decision it draws 16 times as many candidate deals as it needs. It scores each one by how well it explains every bid, pass and the trump call, sizing up each imagined hand the way the bots do for bidding. It then keeps 100 of them in proportion to that score.

For example, after the trump call, Master's imagined deals give the bidder about 4.5 trump on average, against about 3.3 in deals drawn blind (`TestMasterReadsTheTrumpCall`).

Trusting the evidence loosely worked better than trusting it strictly. People and bots bid for reasons a rough estimate misses, and a strict reading throws away deals that really happen.

To support this, the game now records the bid that stood when each player passed (`passed_at`).

### No bot cheats

Every level decides from what that seat could know at a real table: its own hand, the bids, trump, the cards played, and the kitty only if it buried it. No level reads another player's hand, and Expert's and Master's imagined deals are drawn only from the cards that seat has not seen. Master's reading uses only public facts: the bids, the passes and the trump call.

### How strong each level is

These numbers come from a **duplicate league** (`server/game/bot_league_test.go`). Every deal is played twice, with the two levels swapping seats the second time, so neither side is helped by the cards. The edge is the average points per hand the stronger level scored over the weaker one.

| Matchup | Edge per hand | Deals |
|---|---|---|
| Normal over Easy | +20 | 3000 |
| Skilled over Normal | +22 | 3000 |
| Hard over Skilled | +14 | 3000 |
| Expert over Hard | +26 | 300 |
| Master over Expert | +9 | 300 |

Over three separate sets of 300 deals, Master beat Expert by +9, +10 and +12.

Run it yourself (it takes a few minutes):

```bash
ROOK_BOT_LEAGUE=1 go test ./server/game -run BotLevelsAreInOrder -v
```

Edges move by about ±5 from one set of 300 deals to another (Expert over Hard has measured between +19 and +30 on different sets), so treat them as rough sizes, not exact figures. A smaller version runs with the normal tests and fails if any level stops beating the one below it. It plays too few deals to see Master's small edge over Expert through the noise, so there it checks Master against Hard instead.

### Where the code is

| File | What it holds |
|---|---|
| `server/game/bot_level.go` | the four levels |
| `server/game/bot.go` | Normal, and the per-level dispatch |
| `server/game/bot_easy.go` | Easy |
| `server/game/bot_memory.go` | what a seat has seen: unseen cards, colors each player is out of, top cards |
| `server/game/bot_hard.go` | Skilled and Hard |
| `server/game/bot_expert.go` | Expert's search |
| `server/game/bot_reading.go` | how Master reads the table |
| `server/tables/bots.go` | when bots move, and the hint |

## Hints and auto play

Both use **Master**, whatever level the bots at the table are set to.

**The hint button.** An admin switches it on per player in Settings → Players. It teaches before it plays, in three presses:
1. Say what the suggestion is ("Play the Green 14").
2. Show it: lift the card, set the bid, or pick the five cards and trump.
3. Do it.

A hint is worked out on the server from your own cards and what has been played, so it never knows more than you could. The same position always gets the same hint.

**Auto play** (⏩, or Ctrl+A) plays your cards for the rest of the current hand and turns itself off when the next hand is dealt.

## Shuffling and fairness

**The deals are truly random.**
- Every deal draws a fresh shuffle from Go's `math/rand/v2` generator. That is a cryptographic-strength generator (ChaCha8), seeded from the operating system's randomness when the server starts.
- No seed is chosen by anyone, and nothing is derived from the table, the time or the hand number.
- All tables share the one generator, so no two tables, hands or server restarts are related.
- The shuffle gives all 57! orders of the deck an equal chance.
- The "Random" first-dealer choice uses the same generator.

**The decks are kept on the server only.** Each deal's deck is saved in the table's move log. That is what lets undo, redo and "step back to any move" rebuild a position exactly. Redo after an undo therefore deals the same cards again, because it replays what happened. A new deal after undoing is a fresh shuffle.

Decks, hands and the buried kitty are never sent to a browser that should not see them. The server redacts every view, and the game record that players can see leaves them out.

**Bot reasoning is reproducible.** Expert's imagined deals and Easy's slips use randomness seeded from the position: the hand number, the seat, that seat's own cards and the cards played. So the same position always gets the same move or hint, and a recorded game replays the same way. This only affects what a bot chooses. It never touches the deal, and it reads no hidden cards.

The tests use fixed seeds so failures can be reproduced. That code never runs in the server.

## Accounts and access

**Signing in**
- There are no passwords. You sign in by opening a **one-time login link**, which then sets a session cookie (90 days by default).
- An admin can mint links in Settings → Players, or with `manage create-login`.
- **Sign-up links** (for players allowed to invite) let several people create accounts from one link, within a set number of uses and days.
- When someone signs in through a sign-up link and picks an existing account, that account's **PIN** (if it has one) is asked for.
- **Sign in on another device:** any player can mint a one-time link to themselves from Settings.

**Roles and permissions**
- The **owner** is the account the server was set up with. It always stays an admin and manages players and app-wide settings.
- Admins can see every hand while watching (everyone watching can too, if that is switched on for the table or the app), and step through any game's history.
- Per-player switches: **Can invite**, **Hints**, and **Undo** (step back and forth through moves at their own tables).

**API keys**
- A key carries a role from the same set as users, and never outranks the account that owns it.
- Send it as `Authorization: Bearer <key>`.
- Keys can be switched off completely with `api_keys: false`.

## Configuration

Everything has a working default, so nothing needs configuring.

- Put a `config.yaml` in the app directory to change settings (see [`config.example.yaml`](config.example.yaml) for every key with comments). It is **not** read from the project root.
- Every key also has an environment variable (`port` / `PORT`), and the environment always wins.

Commonly changed settings:

| Key | Default | Meaning |
|---|---|---|
| `port` / `host` | `8080` / `127.0.0.1` | where to listen (`0.0.0.0` for the network, with TLS in front) |
| `secure_cookies` | `true` | `false` only for plain `http://localhost` |
| `bot_delay_ms` | `900` | how long a bot "thinks" before moving |
| `table_retention_hours` | `72` | finished or idle tables are removed after this |
| `max_tables` | `50` | the most tables at once |
| `session_ttl_seconds` | 90 days | how long a sign-in lasts |
| `encrypt_at_rest` | `true` | encrypt every database value with `secret.key` |
| `api_keys` | `true` | allow API keys at all |
| `trust_proxy`, `trusted_proxies` | off | believe a reverse proxy's client-address header |

## Changing the wording

Every word on screen comes from [`language.yaml`](language.yaml). That covers buttons, errors, the bots' names and level descriptions, and the whole Rules & strategy guide. The server sends codes such as `not_your_turn`, never sentences, and the page looks each one up. Rewording the app is therefore a YAML edit.

To try changes without rebuilding, point the server at files on disk:
- With `LANGUAGE_FILE=language.yaml` and `STATIC_DIR=static`, the server reads those files from disk instead of the copies embedded in the binary.
- In Docker, mount your own `language.yaml` into `/app/data/`.

## Project layout

```
main.go, embed.go       entry point; the frontend is embedded with go:embed
cmd/manage/             the standalone account-administration tool
language.yaml           every string the app shows
static/                 the frontend: plain JS, no build step (js/, css/, index.html)
server/
  game/                 the rules engine: pure, no I/O. Dealing, bidding, kitty,
                        play, scoring, the per-seat views, and the bots
  tables/               live tables: seats, locks, timers, history, undo/redo,
                        chat, archiving, and when bots move
  routes/               the HTTP API (Fiber)
  models/               stored records (users, sessions, keys, tables, games...)
  db/, encryption/      the bolt database and encryption at rest
  security/, middleware/  sessions, CSRF, API keys, rate limiting
  config/, bootstrap/   configuration and first-run setup
  manage/, control/     the manage commands and the control socket they use
scripts/                vendoring, favicon and art helpers
old_php_version/        the original PHP game this one replaces
```

The rules live only on the server. A hand of cards is hidden information, so the browser is told what it may show and which cards are legal, and the server is the only judge of a move.

## Development and tests

```bash
go test ./server/...          # everything (about two minutes)
go test ./server/... -short   # quicker
```

Notable tests:
- **`TestBotGamesKeepEveryInvariant`** plays thousands of complete bot games across every table option and every mix of levels. After each move it checks that no card was created or lost, that only legal moves were accepted, and that every hand's points add up to 180.
- **`TestBotLevelsAreInOrder`** checks that each level beats the one below it. With `ROOK_BOT_LEAGUE=1` it is the full league above.
- **`BenchmarkExpertPlay`**, **`BenchmarkExpertBid`** and **`BenchmarkMasterPlay`** time the searching levels' decisions:

```bash
go test ./server/game -run XXX -bench 'Expert|Master'
```

## HTTP API

The browser uses the same JSON API that scripts can. Every request that changes something is a `POST` carrying the session's CSRF token, or an API key. Refusals come back as `{"error": "<code>"}`, and the codes are listed under `errors:` in `language.yaml`. The main groups:

| Path | What it covers |
|---|---|
| `/api/me`, `/api/account/...` | your account: name, avatar, PIN, default bot level, sign-in links |
| `/api/tables` | list and create tables |
| `/api/tables/:id/...` | everything at a table (see below) |
| `/api/tables/:id/events` | a live Server-Sent Events stream of the table |
| `/api/games` | finished games and their records |
| `/api/watch/:code` | read-only watching through a watch link |
| `/api/admin/...` | the owner's player and settings management |
| `/api/keys` | your API keys |

The actions under `/api/tables/:id/...` are:
- **Seats:** `sit`, `leave`, `bot` (with a `level`), `bot/remove`, `bot/level`, `seat/rename`.
- **The game:** `start`, `bid`, `bury`, `play`, `continue`, `auto-play`, `hint`.
- **Around it:** `chat`, `history`, `undo`, `redo`.
