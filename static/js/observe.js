// What someone watching gets instead of a hand to play: a choice of whose
// seat to watch from and every hand at once when an admin has allowed it.
// What is happening is in the table's bar, as for everyone.
const Observe = {
  init() {},

  renderTray( tray , view ) {
    // It must fit the tray's fixed slot: the seat picker (the chosen seat is
    // who you are watching from) and the every-hand switch, then one line
    // of explanation.
    const watched = Seats.name( view.seats[ view.perspective ] );
    const controls = Dom.el( "div" , { class: "observe-controls" } );
    const panel = Dom.el( "div" , { class: "tray-panel observe-panel" , children: [ controls ] } );

    // Whose seat to watch from.
    const seats = Dom.el( "div" , { class: "segmented observe-seats" , attrs: {
      role: "radiogroup" , "aria-label": I18n.get( "observe.choose_seat" ) , title: I18n.format( "observe.watching" , { name: watched } ) ,
    } } );
    view.seats.forEach( function ( seat , index ) {
      seats.appendChild( Dom.el( "button" , {
        text: Seats.name( seat ),
        attrs: { type: "button" , role: "radio" , "aria-checked": String( index === view.perspective ) },
        on: { click: function () { if ( index !== view.perspective ) Table.watchFrom( index , Table.allHands ); } },
      } ) );
    } );
    controls.appendChild( seats );

    if ( view.can_see_all ) {
      const toggle = Dom.el( "input" , { attrs: { type: "checkbox" , role: "switch" } } );
      toggle.checked = view.all_hands;
      toggle.addEventListener( "change" , function () { Table.watchFrom( view.perspective , toggle.checked ); } );
      controls.appendChild( Dom.el( "label" , { class: "switch-row observe-all" , children: [
        Dom.el( "span" , { class: "switch-title" , text: I18n.get( "observe.all_hands" ) } ),
        toggle,
      ] } ) );
    }
    // The explanations, most pressing first, as one line; the whole of it
    // is in the tooltip when it does not fit.
    const notes = [];
    if ( view.signed_in && view.my_request >= 0 ) notes.push( I18n.format( "takeover.waiting" , { name: Seats.name( view.seats[ view.my_request ] ) } ) );
    if ( !view.signed_in ) notes.push( I18n.get( "observe.anonymous" ) );
    // Someone who could still sit down is shown no hidden cards (the server
    // leaves them out); say why the hand area is empty.
    if ( view.hands_hidden ) notes.push( I18n.get( "observe.hands_hidden" ) );
    if ( view.signed_in && view.my_request < 0 && view.seats.some( function ( seat ) { return seat.kind === "bot"; } ) ) notes.push( I18n.get( "takeover.how" ) );
    if ( notes.length ) {
      const text = notes.join( " · " );
      panel.appendChild( Dom.el( "p" , { class: "tray-detail" + ( view.my_request >= 0 ? " takeover-waiting" : "" ) , text: text , attrs: { title: text } } ) );
    }
    tray.appendChild( panel );
  },

  // Every other seat's hand, face up, in a strip above the tray -- only when
  // the server sent them (an allowed watcher who asked).
  renderHands( view ) {
    const strip = Dom.get( "all-hands" );
    const hands = view.game && view.game.hands;
    strip.hidden = !hands;
    Dom.get( "felt" ).classList.toggle( "has-all-hands" , Boolean( hands ) );
    Dom.clear( strip );
    if ( !hands ) return;
    [ 1 , 2 , 3 ].forEach( function ( position ) {
      const seat = Seats.seatAt( view , position );
      const avatar = Dom.el( "span" , { class: "avatar avatar-small" , attrs: { "aria-hidden": "true" } } );
      Seats.avatar( avatar , view.seats[ seat ] );
      const cards = Dom.el( "div" , { class: "row-cards" } );
      hands[ seat ].forEach( function ( id ) { cards.appendChild( Cards.el( id , { class: "row-card" } ) ); } );
      strip.appendChild( Dom.el( "div" , { class: "hand-row" , children: [
        avatar,
        Dom.el( "span" , { class: "hand-row-name" , text: Seats.name( view.seats[ seat ] ) } ),
        cards,
      ] } ) );
    } );
  },
};
