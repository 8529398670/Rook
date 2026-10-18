// The bidding panel: a stepper instead of a number field, so a bid can only
// ever be a legal one -- a multiple of five, above the current bid, at most
// 180 -- and a phone never has to bring up a keyboard.
const Bidding = {
  amount: 0,
  floor: 0,
  refresh: null,

  // Set the stepper from outside -- the hint's "show me" step.
  set( amount ) {
    this.amount = amount;
    if ( this.refresh ) this.refresh();
  },

  render( tray , view , handlers ) {
    const game = view.game;
    if ( this.floor !== game.min_bid ) {
      this.floor = game.min_bid;
      this.amount = game.min_bid;
    }
    this.amount = Math.max( game.min_bid , Math.min( game.max_bid , this.amount ) );

    // "Your bid" and who holds it are in the table's bar; the panel is
    // just the controls.
    const output = Dom.el( "output" , { class: "bid-amount" , text: String( this.amount ) } );
    const bidButton = Dom.el( "button" , { class: "bid-button" , attrs: { type: "button" } } );
    const step = function ( delta ) {
      Bidding.amount = Math.max( game.min_bid , Math.min( game.max_bid , Bidding.amount + delta ) );
      refresh();
    };
    const minus = Dom.el( "button" , { class: "stepper secondary" , text: I18n.get( "bidding.minus" ) , attrs: { type: "button" , "aria-label": I18n.get( "bidding.lower" ) } , on: { click: function () { step( -5 ); } } } );
    const plus = Dom.el( "button" , { class: "stepper secondary" , text: I18n.get( "bidding.plus" ) , attrs: { type: "button" , "aria-label": I18n.get( "bidding.higher" ) } , on: { click: function () { step( 5 ); } } } );
    const refresh = function () {
      Dom.text( output , String( Bidding.amount ) );
      Dom.text( bidButton , I18n.format( "bidding.bid_button" , { amount: Bidding.amount } ) );
      minus.disabled = Bidding.amount <= game.min_bid;
      plus.disabled = Bidding.amount >= game.max_bid;
    };
    bidButton.addEventListener( "click" , function () { handlers.bid( Bidding.amount ); } );
    this.refresh = refresh;
    refresh();

    tray.appendChild( Dom.el( "div" , { class: "tray-panel bid-panel" , children: [
      Dom.el( "div" , { class: "bid-row" , attrs: { "data-hint-slot": "" } , children: [
        Dom.el( "div" , { class: "bid-stepper" , children: [ minus , output , plus ] } ),
        bidButton,
        Dom.el( "button" , { class: "secondary pass-button" , text: I18n.get( "bidding.pass" ) , attrs: { type: "button" } , on: { click: handlers.pass } } ),
      ] } ),
    ] } ) );
  },

  // The auction so far, in the middle of the table while the hand is bid for
  // and while the winner buries: who dealt, then every bid and pass in the
  // order they came, so nobody has to remember what a bubble said a moment
  // ago. Everyone sees it, people watching included.
  log( view ) {
    const box = Dom.get( "auction" );
    const game = view.game;
    const showing = Boolean( game ) && ( game.phase === "bidding" || game.phase === "kitty" ) &&
      view.status !== "waiting" && view.status !== "paused";
    box.hidden = !showing;
    if ( !showing ) {
      box.removeAttribute( "data-signature" );
      return;
    }
    // Redraw only when something in it changed; snapshots arrive often.
    const signature = JSON.stringify( [ game.hand_number , game.phase , game.turn , game.auction , Table.seatNames( view ) , view.perspective ] );
    if ( box.getAttribute( "data-signature" ) === signature ) return;
    box.setAttribute( "data-signature" , signature );

    const name = function ( seat ) { return Seats.name( view.seats[ seat ] ); };
    // The last bid made is the one standing.
    let high = -1;
    game.auction.forEach( function ( entry , index ) { if ( entry.amount > 0 ) high = index; } );
    const list = Dom.el( "ol" , { class: "auction-list" } );
    game.auction.forEach( function ( entry , index ) {
      const classes = [ "auction-row" , Seats.teamClass( view , entry.seat ) , entry.amount > 0 ? "" : "is-pass" , index === high ? "is-high" : "" ];
      list.appendChild( Dom.el( "li" , { class: classes.filter( Boolean ).join( " " ) , children: [
        Dom.el( "span" , { class: "auction-name" , text: name( entry.seat ) } ),
        Dom.el( "span" , { class: "auction-call" , text: entry.amount > 0 ? I18n.format( "bidding.bubble_bid" , { amount: entry.amount } ) : I18n.get( "bidding.passed" ) } ),
      ] } ) );
    } );
    const next = game.phase === "bidding"
      ? I18n.format( "bidding.log_turn" , { name: name( game.turn ) } )
      : I18n.format( "bidding.log_won" , { name: name( game.bidder ) , bid: game.bid } );
    list.appendChild( Dom.el( "li" , { class: "auction-row auction-next" , text: next } ) );

    Dom.clear( box );
    box.appendChild( Dom.el( "div" , { class: "auction-panel" , children: [
      Dom.el( "p" , { class: "auction-dealer" , text: I18n.format( "bidding.log_dealer" , { name: name( game.dealer ) } ) } ),
      list,
    ] } ) );
    list.scrollTop = list.scrollHeight;
  },
};
