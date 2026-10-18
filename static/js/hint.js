// The hint button, for players an admin switched hints on for. Three presses,
// each going one step further, so it teaches before it plays:
//
//   1. say what the suggestion is ("Try the Green 14");
//   2. show it: lift the card, set the bid, pick the five and trump;
//   3. do it.
//
// The suggestion is the bot's move for your seat (server-side, from your own
// cards only), so a hint never knows more than you do.
const Hint = {
  stage: 0,
  key: null,
  data: null,
  button: null,
  message: null,

  reset() {
    this.stage = 0;
    this.key = null;
    this.data = null;
  },

  // A hint is about one decision; a new turn starts the steps over.
  keyFor( view ) {
    const game = view.game;
    return [ game.hand_number , game.phase , game.turn , game.trick_number , game.trick.plays.length , game.bid ].join( ":" );
  },

  element( view ) {
    const key = this.keyFor( view );
    if ( key !== this.key ) {
      this.key = key;
      this.stage = 0;
      this.data = null;
    }
    this.button = Dom.el( "button" , { class: "hint-button small secondary" , attrs: { type: "button" } , on: { click: this.press.bind( this ) } } );
    this.message = Dom.el( "span" , { class: "hint-message" , attrs: { "aria-live": "polite" } } );
    this.paint();
    return Dom.el( "div" , { class: "hint-row" , children: [ this.button , this.message ] } );
  },

  paint() {
    Dom.text( this.button , I18n.get( [ "hint.button" , "hint.show" , "hint.do" ][ this.stage ] ) );
    Dom.text( this.message , this.data ? this.describe( this.data ) : "" );
  },

  describe( hint ) {
    switch ( hint.kind ) {
      case "bid":
        return I18n.format( "hint.bid" , { amount: hint.amount } );
      case "pass":
        return I18n.get( "hint.pass" );
      case "bury":
        return I18n.format( "hint.bury" , { cards: hint.cards.map( function ( id ) { return Cards.name( id ); } ).join( ", " ) , trump: I18n.get( "colors." + hint.trump ) } );
      case "play":
        return I18n.format( "hint.play" , { card: Cards.name( hint.card ) } );
      default:
        return "";
    }
  },

  async press() {
    const id = Table.id;
    if ( this.stage === 0 ) {
      try {
        this.data = await Api.hint( id );
      } catch ( error ) {
        Toast.error( error );
        return;
      }
      this.stage = 1;
      this.paint();
      return;
    }
    const hint = this.data;
    if ( this.stage === 1 ) {
      this.show( hint );
      this.stage = 2;
      this.paint();
      return;
    }
    this.reset();
    switch ( hint.kind ) {
      case "bid": Table.act( Api.bid( id , hint.amount ) ); break;
      case "pass": Table.act( Api.pass( id ) ); break;
      case "bury": Table.act( Api.bury( id , hint.cards , hint.trump ) ); break;
      case "play": Hand.play( hint.card ); break;
    }
  },

  show( hint ) {
    switch ( hint.kind ) {
      case "bid":
        Bidding.set( hint.amount );
        break;
      case "bury":
        Hand.bury = new Set( hint.cards );
        Hand.paint();
        Kitty.select( hint.trump );
        break;
      case "play":
        Hand.highlight( hint.card );
        break;
    }
  },
};
