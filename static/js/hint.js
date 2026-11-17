// The hint button, for players an admin switched hints on for. Three presses,
// each going one step further, so it teaches before it plays:
//
//   1. say what the suggestion is ("Try the Green 14");
//   2. show it: lift the card, set the bid, pick the five and trump;
//   3. do it.
//
// The suggestion is the Expert bot's move for your seat, whatever the bots
// at the table are set to -- worked out on the
// server from your own cards and what has been played, so a hint never
// knows more than you could.
const Hint = {
  stage: 0,
  key: null,
  data: null,
  button: null,
  message: null,

  // The button lives in the bar (index.html) and the suggestion floats at
  // the bottom of the felt, over your own seat, where it covers nothing.
  init() {
    this.button = Dom.get( "table-hint" );
    this.message = Dom.get( "hint-message" );
    this.button.addEventListener( "click" , this.press.bind( this ) );
  },

  reset() {
    this.stage = 0;
    this.key = null;
    this.data = null;
    if ( this.button ) this.paint();
  },

  // A hint is about one decision; a new turn starts the steps over.
  keyFor( view ) {
    const game = view.game;
    return [ game.hand_number , game.phase , game.turn , game.trick_number , game.trick.plays.length , game.bid ].join( ":" );
  },

  // Every snapshot: the button shows for the whole hand once hints are on,
  // so the bar never shifts, and can be pressed on your own turn.
  update( view ) {
    const game = view.game;
    const shown = Boolean( view.can_hint && game && view.you >= 0 && !view.observing &&
      view.status !== "waiting" && [ "bidding" , "kitty" , "playing" ].indexOf( game.phase ) >= 0 );
    Dom.show( this.button , shown );
    const usable = shown && Table.isMyTurn( view ) && !( view.auto_play && game.phase === "playing" );
    this.button.disabled = !usable;
    const key = usable ? this.keyFor( view ) : null;
    if ( key !== this.key ) {
      this.key = key;
      this.stage = 0;
      this.data = null;
    }
    this.paint();
  },

  paint() {
    const label = I18n.get( [ "hint.button" , "hint.show" , "hint.do" ][ this.stage ] );
    Dom.text( this.button , label );
    this.button.classList.toggle( "is-armed" , this.stage > 0 );
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
