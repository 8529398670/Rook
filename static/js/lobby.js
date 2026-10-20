// The lobby: your tables, everyone else's, and the "New table" sheet. It
// stays live over its own event stream, so a seat filling up at another table
// shows here without a refresh.
const Lobby = {
  screenId: "screen-lobby",
  me: null,
  stream: null,
  options: null,
  // Whether the lobby has drawn once since the app loaded. Only that first
  // look may send an invited player straight to their seat; after that they
  // choose from "Seats saved for you", so backing out is never a bounce.
  shown: false,

  init( me ) {
    this.me = me;
    Dom.get( "open-settings" ).addEventListener( "click" , function () { App.go( "#/settings" ); } );
    Dom.get( "open-games" ).addEventListener( "click" , function () { App.go( "#/games" ); } );
    Dom.get( "open-rules" ).addEventListener( "click" , function () { App.go( "#/rules" ); } );
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
        if ( name === "play_to" ) {
          // "Custom" opens a box to type the target into; the number is
          // read from it when the table is created.
          Lobby.customPlayTo = raw === "custom";
          if ( !Lobby.customPlayTo ) Lobby.options.play_to = Number( raw );
        } else {
          Lobby.options[ name ] = raw;
        }
        Lobby.renderOptions();
        if ( raw === "custom" ) Dom.get( "play-to-custom-input" ).focus();
      } );
    } );
  },

  async enter() {
    // Lines said while you were at a table count as read once you are back
    // here, where the global chat is in view.
    Global.render();
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
    // Someone who just redeemed an invite lands here; with exactly one seat
    // waiting for them, take them to it.
    const isHeld = function ( table ) { return table.you >= 0 && table.seats[ table.you ].pending; };
    const saved = list.filter( isHeld );
    const first = !this.shown;
    this.shown = true;
    if ( first && saved.length === 1 ) {
      App.go( "#/t/" + saved[ 0 ].id );
      return;
    }

    // Otherwise a seat someone saved for you gets its own section at the
    // top, with the choice to take it or give it up.
    const mine = list.filter( function ( table ) { return table.you >= 0 && !isHeld( table ); } );
    const others = list.filter( function ( table ) { return table.you < 0; } );

    const fill = function ( id , tables ) {
      const element = Dom.get( id );
      Dom.clear( element );
      tables.forEach( function ( table ) { element.appendChild( Lobby.tableCard( table ) ); } );
    };
    fill( "held-tables" , saved );
    fill( "my-tables" , mine );
    fill( "open-tables" , others );
    Dom.show( Dom.get( "held-tables-section" ) , saved.length > 0 );
    Dom.show( Dom.get( "my-tables-section" ) , mine.length > 0 );
    const anyOfYours = saved.length + mine.length > 0;
    Dom.show( Dom.get( "lobby-empty" ) , others.length === 0 && !anyOfYours );
    // With tables of your own and nothing else open, an empty "Tables"
    // heading would only look broken.
    Dom.show( Dom.get( "open-tables-section" ) , others.length > 0 || !anyOfYours );
  },

  // Seats you could sit in from here: empty chairs at a table you are not
  // at, while seats may change (a paused game takes a new player too).
  openSeats( table ) {
    if ( table.you >= 0 || [ "waiting" , "paused" , "over" ].indexOf( table.status ) < 0 ) return [];
    const seats = [];
    table.seats.forEach( function ( seat , index ) { if ( seat.kind === "" ) seats.push( index ); } );
    return seats;
  },

  async sit( table , seat ) {
    try {
      await Api.sit( table.id , seat );
      App.go( "#/t/" + table.id );
    } catch ( error ) {
      Toast.error( error );
    }
  },

  async decline( table ) {
    try {
      await Api.leave( table.id );
      Toast.show( I18n.get( "lobby.declined" ) );
      this.render( await Api.listTables() );
    } catch ( error ) {
      Toast.error( error );
    }
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
    const open = this.openSeats( table );
    // A tiny compass of the four seats. Seat 0 is at the bottom and play runs
    // clockwise, the same orientation the table itself uses for a spectator.
    // An open seat you could take is a button: tap it to sit there.
    const compass = Dom.el( "div" , { class: "seat-compass" } );
    [ "s" , "w" , "n" , "e" ].forEach( function ( side , index ) {
      const seat = table.seats[ index ];
      const className = "compass-seat compass-" + side + " kind-" + ( seat.kind || "empty" ) + ( index === table.you ? " is-you" : "" ) + " team-" + ( index % 2 );
      if ( open.indexOf( index ) >= 0 ) {
        compass.appendChild( Dom.el( "button" , {
          class: className + " is-open",
          text: "+",
          attrs: { type: "button" , "aria-label": I18n.format( "lobby.join_seat" , { name: table.name } ) , title: I18n.get( "seats.sit" ) },
          on: { click: function () { Lobby.sit( table , index ); } },
        } ) );
        return;
      }
      const dot = Dom.el( "span" , {
        class: className,
        attrs: { "aria-hidden": "true" },
        text: seat.kind === "" ? "" : ( seat.kind === "bot" ? I18n.get( "bots.badge" ) : Art.initials( Seats.name( seat ) ) ),
      } );
      if ( seat.kind === "human" ) Art.avatar( dot , seat );
      compass.appendChild( dot );
    } );

    // Who is playing, with computer players marked so they read as bots at
    // a glance rather than as four people.
    const names = Dom.el( "div" , { class: "table-card-names" } );
    const seated = table.seats.filter( function ( seat ) { return seat.kind !== ""; } );
    seated.forEach( function ( seat , index ) {
      const bot = seat.kind === "bot";
      // Commas between plain names; a pill is set apart by its own outline.
      if ( index > 0 ) names.appendChild( document.createTextNode( bot || seated[ index - 1 ].kind === "bot" ? " " : ", " ) );
      names.appendChild( Dom.el( "span" , {
        class: "table-card-player" + ( bot ? " is-bot" : "" ),
        text: bot ? I18n.get( "bots.badge" ) + " " + Seats.name( seat ) : Seats.name( seat ),
        attrs: bot ? { title: I18n.get( "lobby.bot_label" ) } : {},
      } ) );
      if ( bot ) names.appendChild( Dom.el( "span" , { class: "visually-hidden" , text: " (" + I18n.get( "lobby.bot_label" ) + ")" } ) );
    } );

    const main = Dom.el( "button" , {
      class: "table-card-main",
      attrs: { type: "button" , "aria-label": I18n.format( "lobby.open_table" , { name: table.name } ) },
      on: { click: function () { App.go( "#/t/" + table.id ); } },
      children: [
        Dom.el( "div" , { class: "table-card-body" , children: [
          Dom.el( "div" , { class: "table-card-top" , children: [
            Dom.el( "span" , { class: "table-card-name" , text: table.name } ),
            Dom.el( "span" , { class: "status-chip status-" + table.status , text: I18n.get( "status." + table.status ) } ),
          ] } ),
          Dom.el( "div" , { class: "table-card-status" , text: this.statusText( table ) } ),
          names,
          Dom.el( "ul" , { class: "chip-row" , children: this.ruleChips( table.options ) } ),
        ] } ),
      ],
    } );

    const actions = Dom.el( "div" , { class: "table-card-actions" } );
    const action = function ( key , kind , run ) {
      actions.appendChild( Dom.el( "button" , { class: "small " + kind , text: I18n.get( key ) , attrs: { type: "button" } , on: { click: run } } ) );
    };
    if ( table.you >= 0 && table.seats[ table.you ].pending ) {
      action( "lobby.decline" , "ghost" , function () { Lobby.decline( table ); } );
      action( "lobby.take_seat" , "" , function () { App.go( "#/t/" + table.id ); } );
    } else if ( open.length > 0 ) {
      action( "lobby.join" , "" , function () { Lobby.sit( table , open[ 0 ] ); } );
    }
    // Admins can start any game over, or remove the table, from here.
    if ( this.me && this.me.role === "admin" ) {
      if ( table.status !== "waiting" ) action( "lobby.restart" , "secondary" , function () { Table.confirmManage( "restart" , table.id ); } );
      action( "lobby.delete" , "secondary table-card-delete" , function () { Table.confirmManage( "delete" , table.id ); } );
    }

    const children = [ compass , main ];
    if ( actions.childElementCount > 0 ) children.push( actions );
    return Dom.el( "div" , { class: "table-card status-" + table.status , children: children } );
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
    this.options = { rook_rank: "high" , play_to: 500 };
    this.customPlayTo = false;
    Dom.get( "play-to-custom-input" ).value = "";
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
      const chosen = name === "play_to" && Lobby.customPlayTo ? "custom" : String( options[ name ] );
      Dom.all( "button[data-value]" , control ).forEach( function ( button ) {
        button.setAttribute( "aria-pressed" , String( chosen === button.getAttribute( "data-value" ) ) );
      } );
    } );
    Dom.show( Dom.get( "play-to-custom" ) , this.customPlayTo );
    Dom.get( "play-to-custom-input" ).disabled = !this.customPlayTo;
  },

  async onCreate( event ) {
    event.preventDefault();
    const name = Dom.get( "new-table-name" ).value.trim();
    // The browser has already held the form back unless the box holds a
    // whole number in range (its min, max and step).
    if ( this.customPlayTo ) this.options.play_to = Number( Dom.get( "play-to-custom-input" ).value );
    try {
      const view = await Api.createTable( name , this.options );
      Sheet.close( true );
      App.go( "#/t/" + view.id );
    } catch ( error ) {
      Toast.error( error );
    }
  },
};
