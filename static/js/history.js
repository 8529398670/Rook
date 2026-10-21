// The admin's move history: every bid, burial and card of the game, by hand
// and by trick.
// Tapping a move puts the table back to just after it (and holds it, so
// nothing moves until Resume); Undo and Redo on the strip step one move.
const History = {
  async open() {
    await this.refresh();
    Sheet.open( "sheet-history" , { focus: false } );
    const current = Dom.get( "history-list" ).querySelector( "[aria-current='step']" );
    if ( current ) current.scrollIntoView( { block: "center" } );
  },

  async refresh() {
    let data = null;
    try {
      data = await Api.history( Table.id );
    } catch ( error ) {
      Toast.error( error );
      return;
    }
    const view = Table.view;
    const list = Dom.get( "history-list" );
    Dom.clear( list );
    const button = function ( index , text , extraClass ) {
      const element = Dom.el( "button" , {
        class: "history-move" + ( extraClass ? " " + extraClass : "" ),
        text: text,
        attrs: { type: "button" },
        on: { click: async function () {
          await Table.act( Api.resetTo( Table.id , index ) );
          History.refresh();
        } },
      } );
      if ( index === data.cursor ) element.setAttribute( "aria-current" , "step" );
      if ( index > data.cursor ) element.classList.add( "is-future" );
      return Dom.el( "li" , { children: [ element ] } );
    };

    // Moves are grouped the way the game is: a section per hand (with a
    // header that stays put while you scroll it), and inside it the
    // bidding, the kitty, each trick, and the wrap-up.
    list.appendChild( button( 0 , I18n.get( data.base ? "history.base" : "history.before_deal" ) , "history-start" ) );
    let hand = null;
    let group = null;
    let groupKey = "";
    const openHand = function ( number ) {
      const moves = Dom.el( "ol" , { class: "history-hand-moves" } );
      const section = Dom.el( "li" , { class: "history-hand-section" , children: [
        Dom.el( "div" , { class: "history-hand" , text: I18n.format( "history.hand" , { hand: number } ) } ),
        moves,
      ] } );
      list.appendChild( section );
      hand = { number: number , moves: moves };
      group = null;
      groupKey = "";
    };
    const openGroup = function ( key , title ) {
      const items = Dom.el( "ol" , { class: "history-group-moves" } );
      const heading = Dom.el( "div" , { class: "history-group-head" , text: title } );
      hand.moves.appendChild( Dom.el( "li" , { class: "history-group history-group-" + key.split( ":" )[ 0 ] , children: [ heading , items ] } ) );
      group = { items: items , heading: heading };
      groupKey = key;
    };

    data.entries.forEach( function ( entry ) {
      // A "ready for the next hand" belongs to the hand that just ended.
      if ( !hand || ( entry.hand !== hand.number && entry.kind !== "continue" ) ) openHand( entry.hand );
      let key = "";
      let title = "";
      switch ( entry.kind ) {
        case "start": case "bid": case "pass": case "reshuffle":
          key = "bidding"; title = I18n.get( "history.bidding" ); break;
        case "bury":
          key = "kitty"; title = I18n.get( "history.kitty" ); break;
        case "play":
          key = "trick:" + entry.trick; title = I18n.format( "history.trick" , { number: entry.trick } ); break;
        default:
          key = "wrap"; title = I18n.get( "history.wrap_up" );
      }
      if ( key !== groupKey ) openGroup( key , title );
      group.items.appendChild( button( entry.index , History.describe( view , entry ) , entry.kind === "play" ? "" : "history-key" ) );
      if ( entry.kind === "play" && entry.winner >= 0 ) {
        // The card that finished the trick: name the winner in the header.
        Dom.text( group.heading , I18n.format( "history.trick_won" , {
          number: entry.trick , name: Seats.name( view.seats[ entry.winner ] ) , points: entry.points || 0 ,
        } ) );
        group.heading.classList.add( "is-done" );
      }
    } );
  },

  describe( view , entry ) {
    const name = entry.seat >= 0 ? Seats.name( view.seats[ entry.seat ] ) : "";
    switch ( entry.kind ) {
      case "start":
        return I18n.format( "history.start" , { name: Seats.name( view.seats[ entry.first ] ) , dealer: Seats.name( view.seats[ Seats.dealerOf( entry.first ) ] ) } );
      case "bid":
        return I18n.format( "events.bid" , { name: name , amount: entry.amount } );
      case "pass":
        return I18n.format( "events.pass" , { name: name } );
      case "bury":
        return I18n.format( "events.trump" , { name: name , color: I18n.get( "colors." + entry.trump ) } );
      case "play":
        return I18n.format( "events.play" , { name: name , card: Cards.name( entry.card ) } );
      case "continue":
        return I18n.format( "history.continue" , { name: name } );
      case "reshuffle":
        return I18n.format( "history.reshuffle" , { name: name } );
      default:
        return entry.kind;
    }
  },
};
