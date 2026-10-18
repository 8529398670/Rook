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
  announced: 0,
  // What a watcher asked to see: whose seat, and every hand or not.
  perspective: 0,
  allHands: false,

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
    Chat.init( { send: function ( text ) { Table.act( Api.chat( Table.id , text ) ); } } );

    Dom.get( "table-back" ).addEventListener( "click" , function () { App.go( "#/" ); } );
    Dom.get( "table-menu" ).addEventListener( "click" , this.openMenu.bind( this ) );
    Dom.get( "deal-button" ).addEventListener( "click" , function () { Table.act( Api.start( Table.id ) ); } );
    Dom.get( "waiting-leave" ).addEventListener( "click" , this.confirmLeave.bind( this ) );
    Dom.get( "leave-confirm" ).addEventListener( "click" , this.leaveTable.bind( this ) );
    Dom.get( "manage-confirm" ).addEventListener( "click" , this.manageTable.bind( this ) );
    Dom.get( "sheet-invite" ).addEventListener( "submit" , this.onInvite.bind( this ) );
    Dom.get( "invite-copy" ).addEventListener( "click" , function () {
      Dom.copy( Dom.get( "invite-link" ) , Dom.get( "invite-copied" ) );
    } );
    Dom.get( "invite-share" ).addEventListener( "click" , this.shareInvite.bind( this ) );

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
  async load() {
    const key = this.key;
    this.close();
    Api.tableQuery = this.query();
    let first = null;
    try {
      first = this.target.watch ? await Api.getWatch( this.target.watch ) : await Api.getTable( this.target.id );
    } catch ( error ) {
      Toast.error( error );
      App.go( "#/" );
      return;
    }
    if ( this.key !== key ) return; // navigated away while loading
    this.id = first.id;
    this.force = true;
    this.apply( first );
    const url = this.target.watch ? Api.watchPath( this.target.watch , true ) : Api.tablePath( this.id , "events" );
    this.stream = Stream.open( url , {
      table: function ( view ) { Table.apply( view ); },
      gone: function () {
        Toast.show( I18n.get( "table.gone" ) );
        App.go( App.me ? "#/" : "#/gone" );
      },
      status: function ( connected ) { Dom.show( Dom.get( "connection-banner" ) , !connected ); },
    } );
  },

  close() {
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
      requestSeat: function ( seat ) { Table.act( Api.requestSeat( Table.id , seat ) ); },
      cancelRequest: function () {
        Table.cancelledRequest = true;
        Table.act( Api.cancelRequest( Table.id ) );
      },
    } );
    Trick.update( view , startRects );
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
    Dom.get( "screen-table" ).classList.toggle( "has-contract" , Boolean( playing ) );
    if ( playing ) {
      contract.appendChild( Dom.el( "span" , { text: I18n.format( "table.contract" , { name: Seats.name( view.seats[ game.bidder ] ) , bid: game.bid } ) } ) );
      if ( game.trump ) contract.appendChild( Dom.el( "span" , { class: "contract-trump trump-" + game.trump , children: [
        Art.glyphIcon( game.trump ) , Dom.el( "span" , { text: I18n.get( "colors." + game.trump ) } ),
      ] } ) );
    } else if ( game && game.phase === "bidding" ) {
      contract.appendChild( Dom.el( "span" , { text: I18n.format( "table.hand_number" , { hand: game.hand_number } ) } ) );
    }
    // How many are watching. Admins watch invisibly and are never counted.
    if ( view.watchers > 0 ) {
      contract.appendChild( Dom.el( "span" , { class: "watchers-chip" , text: I18n.format( "observe.watchers" , { count: view.watchers } ) } ) );
    }

    this.renderStatus( view );

    const scores = Dom.get( "table-scores" );
    Dom.clear( scores );
    const us = view.you >= 0 ? view.you % 2 : view.perspective % 2;
    const totals = game ? game.scores : [ 0 , 0 ];
    [ [ us , "us" ] , [ 1 - us , "them" ] ].forEach( function ( pair ) {
      scores.appendChild( Dom.el( "div" , { class: "score score-" + pair[ 1 ] , children: [
        Dom.el( "span" , { class: "score-label" , text: Summary.teamName( view , pair[ 0 ] ) } ),
        Dom.el( "span" , { class: "score-value" , text: String( totals[ pair[ 0 ] ] ) } ),
      ] } ) );
    } );
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
    const withHint = function () {
      if ( view.can_hint && Table.isMyTurn( view ) ) tray.appendChild( Hint.element( view ) );
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
        bury: function ( cards , trump ) { Table.act( Api.bury( Table.id , cards , trump ) ); },
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

  // The admin's controls, in the bar: undo, redo, and Resume while the
  // table is held. The move count shows where there is room.
  renderAdmin( view ) {
    const controls = Dom.get( "admin-controls" );
    controls.hidden = !view.is_admin;
    Dom.get( "screen-table" ).classList.toggle( "is-held" , Boolean( view.held ) );
    if ( !view.is_admin ) return;
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
    Dom.get( "screen-table" ).classList.toggle( "is-seating" , !view.game );
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

    const deal = Dom.get( "deal-button" );
    Dom.show( deal , view.is_host && view.status === "waiting" && view.you >= 0 );
    deal.disabled = seated < 4;
    Dom.show( Dom.get( "waiting-leave" ) , view.you >= 0 );
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
      Dom.show( document.querySelector( "[data-menu='history']" ) , view.is_admin );
      Dom.show( Dom.get( "menu-observers-row" ) , view.is_admin );
      Dom.get( "menu-observers" ).checked = view.observers_see_all;
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

  // Restart or delete, after a confirm: neither can be undone.
  confirmManage( action ) {
    this.manageAction = action;
    Dom.text( Dom.get( "manage-heading" ) , I18n.get( "manage." + action + "_heading" ) );
    Dom.text( Dom.get( "manage-body" ) , I18n.get( "manage." + action + "_body" ) );
    Dom.text( Dom.get( "manage-confirm" ) , I18n.get( "manage." + action + "_confirm" ) );
    Sheet.open( "sheet-confirm-manage" , { focus: false } );
  },

  async manageTable() {
    const action = this.manageAction;
    try {
      if ( action === "delete" ) {
        await Api.deleteTable( this.id );
        Sheet.close( true );
        Toast.show( I18n.get( "manage.deleted" ) );
        App.go( "#/" );
        return;
      }
      const view = await Api.restart( this.id );
      Sheet.close( true );
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

  openInvite( seat ) {
    this.inviteSeat = seat;
    Dom.get( "invite-name" ).value = "";
    Dom.show( Dom.get( "invite-form-fields" ) , true );
    Dom.show( Dom.get( "invite-result" ) , false );
    Sheet.open( "sheet-invite" );
  },

  async onInvite( event ) {
    event.preventDefault();
    const name = Dom.get( "invite-name" ).value.trim();
    if ( !name ) return;
    try {
      const created = await Api.invite( this.id , this.inviteSeat , name );
      Dom.text( Dom.get( "invite-link" ) , window.location.origin + created.login_path );
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
