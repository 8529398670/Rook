// The end-of-hand and end-of-game sheet: who bid what, whether they made it,
// both teams' counters and totals, and the buried kitty -- revealed now and
// only now. It waits for the last trick's animation before opening, so the
// hand ends on the table and not under a sheet.
const Summary = {
  dismissedHand: 0,
  handlers: null,

  init( handlers ) {
    this.handlers = handlers;
    Dom.get( "summary-continue" ).addEventListener( "click" , function () { Summary.handlers.continue(); } );
    Dom.get( "summary-lobby" ).addEventListener( "click" , function () { App.go( "#/" ); } );
  },

  reset() {
    this.dismissedHand = 0;
  },

  teamName( view , team ) {
    if ( view.you < 0 ) return I18n.format( "team.number" , { number: team + 1 } );
    return I18n.get( team === view.you % 2 ? "team.us" : "team.them" );
  },

  update( view ) {
    const game = view.game;
    const over = game && ( game.phase === "hand_over" || game.phase === "game_over" );
    if ( !over ) {
      if ( Sheet.isOpen( "sheet-summary" ) ) Sheet.close( true );
      return;
    }
    if ( Sheet.isOpen( "sheet-summary" ) ) {
      this.fill( view );
      return;
    }
    if ( this.dismissedHand === game.hand_number ) return;
    const hand = game.hand_number;
    Trick.queue.then( function () {
      const latest = Table.view;
      if ( !latest || !latest.game || latest.game.hand_number !== hand ) return;
      if ( latest.game.phase !== "hand_over" && latest.game.phase !== "game_over" ) return;
      Summary.open( latest );
    } );
  },

  open( view ) {
    this.fill( view );
    // The hand summary waits on you, so it only closes through its buttons.
    // The game-over summary can be put away to look at the table or chat.
    const final = view.game.phase === "game_over";
    Sheet.open( "sheet-summary" , {
      dismissible: final,
      focus: false,
      onClose: function () { Summary.dismissedHand = view.game.hand_number; },
    } );
  },

  fill( view ) {
    const game = view.game;
    const result = game.history[ game.history.length - 1 ];
    if ( !result ) return;
    const final = game.phase === "game_over";
    const us = view.you >= 0 ? view.you % 2 : 0;
    const them = 1 - us;
    const bidder = Seats.name( view.seats[ result.bidder ] );

    const art = Dom.get( "summary-art" );
    Dom.clear( art );
    art.hidden = !final;
    if ( final ) {
      art.appendChild( Art.birdSvg( "bird-art" ) );
      const won = view.you >= 0 && game.winner === us;
      Dom.text( Dom.get( "summary-title" ) , view.you < 0
        ? I18n.format( "summary.game_won_team" , { team: this.teamName( view , game.winner ) } )
        : I18n.get( won ? "summary.game_won_us" : "summary.game_won_them" ) );
    } else {
      Dom.text( Dom.get( "summary-title" ) , I18n.format( result.made ? "summary.made" : "summary.set" , { name: bidder , bid: result.bid } ) );
    }
    Dom.text( Dom.get( "summary-subtitle" ) , I18n.format( "summary.subtitle" , {
      hand: result.hand , name: bidder , bid: result.bid , trump: I18n.get( "colors." + result.trump ) ,
    } ) );

    const signed = function ( value ) { return value > 0 ? "+" + value : String( value ); };
    [ [ "us" , us ] , [ "them" , them ] ].forEach( function ( pair ) {
      const prefix = "summary-" + pair[ 0 ] + "-";
      Dom.text( Dom.get( prefix + "name" ) , Summary.teamName( view , pair[ 1 ] ) );
      Dom.text( Dom.get( prefix + "taken" ) , String( result.taken[ pair[ 1 ] ] ) );
      Dom.text( Dom.get( prefix + "scored" ) , signed( result.scored[ pair[ 1 ] ] ) );
      Dom.text( Dom.get( prefix + "total" ) , String( result.totals[ pair[ 1 ] ] ) );
    } );

    Dom.text( Dom.get( "summary-kitty-caption" ) , I18n.format( "summary.kitty" , {
      points: result.kitty_points , team: this.teamName( view , result.kitty_team ) ,
    } ) );
    const kitty = Dom.get( "summary-kitty" );
    Dom.clear( kitty );
    ( result.kitty || [] ).forEach( function ( id ) { kitty.appendChild( Cards.el( id , { class: "mini" } ) ); } );

    const proceed = Dom.get( "summary-continue" );
    const waiting = Dom.get( "summary-waiting" );
    Dom.show( Dom.get( "summary-lobby" ) , final );
    if ( final ) {
      Dom.text( proceed , I18n.get( "summary.rematch" ) );
      Dom.show( proceed , view.is_host && view.you >= 0 );
      proceed.disabled = view.status === "paused";
      Dom.text( waiting , view.is_host ? "" : I18n.get( "summary.rematch_host" ) );
      return;
    }
    Dom.text( proceed , I18n.get( "summary.next_hand" ) );
    Dom.show( proceed , view.you >= 0 );
    proceed.disabled = view.you < 0 || game.ready[ view.you ];
    const pending = [];
    game.ready.forEach( function ( ready , seat ) {
      if ( !ready ) pending.push( Seats.name( view.seats[ seat ] ) );
    } );
    Dom.text( waiting , pending.length && ( view.you < 0 || game.ready[ view.you ] ) ? I18n.format( "summary.waiting_for" , { names: pending.join( ", " ) } ) : "" );
  },
};
