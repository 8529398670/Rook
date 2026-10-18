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
};
