// The kitty panel, shown to whoever won the bid: pick five cards in your hand
// to bury, name trump, done. The cards themselves are picked in the hand
// (hand.js); this panel counts them and holds the trump choice.
//
// Trump is never picked for you, and nothing is sent until you have seen
// your choice spelled out and confirmed it: "Done" turns the panel into a
// one-line question, and only its Confirm buries.
const Kitty = {
  trump: null,
  hand: 0,
  confirming: false,
  sending: false,
  handlers: null,
  panel: null,
  countLong: null,
  countShort: null,
  done: null,
  question: null,
  confirmButton: null,

  render( tray , view , handlers ) {
    const game = view.game;
    if ( this.hand !== game.hand_number ) {
      this.hand = game.hand_number;
      this.trump = null;
      this.confirming = false;
    }
    this.handlers = handlers;

    const swatches = Dom.el( "div" , { class: "trump-picker" , attrs: { role: "radiogroup" , "aria-label": I18n.get( "kitty.trump_label" ) } } );
    [ "red" , "black" , "yellow" , "green" ].forEach( function ( color ) {
      swatches.appendChild( Dom.el( "button" , {
        class: "trump-swatch swatch-" + color,
        attrs: { type: "button" , role: "radio" , "aria-checked": "false" },
        children: [ Art.glyphIcon( color ) , Dom.el( "span" , { text: I18n.get( "colors." + color ) } ) ],
        on: { click: function () { Kitty.select( color ); } },
      } ) );
    } );

    // The count, long where there is room and short on a phone.
    this.countLong = Dom.el( "span" , { class: "kitty-count-long" } );
    this.countShort = Dom.el( "span" , { class: "kitty-count-short" } );
    this.done = Dom.el( "button" , { class: "kitty-done" , text: I18n.get( "kitty.done" ) , attrs: { type: "button" } , on: { click: function () {
      if ( Hand.bury.size !== 5 || !Kitty.trump ) return;
      Kitty.confirming = true;
      Kitty.refresh();
      Kitty.confirmButton.focus();
    } } } );

    this.question = Dom.el( "p" , { class: "kitty-question" , attrs: { "aria-live": "polite" } } );
    this.confirmButton = Dom.el( "button" , { class: "kitty-confirm-button" , text: I18n.get( "kitty.confirm_button" ) , attrs: { type: "button" } , on: { click: function () { Kitty.submit(); } } } );
    const back = Dom.el( "button" , { class: "secondary" , text: I18n.get( "kitty.change" ) , attrs: { type: "button" } , on: { click: function () {
      Kitty.confirming = false;
      Kitty.refresh();
    } } } );

    this.swatches = swatches;
    this.panel = Dom.el( "div" , { class: "tray-panel kitty-panel" , children: [
      swatches,
      Dom.el( "div" , { class: "kitty-actions" , attrs: { "data-hint-slot": "" } , children: [
        Dom.el( "p" , { class: "kitty-count" , attrs: { "aria-live": "polite" } , children: [ this.countLong , this.countShort ] } ),
        this.done,
      ] } ),
      Dom.el( "div" , { class: "kitty-confirm" , children: [
        this.question,
        Dom.el( "div" , { class: "kitty-confirm-actions" , children: [ back , this.confirmButton ] } ),
      ] } ),
    ] } );
    tray.appendChild( this.panel );
    this.refresh();
  },

  // Choose trump: a swatch, or the hint's "show me" step. Either way it is
  // a change of mind, so any question already asked is withdrawn.
  select( color ) {
    this.trump = color;
    this.confirming = false;
    this.refresh();
  },

  async submit() {
    if ( this.sending || !this.confirming ) return;
    this.sending = true;
    this.confirmButton.disabled = true;
    const accepted = await this.handlers.bury( Array.from( Hand.bury ) , this.trump );
    this.sending = false;
    if ( !accepted ) {
      // Refused (say the connection dropped): back to picking, choices kept.
      this.confirming = false;
      this.refresh();
    }
  },

  refresh() {
    if ( !this.done ) return;
    const count = Hand.bury.size;
    // Changing the five while being asked goes back to picking.
    if ( count !== 5 ) this.confirming = false;
    const trump = this.trump;

    if ( this.swatches ) {
      Dom.all( ".trump-swatch" , this.swatches ).forEach( function ( button ) {
        button.setAttribute( "aria-checked" , String( button.classList.contains( "swatch-" + trump ) ) );
      } );
    }
    const needTrump = count === 5 && !trump;
    Dom.text( this.countLong , needTrump ? I18n.get( "kitty.pick_trump" ) : I18n.format( "kitty.choose" , { count: count } ) );
    Dom.text( this.countShort , needTrump ? I18n.get( "kitty.pick_trump_short" ) : I18n.format( "kitty.count" , { count: count } ) );
    this.done.disabled = count !== 5 || !trump;

    this.panel.classList.toggle( "is-confirming" , this.confirming );
    this.confirmButton.disabled = this.sending;
    Dom.clear( this.question );
    if ( this.confirming ) {
      this.question.appendChild( Art.glyphIcon( trump , "glyph trump-" + trump ) );
      this.question.appendChild( Dom.el( "span" , { text: I18n.format( "kitty.confirm" , { color: I18n.get( "colors." + trump ) } ) } ) );
    }
  },
};
