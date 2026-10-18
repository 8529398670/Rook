// The lobby: your tables, everyone else's, and the "New table" sheet. It
// stays live over its own event stream, so a seat filling up at another table
// shows here without a refresh.
const Lobby = {
  screenId: "screen-lobby",
  me: null,
  stream: null,
  options: null,
  // Tables we have already sent an invited player to, so backing out to the
  // lobby does not bounce them straight back in.
  redirected: {},

  init( me ) {
    this.me = me;
    Dom.get( "open-settings" ).addEventListener( "click" , function () { App.go( "#/settings" ); } );
    Dom.get( "new-table-button" ).addEventListener( "click" , this.openCreate.bind( this ) );
    Dom.get( "sheet-new-table" ).addEventListener( "submit" , this.onCreate.bind( this ) );

    Dom.all( "#sheet-new-table [data-option]" ).forEach( function ( control ) {
      const name = control.getAttribute( "data-option" );
      if ( control.tagName === "INPUT" ) {
        control.addEventListener( "change" , function () {
          Lobby.options[ name ] = control.checked;
        } );
        return;
      }
      control.addEventListener( "click" , function ( event ) {
        const button = event.target.closest( "button[data-value]" );
        if ( !button ) return;
        const raw = button.getAttribute( "data-value" );
        Lobby.options[ name ] = name === "play_to" ? Number( raw ) : raw;
        Lobby.renderOptions();
      } );
    } );
  },

  async enter() {
    try {
      this.render( await Api.listTables() );
    } catch ( error ) {
      Toast.error( error );
    }
    this.stream = Stream.open( "/api/lobby/events" , {
      lobby: function ( list ) { Lobby.render( list ); },
    } );
  },

  leave() {
    if ( this.stream ) this.stream.close();
    this.stream = null;
  },

  render( list ) {
    // Someone who just redeemed an invite lands here; their seat is waiting,
    // so take them to it.
    const held = list.find( function ( table ) {
      return table.you >= 0 && table.seats[ table.you ].pending;
    } );
    if ( held && !this.redirected[ held.id ] ) {
      this.redirected[ held.id ] = true;
      App.go( "#/t/" + held.id );
      return;
    }

    const mine = list.filter( function ( table ) { return table.you >= 0; } );
    const others = list.filter( function ( table ) { return table.you < 0; } );

    const myList = Dom.get( "my-tables" );
    Dom.clear( myList );
    mine.forEach( function ( table ) { myList.appendChild( Lobby.tableCard( table ) ); } );
    Dom.show( Dom.get( "my-tables-section" ) , mine.length > 0 );

    const openList = Dom.get( "open-tables" );
    Dom.clear( openList );
    others.forEach( function ( table ) { openList.appendChild( Lobby.tableCard( table ) ); } );
    Dom.show( Dom.get( "lobby-empty" ) , others.length === 0 && mine.length === 0 );
    // With tables of your own and nothing else open, an empty "Tables"
    // heading would only look broken.
    Dom.show( Dom.get( "open-tables-section" ) , others.length > 0 || mine.length === 0 );
  },

  statusText( table ) {
    const seated = table.seats.filter( function ( seat ) { return seat.kind !== ""; } ).length;
    switch ( table.status ) {
      case "waiting":
        return I18n.format( "lobby.status_waiting" , { seated: seated } );
      case "paused":
        return I18n.get( "lobby.status_paused" );
      case "held":
        return I18n.get( "lobby.status_held" );
      case "over":
        return I18n.get( "lobby.status_over" );
      default:
        return I18n.format( "lobby.status_playing" , { hand: table.hand_number , a: table.scores[ 0 ] , b: table.scores[ 1 ] } );
    }
  },

  tableCard( table ) {
    // A tiny compass of the four seats. Seat 0 is at the bottom and play runs
    // clockwise, the same orientation the table itself uses for a spectator.
    const compass = Dom.el( "div" , { class: "seat-compass" , attrs: { "aria-hidden": "true" } } );
    [ "s" , "w" , "n" , "e" ].forEach( function ( side , index ) {
      const seat = table.seats[ index ];
      const dot = Dom.el( "span" , {
        class: "compass-seat compass-" + side + " kind-" + ( seat.kind || "empty" ) + ( index === table.you ? " is-you" : "" ) + " team-" + ( index % 2 ),
        text: seat.kind === "" ? "" : ( seat.kind === "bot" ? I18n.get( "bots.badge" ) : Art.initials( Seats.name( seat ) ) ),
      } );
      if ( seat.kind === "human" ) Art.avatar( dot , seat );
      compass.appendChild( dot );
    } );

    const names = table.seats.filter( function ( seat ) { return seat.kind !== ""; } ).map( Seats.name ).join( ", " );

    const card = Dom.el( "button" , {
      class: "table-card status-" + table.status,
      attrs: { type: "button" },
      on: { click: function () { App.go( "#/t/" + table.id ); } },
      children: [
        compass,
        Dom.el( "div" , { class: "table-card-body" , children: [
          Dom.el( "div" , { class: "table-card-top" , children: [
            Dom.el( "span" , { class: "table-card-name" , text: table.name } ),
            Dom.el( "span" , { class: "status-chip status-" + table.status , text: I18n.get( "status." + table.status ) } ),
          ] } ),
          Dom.el( "div" , { class: "table-card-status" , text: this.statusText( table ) } ),
          Dom.el( "div" , { class: "table-card-names" , text: names } ),
          Dom.el( "ul" , { class: "chip-row" , children: this.ruleChips( table.options ) } ),
        ] } ),
      ],
    } );
    card.setAttribute( "aria-label" , I18n.format( "lobby.open_table" , { name: table.name } ) );
    return card;
  },

  // The rule chips shown on lobby cards, in the waiting room, and on the
  // rules sheet: only what differs from "just Rook", plus the target.
  ruleChips( options ) {
    const chips = [];
    chips.push( options.play_to === 0 ? I18n.get( "options.chip_single" ) : I18n.format( "options.chip_play_to" , { points: options.play_to } ) );
    chips.push( I18n.get( "options.chip_rook_" + options.rook_rank ) );
    if ( options.rook_anytime ) chips.push( I18n.get( "options.chip_anytime" ) );
    if ( !options.rook_first_trick ) chips.push( I18n.get( "options.chip_no_first_trick" ) );
    return chips.filter( Boolean ).map( function ( text ) {
      return Dom.el( "li" , { class: "chip" , text: text } );
    } );
  },

  openCreate() {
    this.options = { rook_rank: "high" , rook_anytime: false , rook_first_trick: true , play_to: 500 };
    Dom.get( "new-table-name" ).value = I18n.format( "create.default_name" , { name: this.me.display_name } );
    this.renderOptions();
    Sheet.open( "sheet-new-table" );
  },

  renderOptions() {
    const options = this.options;
    Dom.all( "#sheet-new-table [data-option]" ).forEach( function ( control ) {
      const name = control.getAttribute( "data-option" );
      if ( control.tagName === "INPUT" ) {
        control.checked = options[ name ] === true;
        return;
      }
      Dom.all( "button[data-value]" , control ).forEach( function ( button ) {
        button.setAttribute( "aria-pressed" , String( String( options[ name ] ) === button.getAttribute( "data-value" ) ) );
      } );
    } );
  },

  async onCreate( event ) {
    event.preventDefault();
    const name = Dom.get( "new-table-name" ).value.trim();
    try {
      const view = await Api.createTable( name , this.options );
      Sheet.close( true );
      App.go( "#/t/" + view.id );
    } catch ( error ) {
      Toast.error( error );
    }
  },
};
