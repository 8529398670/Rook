// The table screen. It owns the current snapshot and hands it to each part
// to draw: seats.js (the four badges), trick.js (the middle), hand.js (your
// cards), and the tray between them -- bidding.js, kitty.js, or a line of
// status. Snapshots arrive two ways, from the live stream and as the answer
// to your own actions, and both go through apply(), which keeps the newest.
//
// There is no game logic here. What is legal, whose turn it is, and what
// anything scored all come from the server in the snapshot.
//
// The same screen serves three kinds of visitor: a player (their own seat,
// their own hand), someone watching (any seat's view, every hand if allowed
// -- see observe.js), and an admin, who also gets the move history and
// undo/redo (history.js). A watch link (#/watch/<code>) opens it without an
// account.
const Table = {
  screenId: "screen-table",
  me: null,
  id: null,
  target: null,
  view: null,
  stream: null,
  inviteSeat: -1,
  // The host's pick for who deals a new table's first hand: a seat, "random",
  // or null until they choose.
  firstDealer: null,
  announced: 0,
  // What a watcher asked to see: whose seat, and every hand or not.
  perspective: 0,
  allHands: false,
  // Each player's points this hand under their team's score in the bar,
  // switched in Settings and the table menu. Chosen per device, so it lives
  // in localStorage (wrapped, like the chat dock: blocked storage means off).
  playerScoresKey: "rook.player_scores",
  playerScores: null,

  init( me ) {
    this.me = me;
    Seats.init( Dom.get( "felt" ) );
    Trick.init( Dom.get( "trick" ) );
    Hand.init( Dom.get( "hand" ) , {
      play: function ( card ) { return Table.act( Api.play( Table.id , card ) ); },
      buryChanged: function () { Kitty.refresh(); },
    } );
    Summary.init( {
      continue: function () {
        const game = Table.view && Table.view.game;
        if ( game && game.phase === "game_over" ) {
          Table.act( Api.start( Table.id ) );
        } else {
          Table.act( Api.continueHand( Table.id ) );
        }
      },
    } );
    // Someone watching through a watch link without signing in chats
    // through the link, under the name they typed.
    Chat.init( { send: function ( text , name ) {
      if ( Table.target && Table.target.watch && Table.view && !Table.view.signed_in ) {
        Table.act( Api.watchChat( Table.target.watch , name , text ) );
        return;
      }
      Table.act( Api.chat( Table.id , text ) );
    } } );

    Dom.get( "table-back" ).addEventListener( "click" , function () { App.go( "#/" ); } );
    Dom.get( "table-menu" ).addEventListener( "click" , this.openMenu.bind( this ) );
    Dom.get( "deal-button" ).addEventListener( "click" , function () {
      if ( Table.firstDealer === null ) return;
      Table.act( Api.start( Table.id , Table.firstDealer ) );
    } );
    Dom.get( "waiting-leave" ).addEventListener( "click" , this.confirmLeave.bind( this ) );
    Dom.get( "leave-confirm" ).addEventListener( "click" , this.leaveTable.bind( this ) );
    Dom.get( "manage-confirm" ).addEventListener( "click" , this.manageTable.bind( this ) );
    Dom.get( "sheet-invite" ).addEventListener( "submit" , this.onInvite.bind( this ) );
    Dom.get( "invite-copy" ).addEventListener( "click" , function () {
      Dom.copy( Dom.get( "invite-link" ) , Dom.get( "invite-copied" ) );
    } );
    Dom.get( "invite-share" ).addEventListener( "click" , this.shareInvite.bind( this ) );
    Dom.get( "invite-filter" ).addEventListener( "input" , this.renderInviteUsers.bind( this ) );
    Dom.get( "invite-filter" ).addEventListener( "keydown" , function ( event ) {
      // Enter in the search box is not "create an invite".
      if ( event.key === "Enter" ) event.preventDefault();
    } );
    Dom.get( "seat-sheet-remove" ).addEventListener( "click" , this.removeFromSeat.bind( this ) );
    Dom.get( "sheet-seat" ).addEventListener( "submit" , function ( event ) {
      event.preventDefault();
      Table.renameSeat( Dom.get( "seat-sheet-rename" ).value );
    } );
    Dom.get( "seat-sheet-reset" ).addEventListener( "click" , function () { Table.renameSeat( "" ); } );

    Dom.all( "[data-menu]" ).forEach( function ( button ) {
      button.addEventListener( "click" , function () {
        switch ( button.getAttribute( "data-menu" ) ) {
          case "chat": Chat.open(); break;
          case "last-trick": Table.openLastTrick(); break;
          case "scores": Table.openScores(); break;
          case "rules": Table.openRules(); break;
          case "share": Table.openShare(); break;
          case "history": History.open(); break;
          case "leave": Table.confirmLeave(); break;
          case "restart": Table.confirmManage( "restart" ); break;
          case "delete": Table.confirmManage( "delete" ); break;
        }
      } );
    } );
    Dom.get( "menu-auto-play" ).addEventListener( "change" , async function ( event ) {
      const toggle = event.target;
      if ( !await Table.setAutoPlay( toggle.checked ) ) toggle.checked = !toggle.checked;
    } );
    // Ctrl+A toggles auto play while you are seated at this table. Not in a
    // text field, where Ctrl+A keeps its usual meaning (select all / start of
    // line), and not with Cmd, so Cmd+A still selects all on a Mac.
    document.addEventListener( "keydown" , function ( event ) {
      if ( !event.ctrlKey || event.metaKey || event.altKey || event.shiftKey ) return;
      if ( event.key.toLowerCase() !== "a" || event.repeat ) return;
      if ( App.screen !== Table || Table.typing( event.target ) ) return;
      Table.toggleAutoPlayShortcut( event );
    } );
    Dom.get( "menu-trick-pause" ).addEventListener( "change" , function ( event ) {
      Table.act( Api.setTrickPause( Table.id , Number( event.target.value ) ) );
    } );
    Dom.get( "menu-observers" ).addEventListener( "change" , function ( event ) {
      Table.act( Api.setObservers( Table.id , event.target.checked ) );
    } );
    Dom.all( "[data-share-copy]" ).forEach( function ( button ) {
      button.addEventListener( "click" , function () {
        Dom.copy( Dom.get( button.getAttribute( "data-share-copy" ) ) , Dom.get( "share-copied" ) );
      } );
    } );
    Dom.all( "[data-share-send]" ).forEach( function ( button ) {
      button.addEventListener( "click" , function () {
        Table.share( Dom.get( button.getAttribute( "data-share-send" ) ).textContent );
      } );
    } );

    Dom.get( "admin-undo" ).addEventListener( "click" , function () { Table.act( Api.undo( Table.id ) ); } );
    Dom.get( "admin-redo" ).addEventListener( "click" , function () { Table.act( Api.redo( Table.id ) ); } );
    Dom.get( "admin-resume" ).addEventListener( "click" , function () { Table.act( Api.resume( Table.id ) ); } );
    Observe.init();
    Zoom.bind( Dom.get( "menu-zoom" ) );
    Dom.all( "[data-player-scores-toggle]" ).forEach( function ( toggle ) {
      toggle.checked = Table.showPlayerScores();
      toggle.addEventListener( "change" , function () { Table.setPlayerScores( toggle.checked ); } );
    } );
  },

  // Auto play: the server plays the bot's choice of card for you on each of
  // your turns, for the rest of this hand. Turns itself off at the next deal.
  setAutoPlay( on ) {
    return this.act( Api.autoPlay( this.id , on ) );
  },

  typing( target ) {
    return Boolean( target ) && ( target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test( target.tagName ) );
  },

  // The Ctrl+A shortcut: the same switch as the one in the table menu, for a
  // seated player once a hand is under way.
  async toggleAutoPlayShortcut( event ) {
    const view = this.view;
    if ( !view || view.you < 0 || !view.game || view.status === "waiting" ) return;
    event.preventDefault();
    if ( view.game.phase !== "playing" || view.status !== "playing" ) {
      Toast.show( I18n.get( "auto_play.only_playing" ) );
      return;
    }
    const on = !view.auto_play;
    if ( await this.setAutoPlay( on ) ) Toast.show( I18n.get( on ? "auto_play.toast_on" : "auto_play.toast_off" ) );
  },

  showPlayerScores() {
    if ( this.playerScores === null ) {
      try {
        this.playerScores = window.localStorage.getItem( this.playerScoresKey ) === "on";
      } catch ( error ) {
        this.playerScores = false;
      }
    }
    return this.playerScores;
  },

  setPlayerScores( on ) {
    this.playerScores = on;
    try {
      window.localStorage.setItem( this.playerScoresKey , on ? "on" : "off" );
    } catch ( error ) {
      // Storage refused: the choice still applies for this page view.
    }
    Dom.all( "[data-player-scores-toggle]" ).forEach( function ( toggle ) { toggle.checked = on; } );
    if ( this.view ) this.renderBar( this.view );
  },

  // target is { id } for #/t/<id> or { watch: code } for a watch link.
  async enter( target ) {
    const key = target.id ? "t:" + target.id : "w:" + target.watch;
    if ( this.key !== key ) {
      this.close();
      Trick.reset();
      Hand.reset();
      Chat.reset();
      Summary.reset();
      Hint.reset();
      Kitty.hand = 0;
      this.view = null;
      this.id = target.id || null;
      this.traySignature = null;
      this.firstDealer = null;
      this.retryDelay = 0;
      this.announced = 0;
      this.perspective = 0;
      this.allHands = false;
    }
    this.key = key;
    this.target = target;
    Dom.show( Dom.get( "connection-banner" ) , false );
    await this.load();
  },

  // The query a watcher's requests carry: whose seat, and every hand or not.
  // A player sends it too and the server ignores it.
  query() {
    return "?seat=" + this.perspective + ( this.allHands ? "&all=1" : "" );
  },

  // Fetch a fresh snapshot and (re)open the stream -- on arrival, and again
  // whenever a watcher changes seat or switches every-hand view.
  //
  // Only a real "no" sends you back to the lobby: the table or link does not
  // exist, or you may not see it. A blip -- no network, the server busy or
  // restarting, too many requests from one house -- keeps you at the table
  // and tries again, with the reconnecting banner up meanwhile.
  async load() {
    const key = this.key;
    this.close();
    Api.tableQuery = this.query();
    let first = null;
    try {
      first = this.target.watch ? await Api.getWatch( this.target.watch ) : await Api.getTable( this.target.id );
    } catch ( error ) {
      if ( this.key !== key ) return;
      if ( [ 401 , 403 , 404 ].indexOf( error.status ) >= 0 ) {
        Toast.error( error );
        App.go( "#/" );
        return;
      }
      Dom.show( Dom.get( "connection-banner" ) , true );
      this.retryDelay = Math.min( ( this.retryDelay || 500 ) * 2 , 15000 );
      this.retryTimer = window.setTimeout( function () {
        if ( Table.key === key ) Table.load();
      } , this.retryDelay );
      return;
    }
    if ( this.key !== key ) return; // navigated away while loading
    this.retryDelay = 0;
    Dom.show( Dom.get( "connection-banner" ) , false );
    this.id = first.id;
    this.force = true;
    this.apply( first );
    const url = this.target.watch ? Api.watchPath( this.target.watch , true ) : Api.tablePath( this.id , "events" );
    this.stream = Stream.open( url , {
      table: function ( view ) { Table.apply( view ); },
      // The stream says the table is gone. Ask once more before leaving:
      // if it really is, load() gets the "not open any more" answer and
      // goes back to the lobby; if not, this simply reconnects.
      gone: function () {
        Table.retryTimer = window.setTimeout( function () {
          if ( Table.key === key ) Table.load();
        } , 1000 );
      },
      status: function ( connected ) { Dom.show( Dom.get( "connection-banner" ) , !connected ); },
    } );
  },

  close() {
    window.clearTimeout( this.retryTimer );
    if ( this.stream ) this.stream.close();
    this.stream = null;
  },

  leave() {
    this.close();
    this.key = null;
    this.id = null;
    Api.tableQuery = "";
  },

  // A watcher picks a different seat or every-hand view.
  watchFrom( seat , allHands ) {
    this.perspective = seat;
    this.allHands = allHands;
    this.traySignature = null;
    Trick.reset();
    Hand.reset();
    this.load();
  },

  // Keep the newest snapshot. An action's answer and the stream can arrive
  // in either order; an older version is simply ignored. The same version
  // may come again with only "who is online" changed, so equal is drawn.
  // force takes the next one regardless, after a watcher changes view.
  apply( view ) {
    if ( !view || view.id !== this.id ) return;
    if ( this.view && view.version < this.view.version && !this.force ) return;
    this.force = false;
    const previous = this.view;
    this.view = view;
    this.render( previous , view );
  },

  // Resolves true when the server accepted the action, so a caller that
  // acted ahead of it (a card already flying to the pile) can undo that.
  async act( request ) {
    try {
      this.apply( await request );
      return true;
    } catch ( error ) {
      Toast.error( error );
      return false;
    }
  },

  render( previous , view ) {
    // Sat down (a host let you take over a bot) or got up: the whole screen
    // is now drawn from a different seat, so start the moving parts afresh.
    if ( previous && previous.you !== view.you ) {
      Trick.reset();
      Hand.reset();
      this.traySignature = null;
      if ( view.you >= 0 && previous.you < 0 && previous.my_request === view.you ) {
        Toast.show( I18n.format( "takeover.welcome" , { name: Seats.name( previous.seats[ view.you ] ) } ) );
      }
    }
    if ( previous && previous.my_request >= 0 && view.my_request < 0 && view.you < 0 && previous.you < 0 ) {
      // Asked, and the request is gone without a seat: turned down, or the
      // bot left -- unless you withdrew it yourself.
      if ( !this.cancelledRequest ) Toast.show( I18n.get( "takeover.not_now" ) );
      this.cancelledRequest = false;
    }

    // Note where your own just-played cards sat in your hand before the hand
    // redraws without them, so they can fly from there.
    const startRects = {};
    if ( view.game && view.you >= 0 && view.perspective === view.you ) {
      view.game.events.forEach( function ( event ) {
        const predicted = Trick.pending && Trick.pending.card === event.card;
        if ( event.kind === "play" && event.seat === view.you && event.seq > Trick.lastSeq && !predicted ) {
          startRects[ event.card ] = Hand.rectOf( event.card );
        }
      } );
    }

    document.title = view.name + " · " + I18n.get( "app.title" );
    this.renderBar( view );
    Seats.render( view , {
      sit: function ( seat ) { Table.act( Api.sit( Table.id , seat ) ); },
      addBot: function ( seat ) { Table.act( Api.addBot( Table.id , seat ) ); },
      removeBot: function ( seat ) { Table.act( Api.removeBot( Table.id , seat ) ); },
      invite: function ( seat ) { Table.openInvite( seat ); },
      manageSeat: function ( seat ) { Table.manageSeat( seat ); },
      requestSeat: function ( seat ) { Table.act( Api.requestSeat( Table.id , seat ) ); },
      cancelRequest: function () {
        Table.cancelledRequest = true;
        Table.act( Api.cancelRequest( Table.id ) );
      },
    } );
    Trick.update( view , startRects );
    Bidding.log( view );
    Hand.render( view );
    Observe.renderHands( view );
    this.renderAdmin( view );
    this.renderRequests( previous , view );
    this.renderTray( view );
    this.renderWaiting( view );
    Summary.update( view );
    Chat.update( view );
    this.announce( view );
    this.nudge( previous , view );
  },

  isMyTurn( view ) {
    const game = view && view.game;
    return Boolean( game && view.status === "playing" && view.you >= 0 && game.turn === view.you &&
      [ "bidding" , "kitty" , "playing" ].indexOf( game.phase ) >= 0 );
  },

  // A short buzz when it becomes your turn, where the phone supports it.
  nudge( previous , view ) {
    if ( this.isMyTurn( view ) && !this.isMyTurn( previous ) && navigator.vibrate ) {
      try { navigator.vibrate( 30 ); } catch ( error ) { /* not allowed: fine */ }
    }
  },

  // The one bar above the table: which table and what contract on the
  // first line, what is happening right now on the second, then the score.
  renderBar( view ) {
    Dom.text( Dom.get( "table-name" ) , view.name );

    const contract = Dom.get( "table-contract" );
    Dom.clear( contract );
    const game = view.game;
    const playing = game && game.bidder >= 0 && ( game.phase === "kitty" || game.phase === "playing" );
    // On a phone the hand's line (who dealt, or who holds the contract)
    // matters more than the table's name, which gives up its room.
    const handLine = playing || ( game && game.phase === "bidding" );
    Dom.get( "screen-table" ).classList.toggle( "has-contract" , Boolean( handLine ) );
    if ( playing ) {
      contract.appendChild( Dom.el( "span" , { class: "contract-bid" , text: I18n.format( "table.contract" , { name: Seats.name( view.seats[ game.bidder ] ) , bid: game.bid } ) } ) );
      if ( game.trump ) contract.appendChild( Dom.el( "span" , { class: "contract-trump trump-" + game.trump , children: [
        Art.glyphIcon( game.trump ) , Dom.el( "span" , { text: I18n.format( "table.trump" , { color: I18n.get( "colors." + game.trump ) } ) } ),
      ] } ) );
      contract.appendChild( Dom.el( "span" , { class: "contract-dealer" , text: I18n.format( "table.dealer" , { name: Seats.name( view.seats[ game.dealer ] ) } ) } ) );
    } else if ( game && game.phase === "bidding" ) {
      contract.appendChild( Dom.el( "span" , { text: I18n.format( "table.hand_number" , { hand: game.hand_number , dealer: Seats.name( view.seats[ game.dealer ] ) } ) } ) );
    }
    // How many are watching. Admins watch invisibly and are never counted.
    if ( view.watchers > 0 ) {
      contract.appendChild( Dom.el( "span" , { class: "watchers-chip" , text: I18n.format( "observe.watchers" , { count: view.watchers } ) } ) );
    }
    // The phone's copy, in its own row under the bar (see index.html).
    Dom.get( "table-contract-row" ).replaceChildren( ...Array.from( contract.childNodes ).map( function ( node ) { return node.cloneNode( true ); } ) );

    this.renderStatus( view );

    const scores = Dom.get( "table-scores" );
    Dom.clear( scores );
    const seat = view.you >= 0 ? view.you : view.perspective;
    const us = seat % 2;
    const totals = game ? game.scores : [ 0 , 0 ];
    // With the setting on, each team's two players and the points they have
    // taken this hand: you (or whoever is watched) then your partner, and
    // your left then your right. Zeros while the hand is bid for, so the bar
    // keeps one height all hand.
    const players = this.showPlayerScores() && game && view.status !== "waiting";
    scores.classList.toggle( "has-players" , Boolean( players ) );
    [ [ us , "us" , seat ] , [ 1 - us , "them" , seat + 1 ] ].forEach( function ( pair ) {
      const children = [
        Dom.el( "span" , { class: "score-label" , text: Summary.teamName( view , pair[ 0 ] ) } ),
        Dom.el( "span" , { class: "score-value" , text: String( totals[ pair[ 0 ] ] ) } ),
      ];
      if ( players ) children.push( Table.scorePlayers( view , [ pair[ 2 ] % 4 , ( pair[ 2 ] + 2 ) % 4 ] ) );
      scores.appendChild( Dom.el( "div" , { class: "score score-" + pair[ 1 ] , children: children } ) );
    } );
  },

  // One team's players under its score. The name is swapped for initials (or
  // their icon) on a phone, where the bar has no room for two names a side.
  scorePlayers( view , seats ) {
    return Dom.el( "span" , { class: "score-players" , children: seats.map( function ( seat ) {
      const info = view.seats[ seat ];
      const name = Seats.name( info );
      const points = view.game.seats[ seat ].taken;
      const label = I18n.format( "player_scores.label" , { name: name , points: points } );
      return Dom.el( "span" , { class: "score-player" , attrs: { title: label } , children: [
        Dom.el( "span" , { class: "score-player-name" , text: name , attrs: { "aria-hidden": "true" } } ),
        Dom.el( "span" , { class: "score-player-short" , text: info && info.kind === "human" && info.icon ? info.icon : Art.initials( name ) , attrs: { "aria-hidden": "true" } } ),
        Dom.el( "span" , { class: "score-player-points" , text: String( points ) , attrs: { "aria-hidden": "true" } } ),
        Dom.el( "span" , { class: "visually-hidden" , text: label } ),
      ] } );
    } ) } );
  },

  // Whose turn and how the hand stands, in the bar's second line. Your own
  // turn is called out; the long "how to play a card" hint only shows where
  // there is room for it.
  renderStatus( view ) {
    const status = Dom.get( "table-status" );
    Dom.clear( status );
    const game = view.game;
    const mine = this.isMyTurn( view );
    status.classList.toggle( "is-your-turn" , mine );
    if ( !game ) {
      Dom.text( status , I18n.get( "status." + view.status ) );
      return;
    }
    const lines = this.statusLines( view );
    let title = lines[ 0 ];
    let hint = "";
    if ( mine && game.phase === "playing" && view.status === "playing" ) hint = I18n.get( "play.your_turn_hint" );
    if ( mine && game.phase === "bidding" && view.status === "playing" ) title = I18n.get( "play.your_bid" );
    if ( mine && game.phase === "kitty" && view.status === "playing" ) title = I18n.get( "play.your_kitty" );
    status.appendChild( Dom.el( "span" , { class: "status-title" , text: title } ) );
    if ( hint ) status.appendChild( Dom.el( "span" , { class: "status-hint" , text: hint } ) );
    if ( game.phase === "playing" && view.status !== "held" && view.status !== "paused" ) {
      const us = view.you >= 0 ? view.you % 2 : 0;
      const numbers = { trick: game.trick_number , us: game.taken[ us ] , them: game.taken[ 1 - us ] };
      status.appendChild( Dom.el( "span" , { class: "status-progress status-long" , text: I18n.format( "play.progress" , numbers ) } ) );
      status.appendChild( Dom.el( "span" , { class: "status-progress status-short" , text: I18n.format( "play.progress_short" , numbers ) } ) );
    } else if ( lines[ 1 ] ) {
      status.appendChild( Dom.el( "span" , { class: "status-progress" , text: lines[ 1 ] } ) );
    }
  },

  renderTray( view ) {
    const tray = Dom.get( "tray" );
    const game = view.game;
    // Skip identical redraws: snapshots also arrive for things the tray does
    // not show (someone coming online, a chat line), and rebuilding the bid
    // stepper under a finger would swallow the tap.
    const signature = JSON.stringify( game ? [
      view.status , view.you , game.phase , game.turn , game.bid , game.high_bidder , game.hand_number ,
      game.trick_number , game.taken , game.trick.plays.length , this.seatNames( view ) ,
      view.perspective , view.all_hands , view.can_see_all , view.can_hint , view.watchers , view.hands_hidden ,
      view.auto_play ,
    ] : [ view.status , view.observing , view.perspective ] ).concat( [ view.my_request ] );
    if ( signature === this.traySignature ) return;
    this.traySignature = signature;
    Dom.clear( tray );
    if ( view.observing ) {
      Observe.renderTray( tray , view );
      return;
    }
    if ( !game || view.status === "waiting" ) return;

    // The hint button, for players an admin gave hints to, on their turn.
    // It goes inside the panel it helps with (its [data-hint-slot]), so the
    // panel stays centered and the tray keeps its one fixed height.
    const withHint = function () {
      if ( !view.can_hint || !Table.isMyTurn( view ) ) return;
      ( tray.querySelector( "[data-hint-slot]" ) || tray ).appendChild( Hint.element( view ) );
    };
    if ( this.isMyTurn( view ) && game.phase === "bidding" ) {
      Bidding.render( tray , view , {
        bid: function ( amount ) { Table.act( Api.bid( Table.id , amount ) ); },
        pass: function () { Table.act( Api.pass( Table.id ) ); },
      } );
      withHint();
      return;
    }
    if ( this.isMyTurn( view ) && game.phase === "kitty" ) {
      Kitty.render( tray , view , {
        bury: function ( cards , trump ) { return Table.act( Api.bury( Table.id , cards , trump ) ); },
      } );
      withHint();
      return;
    }

    // What is happening is in the bar; the tray only holds things to
    // press -- here, the hand's results once it is over.
    if ( game.phase === "game_over" || game.phase === "hand_over" ) {
      tray.appendChild( Dom.el( "div" , { class: "tray-panel" , children: [
        Dom.el( "button" , { class: "small secondary" , text: I18n.get( "table.show_results" ) , attrs: { type: "button" } , on: { click: function () { Summary.open( Table.view ); } } } ),
      ] } ) );
    }
    // While the server plays your cards, say so, with a way to stop it.
    if ( view.auto_play && game.phase === "playing" ) {
      tray.appendChild( Dom.el( "div" , { class: "tray-panel auto-play-panel" , children: [
        Dom.el( "span" , { class: "auto-play-note" , text: I18n.get( "auto_play.on" ) } ),
        Dom.el( "button" , { class: "small secondary" , text: I18n.get( "auto_play.stop" ) , attrs: { type: "button" } , on: { click: function () { Table.setAutoPlay( false ); } } } ),
      ] } ) );
      return;
    }
    withHint();
  },

  // People asking to take over a bot, for the host (and admins) to answer.
  // One small card each, pinned to the top of the felt.
  renderRequests( previous , view ) {
    const box = Dom.get( "takeover-requests" );
    const requests = view.requests || [];
    const signature = JSON.stringify( requests.map( function ( request ) { return [ request.user_id , request.seat , request.name ]; } ) );
    if ( box.getAttribute( "data-signature" ) === signature ) return;
    box.setAttribute( "data-signature" , signature );
    Dom.clear( box );
    box.hidden = requests.length === 0;

    const before = new Set( ( previous && previous.requests ? previous.requests : [] ).map( function ( request ) { return request.user_id; } ) );
    const fresh = requests.filter( function ( request ) { return !before.has( request.user_id ); } );
    if ( previous && fresh.length ) {
      Toast.show( I18n.format( "takeover.incoming" , { name: fresh[ 0 ].name } ) );
      if ( navigator.vibrate ) {
        try { navigator.vibrate( [ 30 , 60 , 30 ] ); } catch ( error ) { /* not allowed: fine */ }
      }
    }

    requests.forEach( function ( request ) {
      const avatar = Dom.el( "span" , { class: "avatar avatar-small" , attrs: { "aria-hidden": "true" } } );
      Art.avatar( avatar , { name: request.name , display_name: request.name , icon: request.icon , color: request.color } );
      const answer = function ( accept ) {
        return function () { Table.act( Api.answerRequest( Table.id , request.user_id , accept ) ); };
      };
      box.appendChild( Dom.el( "div" , { class: "takeover-card" , children: [
        avatar,
        Dom.el( "p" , { class: "takeover-text" , text: I18n.format( "takeover.prompt" , { name: request.name , seat: Seats.name( view.seats[ request.seat ] ) } ) } ),
        Dom.el( "div" , { class: "takeover-actions" , children: [
          Dom.el( "button" , { class: "small" , text: I18n.get( "takeover.allow" ) , attrs: { type: "button" } , on: { click: answer( true ) } } ),
          Dom.el( "button" , { class: "small ghost" , text: I18n.get( "takeover.decline" ) , attrs: { type: "button" } , on: { click: answer( false ) } } ),
        ] } ),
      ] } ) );
    } );
  },

  // The history controls, in the bar: undo, redo, and Resume while the
  // table is held. The move count shows where there is room. For admins,
  // and for players an admin let undo at their own tables (can_rewind).
  renderAdmin( view ) {
    const controls = Dom.get( "admin-controls" );
    controls.hidden = !view.can_rewind;
    Dom.get( "screen-table" ).classList.toggle( "is-held" , Boolean( view.held ) );
    if ( !view.can_rewind ) return;
    Dom.get( "admin-undo" ).disabled = !view.can_undo;
    Dom.get( "admin-redo" ).disabled = !view.can_redo;
    Dom.show( Dom.get( "admin-resume" ) , view.held );
    Dom.text( Dom.get( "admin-position" ) , view.moves > 0 ? I18n.format( "admin_game.position" , { at: view.cursor , of: view.moves } ) : "" );
  },

  seatNames( view ) {
    return view.seats.map( function ( seat ) { return Seats.name( seat ); } ).join( "|" );
  },

  statusLines( view ) {
    const game = view.game;
    const turnName = game.turn >= 0 ? Seats.name( view.seats[ game.turn ] ) : "";
    const us = view.you >= 0 ? view.you % 2 : 0;
    // The panel on the felt explains a pause; the tray only names it.
    if ( view.status === "paused" ) return [ I18n.get( "table.paused" ) , "" ];
    if ( view.status === "held" ) return [ I18n.get( "table.held" ) , I18n.get( "table.held_detail" ) ];
    switch ( game.phase ) {
      case "bidding":
        return [
          I18n.format( "bidding.waiting" , { name: turnName } ),
          game.high_bidder >= 0
            ? I18n.format( "bidding.current" , { bid: game.bid , name: Seats.name( view.seats[ game.high_bidder ] ) } )
            : I18n.format( "bidding.opening" , { bid: game.min_bid } ),
        ];
      case "kitty":
        return [ I18n.format( "kitty.waiting" , { name: turnName } ) , "" ];
      case "playing":
        return [
          this.isMyTurn( view ) ? I18n.get( "play.your_turn" ) : I18n.format( "play.waiting" , { name: turnName } ),
          I18n.format( "play.progress" , { trick: game.trick_number , us: game.taken[ us ] , them: game.taken[ 1 - us ] } ),
        ];
      case "hand_over":
        return [ I18n.get( "table.hand_over" ) , "" ];
      case "game_over":
        return [ I18n.get( "table.game_over" ) , "" ];
      default:
        return [ "" , "" ];
    }
  },

  // The panel over the felt before the deal, or while a seat is empty.
  renderWaiting( view ) {
    const room = Dom.get( "waiting-room" );
    const showing = view.status === "waiting" || view.status === "paused";
    room.hidden = !showing;
    Dom.get( "felt" ).classList.toggle( "is-waiting" , showing );
    if ( !showing ) return;

    Dom.text( Dom.get( "waiting-title" ) , view.status === "paused" ? I18n.get( "table.paused" ) : view.name );
    const rules = Dom.get( "waiting-rules" );
    Dom.clear( rules );
    Lobby.ruleChips( view.options ).forEach( function ( chip ) { rules.appendChild( chip ); } );

    const seated = view.seats.filter( function ( seat ) { return seat.kind !== ""; } ).length;
    const host = view.seats.find( function ( seat ) { return seat.host; } );
    let status = "";
    if ( view.you < 0 ) {
      status = I18n.get( "waiting.pick_seat" );
    } else if ( view.status === "paused" ) {
      status = I18n.get( "table.paused_detail" );
    } else if ( seated < 4 ) {
      status = I18n.format( "waiting.seated" , { seated: seated } );
    } else if ( view.is_host ) {
      status = I18n.get( "waiting.ready_host" );
    } else {
      status = I18n.format( "waiting.ready_guest" , { name: host ? host.name : "" } );
    }
    Dom.text( Dom.get( "waiting-status" ) , status );

    const dealing = view.is_host && view.status === "waiting" && view.you >= 0;
    this.renderDealerPick( view , dealing && seated === 4 );
    const deal = Dom.get( "deal-button" );
    Dom.show( deal , dealing );
    // Nobody deals by default: the host has to say who deals first.
    deal.disabled = seated < 4 || this.firstDealer === null;
    Dom.show( Dom.get( "waiting-leave" ) , view.you >= 0 );
  },

  // Who deals first: any of the four, clockwise from you as they sit, or
  // Random. Nothing is picked until the host picks.
  renderDealerPick( view , showing ) {
    Dom.show( Dom.get( "dealer-pick" ) , showing );
    const box = Dom.get( "dealer-pick-options" );
    if ( !showing ) {
      box.removeAttribute( "data-signature" );
      return;
    }
    const signature = JSON.stringify( [ this.seatNames( view ) , view.you , this.firstDealer ] );
    if ( box.getAttribute( "data-signature" ) === signature ) return;
    box.setAttribute( "data-signature" , signature );
    Dom.clear( box );
    const option = function ( value , text ) {
      box.appendChild( Dom.el( "button" , {
        class: "small secondary dealer-pick-option" + ( value === "random" ? " dealer-pick-random" : "" ),
        text: text,
        attrs: { type: "button" , role: "radio" , "aria-checked": String( Table.firstDealer === value ) },
        on: { click: function () {
          Table.firstDealer = value;
          Table.renderWaiting( Table.view );
        } },
      } ) );
    };
    for ( let step = 0; step < 4; step += 1 ) {
      const seat = ( view.you + step ) % 4;
      const name = Seats.name( view.seats[ seat ] );
      // "You" rather than "Ana (you)": two to a row has little room.
      option( seat , seat === view.you ? I18n.get( "waiting.dealer_you" ) : name );
    }
    option( "random" , I18n.get( "waiting.dealer_random" ) );
  },

  // Read the newest game event to a screen reader.
  announce( view ) {
    if ( !view.game ) return;
    const events = view.game.events;
    const latest = events[ events.length - 1 ];
    if ( !latest || latest.seq <= this.announced ) return;
    this.announced = latest.seq;
    Dom.text( Dom.get( "game-live" ) , Chat.eventText( view , latest ) );
  },

  openMenu() {
    const view = this.view;
    Dom.text( Dom.get( "menu-title" ) , view ? view.name : "" );
    if ( view ) {
      Dom.show( document.querySelector( "[data-menu='share']" ) , Boolean( view.join_code ) );
      Dom.show( document.querySelector( "[data-menu='history']" ) , view.can_rewind );
      Dom.show( Dom.get( "menu-observers-row" ) , view.is_admin );
      Dom.get( "menu-observers" ).checked = view.observers_see_all;
      // This table's pause after a trick: admins only.
      Dom.show( Dom.get( "menu-trick-pause-row" ) , view.is_admin );
      if ( view.is_admin ) {
        const pause = Dom.get( "menu-trick-pause" );
        Links.fillPauses( pause , view.trick_pause_default );
        pause.value = String( view.trick_pause_table || 0 );
      }
      // Auto play: for a seated player, once the cards are being played.
      const playing = Boolean( view.game ) && view.game.phase === "playing" && view.status === "playing";
      Dom.show( Dom.get( "menu-auto-play-row" ) , view.you >= 0 && Boolean( view.game ) && view.status !== "waiting" );
      Dom.get( "menu-auto-play" ).checked = Boolean( view.auto_play );
      Dom.get( "menu-auto-play" ).disabled = !playing;
      Dom.text( Dom.get( "menu-auto-play-help" ) , I18n.get( playing ? "auto_play.description" : "auto_play.only_playing" ) );
      Dom.show( document.querySelector( "[data-menu='leave']" ) , view.you >= 0 );
      // Restart and delete: the host's call, or an admin's.
      const manager = view.is_host || view.is_admin;
      Dom.show( document.querySelector( "[data-menu='restart']" ) , manager && Boolean( view.game ) );
      Dom.show( document.querySelector( "[data-menu='delete']" ) , manager );
    }
    Sheet.open( "sheet-menu" , { focus: false } );
  },

  // The table's two shareable links: join (signed-in people take a seat)
  // and watch (anyone can look on).
  openShare() {
    const view = this.view;
    Dom.text( Dom.get( "share-join-link" ) , window.location.origin + "/#/join/" + view.join_code );
    Dom.text( Dom.get( "share-watch-link" ) , window.location.origin + "/#/watch/" + view.watch_code );
    Dom.all( "[data-share-send]" ).forEach( function ( button ) {
      Dom.show( button , typeof navigator.share === "function" );
    } );
    Sheet.open( "sheet-share" , { focus: false } );
  },

  async share( url ) {
    try {
      await navigator.share( { title: this.view ? this.view.name : I18n.get( "app.title" ) , url: url } );
    } catch ( error ) {
      // Dismissing the share sheet rejects; nothing to report.
    }
  },

  openLastTrick() {
    const view = this.view;
    const last = view && view.game ? view.game.last_trick : null;
    const cards = Dom.get( "last-trick-cards" );
    Dom.clear( cards );
    Dom.show( Dom.get( "last-trick-empty" ) , !last );
    Dom.text( Dom.get( "last-trick-caption" ) , last ? I18n.format( "last_trick.caption" , { name: Seats.name( view.seats[ last.winner ] ) , points: last.points } ) : "" );
    if ( last ) {
      last.plays.forEach( function ( play ) {
        const figure = Dom.el( "figure" , { class: "mini-play" + ( play.seat === last.winner ? " is-winner" : "" ) , children: [
          Cards.el( play.card , { class: "mini" } ),
          Dom.el( "figcaption" , { text: Seats.name( view.seats[ play.seat ] ) } ),
        ] } );
        cards.appendChild( figure );
      } );
    }
    Sheet.open( "sheet-last-trick" , { focus: false } );
  },

  openScores() {
    const view = this.view;
    const us = view.you >= 0 ? view.you % 2 : 0;
    Dom.text( Dom.get( "scores-us-heading" ) , Summary.teamName( view , us ) );
    Dom.text( Dom.get( "scores-them-heading" ) , Summary.teamName( view , 1 - us ) );
    const body = Dom.get( "scores-body" );
    Dom.clear( body );
    const history = view.game ? view.game.history : [];
    Dom.show( Dom.get( "scores-empty" ) , history.length === 0 );
    history.forEach( function ( result ) {
      const cell = function ( team ) {
        return Dom.el( "td" , { class: result.scored[ team ] < 0 ? "is-set" : "" , children: [
          Dom.el( "span" , { class: "score-delta" , text: ( result.scored[ team ] > 0 ? "+" : "" ) + result.scored[ team ] } ),
          Dom.el( "span" , { class: "score-total" , text: String( result.totals[ team ] ) } ),
        ] } );
      };
      body.appendChild( Dom.el( "tr" , { children: [
        Dom.el( "td" , { text: String( result.hand ) } ),
        Dom.el( "td" , { children: [
          Dom.el( "span" , { text: I18n.format( "scores.bid_by" , { bid: result.bid , name: Seats.name( view.seats[ result.bidder ] ) } ) } ),
          Art.glyphIcon( result.trump , "glyph inline-glyph" ),
        ] } ),
        cell( us ),
        cell( 1 - us ),
      ] } ) );
    } );
    Sheet.open( "sheet-scores" , { focus: false } );
  },

  openRules() {
    const chips = Dom.get( "rules-table-options" );
    Dom.clear( chips );
    Lobby.ruleChips( this.view.options ).forEach( function ( chip ) { chips.appendChild( chip ); } );
    Sheet.open( "sheet-rules" , { focus: false } );
  },

  confirmLeave() {
    const view = this.view;
    const inGame = view && view.game && view.status !== "over";
    Dom.text( Dom.get( "leave-body" ) , I18n.get( inGame ? "leave.body_game" : "leave.body_waiting" ) );
    Sheet.open( "sheet-confirm-leave" , { focus: false } );
  },

  // Restart or delete, after a confirm: neither can be undone. Without a
  // table id it is the table on screen; the lobby passes one so an admin can
  // manage any game from the list.
  confirmManage( action , tableId ) {
    this.manageAction = action;
    this.manageId = tableId || null;
    Dom.text( Dom.get( "manage-heading" ) , I18n.get( "manage." + action + "_heading" ) );
    Dom.text( Dom.get( "manage-body" ) , I18n.get( "manage." + action + "_body" ) );
    Dom.text( Dom.get( "manage-confirm" ) , I18n.get( "manage." + action + "_confirm" ) );
    Sheet.open( "sheet-confirm-manage" , { focus: false } );
  },

  async manageTable() {
    const action = this.manageAction;
    const fromLobby = Boolean( this.manageId );
    const id = this.manageId || this.id;
    try {
      if ( action === "delete" ) {
        await Api.deleteTable( id );
        Sheet.close( true );
        Toast.show( I18n.get( "manage.deleted" ) );
        // The lobby redraws itself from its own stream.
        if ( !fromLobby ) App.go( "#/" );
        return;
      }
      const view = await Api.restart( id );
      Sheet.close( true );
      if ( fromLobby ) {
        Toast.show( I18n.get( "manage.restarted" ) );
        return;
      }
      // A new game from the first card: draw it fresh, not as a replay.
      Trick.reset();
      Hand.reset();
      Record.reset();
      Summary.reset();
      this.traySignature = null;
      this.force = true;
      this.apply( view );
      Toast.show( I18n.get( "manage.restarted" ) );
    } catch ( error ) {
      Toast.error( error );
    }
  },

  async leaveTable() {
    try {
      await Api.leave( this.id );
      Sheet.close( true );
      App.go( "#/" );
    } catch ( error ) {
      Toast.error( error );
    }
  },

  // The host or an admin clicked a player's avatar: rename them for this
  // table, or take them out of the seat.
  manageSeat( seat ) {
    const view = this.view;
    const info = view && view.seats[ seat ];
    if ( !info || info.kind === "" ) return;
    this.manageSeatIndex = seat;
    this.manageSeatName = Seats.name( info );
    if ( info.kind === "human" ) {
      Art.avatar( Dom.get( "seat-sheet-avatar" ) , info );
    } else {
      Dom.text( Dom.get( "seat-sheet-avatar" ) , I18n.get( "bots.badge" ) );
    }
    Dom.text( Dom.get( "seat-sheet-name" ) , this.manageSeatName );
    Dom.get( "seat-sheet-rename" ).value = this.manageSeatName;
    Dom.show( Dom.get( "seat-sheet-reset" ) , info.renamed );

    // Removing: anyone's seat but your own (that is Leave), person or bot,
    // at any point in the game.
    const removable = seat !== view.you;
    const midGame = Boolean( view.game ) && view.status !== "over";
    let body = "seat_manage.body_waiting";
    if ( info.pending ) {
      body = "seat_manage.body_pending";
    } else if ( info.kind === "bot" ) {
      body = midGame ? "seat_manage.body_bot_game" : "seat_manage.body_bot";
    } else if ( midGame ) {
      body = "seat_manage.body_game";
    }
    Dom.text( Dom.get( "seat-sheet-body" ) , I18n.get( body ) );
    Dom.show( Dom.get( "seat-sheet-remove-box" ) , removable );
    Dom.show( Dom.get( "seat-sheet-close-only" ) , !removable );
    Sheet.open( "sheet-seat" , { focus: false } );
  },

  async renameSeat( name ) {
    const before = this.manageSeatName;
    try {
      const view = await Api.renameSeat( this.id , this.manageSeatIndex , name.trim() );
      Sheet.close( true );
      this.apply( view );
      Toast.show( I18n.format( "seat_manage.renamed" , { old: before , name: Seats.name( view.seats[ this.manageSeatIndex ] ) } ) );
    } catch ( error ) {
      Toast.error( error );
    }
  },

  async removeFromSeat() {
    const name = this.manageSeatName;
    Sheet.close( true );
    if ( await this.act( Api.removePlayer( this.id , this.manageSeatIndex ) ) ) {
      Toast.show( I18n.format( "seat_manage.removed" , { name: name } ) );
    }
  },

  // Invite to an empty seat: pick someone who already has an account, or --
  // for people allowed to bring new ones in -- create one with a login link.
  openInvite( seat ) {
    this.inviteSeat = seat;
    this.inviteUsers = null;
    const canCreate = Boolean( this.view && this.view.can_invite );
    Dom.get( "invite-name" ).value = "";
    Dom.get( "invite-filter" ).value = "";
    Dom.show( Dom.get( "invite-pick" ) , true );
    Dom.show( Dom.get( "invite-form-fields" ) , canCreate );
    Dom.show( Dom.get( "invite-pick-only" ) , !canCreate );
    Dom.show( Dom.get( "invite-result" ) , false );
    this.renderInviteUsers();
    Sheet.open( "sheet-invite" , { focus: false } );
    Api.listTeam().then( function ( users ) {
      Table.inviteUsers = users;
      Table.renderInviteUsers();
    } ).catch( function ( error ) { Toast.error( error ); } );
  },

  renderInviteUsers() {
    const list = Dom.get( "invite-users" );
    Dom.clear( list );
    if ( !this.inviteUsers ) return;
    const seated = {};
    ( this.view ? this.view.seats : [] ).forEach( function ( seat ) {
      if ( seat.user_id ) seated[ seat.user_id ] = true;
    } );
    const filter = Dom.get( "invite-filter" ).value.trim().toLowerCase();
    const users = this.inviteUsers.filter( function ( user ) {
      return !seated[ user.id ] && ( filter === "" || user.display_name.toLowerCase().indexOf( filter ) >= 0 );
    } );
    if ( users.length === 0 ) {
      list.appendChild( Dom.el( "li" , { class: "invite-users-empty" , text: I18n.get( "invite.no_users" ) } ) );
      return;
    }
    users.forEach( function ( user ) {
      const avatar = Dom.el( "span" , { class: "mini-avatar" , attrs: { "aria-hidden": "true" } } );
      Art.avatar( avatar , user );
      list.appendChild( Dom.el( "li" , { class: "invite-user" , children: [
        avatar,
        Dom.el( "span" , { class: "invite-user-name" , text: user.display_name } ),
        Dom.el( "button" , {
          class: "small" , text: I18n.get( "invite.assign" ) , attrs: { type: "button" } ,
          on: { click: function () { Table.assignSeat( user ); } },
        } ),
      ] } ) );
    } );
  },

  async assignSeat( user ) {
    try {
      const view = await Api.assignSeat( this.id , this.inviteSeat , user.id );
      Sheet.close( true );
      this.apply( view );
      Toast.show( I18n.format( "invite.assigned" , { name: user.display_name } ) );
    } catch ( error ) {
      Toast.error( error );
    }
  },

  async onInvite( event ) {
    event.preventDefault();
    const name = Dom.get( "invite-name" ).value.trim();
    if ( !name ) return;
    try {
      const created = await Api.invite( this.id , this.inviteSeat , name );
      Dom.text( Dom.get( "invite-link" ) , window.location.origin + created.login_path );
      Dom.show( Dom.get( "invite-pick" ) , false );
      Dom.show( Dom.get( "invite-form-fields" ) , false );
      Dom.show( Dom.get( "invite-result" ) , true );
      // The share sheet is the natural way to send a link from a phone;
      // desktop browsers mostly lack it, and Copy covers them.
      Dom.show( Dom.get( "invite-share" ) , typeof navigator.share === "function" );
      this.inviteName = name;
    } catch ( error ) {
      Toast.error( error );
    }
  },

  async shareInvite() {
    try {
      await navigator.share( {
        title: I18n.get( "app.title" ),
        text: I18n.format( "invite.share_text" , { name: this.inviteName , table: this.view ? this.view.name : "" } ),
        url: Dom.get( "invite-link" ).textContent,
      } );
    } catch ( error ) {
      // Dismissing the share sheet rejects; nothing to report.
    }
  },
};
