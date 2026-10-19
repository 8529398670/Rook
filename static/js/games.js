// The game history: every game played here -- finished, or given up partway
// -- newest first, open to anyone signed in. #/games is the list; a game opens
// at #/games/<id> with its final score and the whole hand-by-hand log, drawn
// by record.js exactly as at the table. The owner may delete a game, from
// its card or from the game itself.
const Games = {
  screenId: "screen-games",
  bound: false,
  list: [],
  more: false,
  loading: false,
  detail: false,

  bind() {
    if ( this.bound ) return;
    this.bound = true;
    Dom.get( "games-back" ).addEventListener( "click" , function () { App.go( Games.detail ? "#/games" : "#/" ); } );
    Dom.get( "games-more" ).addEventListener( "click" , function () { Games.loadMore(); } );
  },

  async enter( id ) {
    this.bind();
    this.detail = Boolean( id );
    Dom.show( Dom.get( "games-list-view" ) , !this.detail );
    Dom.show( Dom.get( "games-detail-view" ) , this.detail );
    Dom.text( Dom.get( "games-title" ) , I18n.get( "games.heading" ) );
    if ( this.detail ) {
      await this.showGame( id );
      return;
    }
    // Always from the top: games may have ended since the last visit.
    this.list = [];
    this.more = false;
    Dom.clear( Dom.get( "games-list" ) );
    await this.loadMore();
  },

  leave() {
    Record.reset();
  },

  async loadMore() {
    if ( this.loading ) return;
    this.loading = true;
    try {
      const page = await Api.listGames( this.list.length );
      this.list = this.list.concat( page.games );
      this.more = page.more;
      page.games.forEach( function ( game ) { Dom.get( "games-list" ).appendChild( Games.card( game ) ); } );
    } catch ( error ) {
      Toast.error( error );
    }
    this.loading = false;
    Dom.show( Dom.get( "games-empty" ) , this.list.length === 0 );
    Dom.show( Dom.get( "games-more" ) , this.more );
  },

  when( game ) {
    try {
      return new Date( game.ended_at ).toLocaleString( undefined , { dateStyle: "medium" , timeStyle: "short" } );
    } catch ( error ) {
      return new Date( game.ended_at ).toLocaleString();
    }
  },

  hands( game ) {
    return game.hands_played === 1 ? I18n.get( "games.one_hand" ) : I18n.format( "games.hands" , { count: game.hands_played } );
  },

  avatar( seat ) {
    const avatar = Dom.el( "span" , { class: "avatar avatar-small" , attrs: { "aria-hidden": "true" } } );
    if ( seat.kind === "human" ) {
      Art.avatar( avatar , seat );
    } else {
      avatar.classList.add( "is-" + ( seat.kind || "empty" ) );
      Dom.text( avatar , seat.kind === "bot" ? I18n.get( "bots.badge" ) : "" );
    }
    return avatar;
  },

  // One team's line: both partners, their score, and "Won" for the winners.
  // Seats 0 and 2 are team 1, seats 1 and 3 team 2.
  teamRow( game , team ) {
    const seats = [ game.seats[ team ] , game.seats[ team + 2 ] ];
    const won = game.status === "finished" && game.winner === team;
    const row = Dom.el( "div" , { class: "game-team" + ( won ? " is-winner" : "" ) , children: [
      Dom.el( "span" , { class: "game-team-avatars" , children: seats.map( Games.avatar ) } ),
      Dom.el( "span" , { class: "game-team-names" , text: I18n.format( "games.team_names" , { a: Seats.name( seats[ 0 ] ) , b: Seats.name( seats[ 1 ] ) } ) } ),
    ] } );
    if ( won ) row.appendChild( Dom.el( "span" , { class: "status-chip status-playing" , text: I18n.get( "games.won" ) } ) );
    row.appendChild( Dom.el( "span" , { class: "game-team-score" , text: String( game.totals[ team ] ) } ) );
    return row;
  },

  statusChip( game ) {
    const finished = game.status === "finished";
    return Dom.el( "span" , {
      class: "status-chip " + ( finished ? "status-over" : "status-paused" ),
      text: I18n.get( finished ? "games.finished" : "games.unfinished" ),
    } );
  },

  isOwner() {
    return Boolean( App.me && App.me.owner );
  },

  card( game ) {
    const when = this.when( game );
    const body = Dom.el( "div" , { class: "table-card-body" , children: [
      Dom.el( "div" , { class: "table-card-top" , children: [
        Dom.el( "span" , { class: "table-card-name" , text: game.table_name } ),
        this.statusChip( game ),
      ] } ),
      Dom.el( "span" , { class: "table-card-status" , text: when + " · " + this.hands( game ) } ),
      this.teamRow( game , 0 ),
      this.teamRow( game , 1 ),
    ] } );
    const open = {
      attrs: { type: "button" , "aria-label": I18n.format( "games.open_game" , { name: game.table_name , date: when } ) },
      on: { click: function () { App.go( "#/games/" + game.id ); } },
      children: [ body ],
    };
    if ( !this.isOwner() ) return Dom.el( "button" , Object.assign( { class: "table-card game-card" } , open ) );

    // The owner's card: the opening button with Delete beside it, as on the
    // lobby's table cards, so no button sits inside another.
    const card = Dom.el( "div" , { class: "table-card game-card-owned" , children: [
      Dom.el( "button" , Object.assign( { class: "table-card-main" } , open ) ),
    ] } );
    card.appendChild( Dom.el( "div" , { class: "table-card-actions" , children: [
      this.deleteButton( game , function () {
        Games.list = Games.list.filter( function ( other ) { return other.id !== game.id; } );
        card.remove();
        Dom.show( Dom.get( "games-empty" ) , Games.list.length === 0 && !Games.more );
      } ),
    ] } ) );
    return card;
  },

  // Delete asks first, then runs done() once the game is gone.
  deleteButton( game , done ) {
    return Dom.el( "button" , {
      class: "small secondary table-card-delete",
      text: I18n.get( "games.delete" ),
      attrs: { type: "button" },
      on: { click: async function () {
        const question = I18n.format( "games.delete_confirm" , { name: game.table_name } );
        if ( question !== "" && !window.confirm( question ) ) return;
        try {
          await Api.deleteGame( game.id );
          Toast.show( I18n.get( "games.deleted" ) );
          done();
        } catch ( error ) {
          Toast.error( error );
        }
      } },
    } );
  },

  async showGame( id ) {
    const head = Dom.get( "games-detail-head" );
    const list = Dom.get( "games-detail-record" );
    Dom.clear( head );
    Dom.clear( list );
    Record.reset();
    let game = null;
    try {
      game = await Api.getGame( id );
    } catch ( error ) {
      Toast.error( error );
      App.go( "#/games" );
      return;
    }
    if ( App.screen !== this || !this.detail ) return; // navigated away meanwhile
    Dom.text( Dom.get( "games-title" ) , game.table_name );
    head.appendChild( Dom.el( "div" , { class: "table-card-top" , children: [
      Dom.el( "span" , { class: "table-card-status" , text: this.when( game ) + " · " + this.hands( game ) } ),
      this.statusChip( game ),
    ] } ) );
    head.appendChild( this.teamRow( game , 0 ) );
    head.appendChild( this.teamRow( game , 1 ) );
    if ( this.isOwner() ) {
      head.appendChild( Dom.el( "div" , { class: "game-detail-actions" , children: [
        this.deleteButton( game , function () { App.go( "#/games" ); } ),
      ] } ) );
    }

    // record.js draws from a table view; an archived game is seen as from
    // the rail, so the teams are "Team 1" and "Team 2".
    const view = { seats: game.seats , you: -1 , perspective: 0 };
    const hands = game.record.hands;
    if ( hands.length === 0 ) {
      list.appendChild( Dom.el( "li" , { class: "chat-empty" , text: I18n.get( "chat.log_empty" ) } ) );
      return;
    }
    hands.forEach( function ( hand , index ) { list.appendChild( Record.handSection( view , hand , index === hands.length - 1 ) ); } );
  },
};
