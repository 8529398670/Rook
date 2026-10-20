// Playing from the keyboard, at a table, when it is your move.
//
//   r1, g10, y14, b5   type a card: a color letter and its number
//   R                  the Rook (an R on its own)
//   13                 just the number, when only one card you could pick
//                      has it -- following red, the only red 13
//   Tab / →            the next card       Shift+Tab / ←   the previous one
//   Enter              play the selected card
//   Shift (alone), Esc clear the selection
//
// Typing selects as you go: "R" lifts the Rook, a "1" after it moves on to
// the red 1, and a "0" after that to the red 10. Tab and the arrows step through the cards you may play
// (the dimmed ones are skipped), in the order they sit in your hand.
//
// The color can be left off whenever it would not tell anything apart:
// "13" is enough when the 13 is the only card with that number among the
// ones you may play (or, burying, among your whole hand). When two could be
// meant, the hint above your hand names them and asks for the color.
//
// Burying the kitty works the same way, for five cards instead of one:
// each typed card goes in or out of the five ("r1 g10 R", with a space, a
// comma or the next color letter between them), Tab and the arrows move a
// ring along the hand and Space or Enter puts the ringed card in or out.
// Trump is "t" and a color letter: "tr" for red, "tb" for black. Enter with
// nothing typed or ringed then presses Done, and once more Confirm. Shift
// on its own clears the five.
//
// None of it applies while typing in a text field, while a sheet is open,
// or when it is not your move -- keys then do what they always do.
const CardKeys = {
  typed: "",
  chip: null,
  // A Shift pressed and released with nothing else in between clears the
  // selection. Shift+R (a capital R) and Shift+Tab are other keys, so they
  // never count as "Shift alone".
  shiftAlone: false,

  colorLetters: { r: "R" , g: "G" , y: "Y" , b: "B" },
  colorNames: { r: "red" , g: "green" , y: "yellow" , b: "black" },
  // In the kitty, "t" waits for the trump color's letter.
  trumpNext: false,

  init() {
    this.chip = Dom.get( "card-entry" );
    document.addEventListener( "keydown" , this.onKeyDown.bind( this ) );
    document.addEventListener( "keyup" , this.onKeyUp.bind( this ) );
    // Leaving the page with Shift held must not count as a lone press later.
    window.addEventListener( "blur" , function () { CardKeys.shiftAlone = false; } );
  },

  active() {
    return App.screen === Table && !Sheet.current && ( Hand.mode === "play" || Hand.mode === "bury" );
  },

  // Keys pressed with focus in the hand (a card clicked earlier) or on
  // nothing in particular are ours; anywhere else -- a text field, a menu
  // button -- they keep their usual meaning.
  ours( target ) {
    if ( !target || target === document.body || target === document.documentElement ) return true;
    if ( Table.typing( target ) ) return false;
    return Boolean( target.closest && target.closest( "#hand" ) );
  },

  onKeyDown( event ) {
    if ( event.key !== "Shift" ) this.shiftAlone = false;
    if ( event.ctrlKey || event.metaKey || event.altKey ) return;
    if ( event.key === "Shift" ) {
      this.shiftAlone = !event.repeat;
      return;
    }
    if ( !this.active() || !this.ours( event.target ) ) {
      this.clearTyped();
      return;
    }

    const key = event.key;
    let handled = true;
    if ( this.trumpNext && !this.colorLetters[ key.toLowerCase() ] ) {
      this.trumpNext = false;
      this.renderChip( null );
    }
    if ( key === "Tab" ) {
      this.step( event.shiftKey ? -1 : 1 );
    } else if ( key === "ArrowRight" ) {
      this.step( 1 );
    } else if ( key === "ArrowLeft" ) {
      this.step( -1 );
    } else if ( key === "Enter" ) {
      this.enter();
    } else if ( key === " " || key === "," ) {
      if ( Hand.mode === "bury" ) this.space();
      else if ( key === " " && this.typed === "" ) handled = false;
    } else if ( key === "Escape" ) {
      if ( this.typed === "" && !this.trumpNext && !this.hasSelection() ) handled = false;
      else this.clear();
    } else if ( key === "Backspace" ) {
      if ( this.typed === "" ) handled = false;
      else this.setTyped( this.typed.slice( 0 , -1 ) );
    } else if ( Hand.mode === "bury" && key.toLowerCase() === "t" ) {
      if ( this.typed !== "" ) this.commit();
      this.trumpNext = true;
      this.renderChip( null );
    } else if ( this.trumpNext && this.colorLetters[ key.toLowerCase() ] ) {
      this.trumpNext = false;
      this.renderChip( null );
      Kitty.select( this.colorNames[ key.toLowerCase() ] );
    } else if ( this.colorLetters[ key.toLowerCase() ] ) {
      // A new color letter starts a new card; in the kitty the one before
      // it goes in first.
      if ( Hand.mode === "bury" && this.typed !== "" ) this.commit();
      this.setTyped( key );
    } else if ( /^[0-9]$/.test( key ) ) {
      // A digit with nothing before it starts a number-only card ("13").
      const numberOnly = /^[0-9]*$/.test( this.typed );
      if ( this.typed.length < ( numberOnly ? 2 : 3 ) ) this.setTyped( this.typed + key );
    } else {
      handled = false;
    }
    if ( handled ) event.preventDefault();
  },

  onKeyUp( event ) {
    if ( event.key !== "Shift" ) return;
    const alone = this.shiftAlone;
    this.shiftAlone = false;
    if ( alone && this.active() && this.ours( event.target ) ) this.clear();
  },

  // "r1" -> "R1", "G10" -> "G10", "R" -> "ROOK", and "13" -> whichever
  // single card it can only mean. Null for anything that is not (yet) a card.
  resolve( typed ) {
    if ( /^[0-9]{1,2}$/.test( typed ) ) {
      const options = this.byNumber( typed );
      return options.length === 1 ? options[ 0 ] : null;
    }
    // An r on its own can only be the Rook, so either case will do.
    if ( typed === "R" || typed === "r" ) return "ROOK";
    const match = typed.match( /^([rgybRGYB])([0-9]{1,2})$/ );
    if ( !match ) return null;
    const number = Number( match[ 2 ] );
    if ( number < 1 || number > 14 || match[ 2 ].charAt( 0 ) === "0" ) return null;
    return this.colorLetters[ match[ 1 ].toLowerCase() ] + number;
  },

  // The cards a bare number could mean: the ones you may play with that
  // number, or every card in your hand with it while burying.
  byNumber( typed ) {
    const number = Number( typed );
    return this.stops().filter( function ( id ) {
      const card = Cards.parse( id );
      return !card.rook && card.number === number;
    } );
  },

  inHand( id ) {
    return Boolean( id ) && Hand.nodes.has( id ) && !Hand.launching.has( id );
  },

  setTyped( text ) {
    this.typed = text;
    const id = this.resolve( text );
    // As you type: lift the card in play, ring it in the kitty. A card you
    // cannot play is not lifted -- you may still be typing ("r1" on the way
    // to "r14") -- and Enter says why.
    if ( Hand.mode === "play" ) {
      Hand.selected = this.inHand( id ) && Hand.legal.has( id ) ? id : null;
    } else {
      Hand.cursor = this.inHand( id ) ? id : null;
    }
    Hand.paint();
    this.renderChip( id );
  },

  clearTyped() {
    if ( this.typed === "" && !this.trumpNext ) return;
    this.typed = "";
    this.trumpNext = false;
    this.renderChip( null );
  },

  hasSelection() {
    return Hand.mode === "play" ? Boolean( Hand.selected ) : Boolean( Hand.cursor ) || Hand.bury.size > 0;
  },

  // Shift alone, or Escape: nothing typed, nothing selected.
  clear() {
    this.typed = "";
    this.trumpNext = false;
    Hand.selected = null;
    Hand.cursor = null;
    if ( Hand.mode === "bury" && Hand.bury.size > 0 ) {
      Hand.bury = new Set();
      Hand.handlers.buryChanged();
    }
    Hand.paint();
    this.renderChip( null );
  },

  renderChip( id ) {
    if ( !this.chip ) return;
    if ( this.trumpNext ) {
      Dom.text( this.chip , I18n.get( "card_keys.trump_prompt" ) );
      Dom.show( this.chip , true );
      return;
    }
    if ( this.typed === "" ) {
      Dom.show( this.chip , false );
      return;
    }
    let text = I18n.format( "card_keys.typing" , { typed: this.typed } );
    const options = /^[0-9]+$/.test( this.typed ) ? this.byNumber( this.typed ) : [];
    if ( options.length > 1 ) text = I18n.format( "card_keys.which" , { typed: this.typed , cards: this.list( options ) } );
    else if ( id && this.inHand( id ) ) text = I18n.format( "card_keys.preview" , { typed: this.typed , card: Cards.name( id ) } );
    else if ( id ) text = I18n.format( "card_keys.preview_missing" , { typed: this.typed , card: Cards.name( id ) } );
    Dom.text( this.chip , text );
    Dom.show( this.chip , true );
  },

  // The cards Tab and the arrows walk through: the ones you may play, or
  // every card when burying.
  stops() {
    return Hand.order.filter( function ( id ) {
      return Hand.mode === "bury" || Hand.legal.has( id );
    } );
  },

  step( direction ) {
    this.clearTyped();
    const stops = this.stops();
    if ( stops.length === 0 ) return;
    const current = Hand.mode === "play" ? Hand.selected : Hand.cursor;
    let index = stops.indexOf( current );
    if ( index < 0 ) index = direction > 0 ? 0 : stops.length - 1;
    else index = ( index + direction + stops.length ) % stops.length;
    if ( Hand.mode === "play" ) Hand.selected = stops[ index ];
    else Hand.cursor = stops[ index ];
    // A card clicked earlier keeps keyboard focus, and Enter on it would
    // play that one rather than the one now selected.
    const focused = document.activeElement;
    if ( focused && focused.closest && focused.closest( "#hand" ) ) focused.blur();
    Hand.paint();
  },

  // A finished card in the kitty: in or out of the five.
  commit() {
    const typed = this.typed;
    const id = this.resolve( typed );
    this.typed = "";
    Hand.cursor = null;
    this.renderChip( null );
    if ( !this.check( typed , id ) ) {
      Hand.paint();
      return;
    }
    Hand.tap( id );
  },

  // Says what is wrong with a typed card, if anything.
  check( typed , id ) {
    if ( !id && /^[0-9]+$/.test( typed ) ) {
      const options = this.byNumber( typed );
      if ( options.length > 1 ) Toast.show( I18n.format( "card_keys.which" , { typed: typed , cards: this.list( options ) } ) );
      else Toast.show( I18n.format( Hand.mode === "play" ? "card_keys.no_number_play" : "card_keys.no_number" , { number: typed } ) );
      return false;
    }
    if ( !id ) {
      Toast.show( I18n.format( "card_keys.unknown" , { typed: typed } ) );
      return false;
    }
    if ( !this.inHand( id ) ) {
      Toast.show( I18n.format( "card_keys.not_in_hand" , { card: Cards.name( id ) } ) );
      return false;
    }
    return true;
  },

  // "Red 13 or Green 13", "Red 5, Black 5 or Green 5".
  list( ids ) {
    const names = ids.map( function ( id ) { return Cards.name( id ); } );
    if ( names.length < 2 ) return names.join( "" );
    return I18n.format( "card_keys.list" , { first: names.slice( 0 , -1 ).join( ", " ) , last: names[ names.length - 1 ] } );
  },

  space() {
    if ( this.typed !== "" ) {
      this.commit();
      return;
    }
    if ( Hand.cursor ) Hand.tap( Hand.cursor );
  },

  enter() {
    if ( Hand.mode === "bury" ) {
      if ( this.typed !== "" || Hand.cursor ) {
        this.space();
        Hand.cursor = null;
        Hand.paint();
        return;
      }
      this.finishKitty();
      return;
    }

    const typed = this.typed;
    if ( typed !== "" ) {
      const id = this.resolve( typed );
      this.typed = "";
      this.renderChip( null );
      if ( !this.check( typed , id ) ) return;
      // Hand.tap explains a card you cannot play (and shakes it); a card
      // you can is played at once.
      if ( !Hand.legal.has( id ) ) {
        Hand.tap( id );
        return;
      }
      Hand.selected = id;
      Hand.play( id );
      return;
    }
    if ( Hand.selected ) Hand.play( Hand.selected );
  },

  // Enter with the five chosen: Done, and then Confirm -- the same two
  // steps as the buttons, so nothing is buried without being asked.
  finishKitty() {
    if ( Kitty.confirming ) {
      Kitty.submit();
      return;
    }
    if ( Hand.bury.size !== 5 ) {
      Toast.show( I18n.format( "kitty.choose" , { count: Hand.bury.size } ) );
      return;
    }
    if ( !Kitty.trump ) {
      Toast.show( I18n.get( "kitty.pick_trump" ) );
      return;
    }
    Kitty.done.click();
  },
};
