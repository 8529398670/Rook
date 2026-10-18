// The kitty panel, shown to whoever won the bid: pick five cards in your hand
// to bury, name trump, done. The cards themselves are picked in the hand
// (hand.js); this panel counts them and holds the trump choice.
const Kitty = {
  trump: null,
  hand: 0,
  count: null,
  done: null,

  render( tray , view , handlers ) {
    const game = view.game;
    if ( this.hand !== game.hand_number ) {
      this.hand = game.hand_number;
      this.trump = this.longestColor( game.hand );
    }

    const swatches = Dom.el( "div" , { class: "trump-picker" , attrs: { role: "radiogroup" , "aria-label": I18n.get( "kitty.trump_label" ) } } );
    [ "red" , "black" , "yellow" , "green" ].forEach( function ( color ) {
      const button = Dom.el( "button" , {
        class: "trump-swatch swatch-" + color,
        attrs: { type: "button" , role: "radio" , "aria-checked": String( Kitty.trump === color ) },
        children: [ Art.glyphIcon( color ) , Dom.el( "span" , { text: I18n.get( "colors." + color ) } ) ],
        on: { click: function () {
          Kitty.trump = color;
          Dom.all( ".trump-swatch" , swatches ).forEach( function ( other ) {
            other.setAttribute( "aria-checked" , String( other === button ) );
          } );
          Kitty.refresh();
        } },
      } );
      swatches.appendChild( button );
    } );

    this.count = Dom.el( "p" , { class: "tray-title" } );
    this.done = Dom.el( "button" , { class: "kitty-done" , text: I18n.get( "kitty.done" ) , attrs: { type: "button" } , on: { click: function () {
      handlers.bury( Array.from( Hand.bury ) , Kitty.trump );
    } } } );

    this.swatches = swatches;
    tray.appendChild( Dom.el( "div" , { class: "tray-panel kitty-panel" , children: [
      this.count,
      swatches,
      this.done,
    ] } ) );
    this.refresh();
  },

  // Choose trump from outside -- the hint's "show me" step.
  select( color ) {
    this.trump = color;
    if ( this.swatches ) {
      Dom.all( ".trump-swatch" , this.swatches ).forEach( function ( button ) {
        button.setAttribute( "aria-checked" , String( button.classList.contains( "swatch-" + color ) ) );
      } );
    }
    this.refresh();
  },

  refresh() {
    if ( !this.count ) return;
    Dom.text( this.count , I18n.format( "kitty.choose" , { count: Hand.bury.size } ) );
    this.done.disabled = Hand.bury.size !== 5 || !this.trump;
  },

  // A sensible default: the color you hold most of.
  longestColor( hand ) {
    const counts = { red: 0 , black: 0 , yellow: 0 , green: 0 };
    hand.forEach( function ( id ) {
      const card = Cards.parse( id );
      if ( !card.rook ) counts[ card.color ] += 1;
    } );
    return Object.keys( counts ).sort( function ( a , b ) { return counts[ b ] - counts[ a ]; } )[ 0 ];
  },
};
