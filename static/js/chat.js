// Table chat and the game log, two tabs of one sheet. Chat lines are plain
// text from other players and are only ever set with textContent. The log is
// the game record, drawn by record.js; eventText below still turns single
// events into sentences for the screen reader.
const Chat = {
  tab: "chat",
  seenChat: 0,
  primed: false,
  handlers: null,

  init( handlers ) {
    this.handlers = handlers;
    Dom.all( "[data-chat-tab]" ).forEach( function ( button ) {
      button.addEventListener( "click" , function () {
        Chat.tab = button.getAttribute( "data-chat-tab" );
        Chat.render( Table.view );
      } );
    } );
    Dom.get( "chat-form" ).addEventListener( "submit" , function ( event ) {
      event.preventDefault();
      const input = Dom.get( "chat-input" );
      const text = input.value.trim();
      if ( !text ) return;
      input.value = "";
      Chat.handlers.send( text );
    } );
  },

  reset() {
    this.seenChat = 0;
    this.primed = false;
    this.tab = "chat";
    Record.reset();
  },

  open() {
    Sheet.open( "sheet-chat" , { focus: false } );
    this.render( Table.view );
  },

  update( view ) {
    const latest = view.chat.length ? view.chat[ view.chat.length - 1 ].seq : 0;
    if ( !this.primed ) {
      // Arriving at a table should not light up everything said before you
      // got there.
      this.seenChat = latest;
      this.primed = true;
    }
    if ( Sheet.isOpen( "sheet-chat" ) ) this.render( view );
    const unread = latest > this.seenChat;
    Dom.show( Dom.get( "chat-unread" ) , unread );
    Dom.show( Dom.get( "menu-chat-unread" ) , unread );
  },

  render( view ) {
    if ( !view ) return;
    Dom.all( "[data-chat-tab]" ).forEach( function ( button ) {
      button.setAttribute( "aria-selected" , String( button.getAttribute( "data-chat-tab" ) === Chat.tab ) );
    } );
    const chatList = Dom.get( "chat-list" );
    const logList = Dom.get( "log-list" );
    chatList.hidden = this.tab !== "chat";
    logList.hidden = this.tab !== "log";
    Dom.show( Dom.get( "chat-form" ) , this.tab === "chat" && view.you >= 0 );

    if ( this.tab === "chat" ) {
      Dom.clear( chatList );
      if ( view.chat.length === 0 ) chatList.appendChild( Dom.el( "li" , { class: "chat-empty" , text: I18n.get( "chat.empty" ) } ) );
      view.chat.forEach( function ( line ) {
        chatList.appendChild( Dom.el( "li" , { class: "chat-line" + ( line.seat === view.you ? " is-mine" : "" ) , children: [
          Dom.el( "span" , { class: "chat-name" , text: line.name } ),
          Dom.el( "span" , { class: "chat-text" , text: line.text } ),
        ] } ) );
      } );
      if ( view.chat.length ) this.seenChat = view.chat[ view.chat.length - 1 ].seq;
      chatList.scrollTop = chatList.scrollHeight;
      Dom.show( Dom.get( "chat-unread" ) , false );
      Dom.show( Dom.get( "menu-chat-unread" ) , false );
    } else {
      // The log is the game record (record.js): hands, then tricks.
      if ( logList.childElementCount === 0 || !Record.data ) Record.render( view , logList );
      Record.refresh( view , logList );
    }
  },

  // One game event as a sentence, e.g. "Ana bids 85."
  eventText( view , event ) {
    const name = event.seat >= 0 ? Seats.name( view.seats[ event.seat ] ) : "";
    switch ( event.kind ) {
      case "deal":
        return I18n.format( "events.deal" , { hand: event.amount , name: name } );
      case "bid":
        return I18n.format( "events.bid" , { name: name , amount: event.amount } );
      case "pass":
        return I18n.format( "events.pass" , { name: name } );
      case "won_bid":
        return I18n.format( "events.won_bid" , { name: name , amount: event.amount } );
      case "trump":
        return I18n.format( "events.trump" , { name: name , color: I18n.get( "colors." + event.color ) } );
      case "play":
        return I18n.format( "events.play" , { name: name , card: Cards.name( event.card ) } );
      case "trick":
        return I18n.format( "events.trick" , { name: name , card: Cards.name( event.card ) , points: event.amount } );
      case "hand_over":
        return I18n.format( event.amount ? "events.made" : "events.set" , { name: name } );
      case "game_over":
        if ( view.you < 0 ) return I18n.format( "events.game_over_team" , { team: Summary.teamName( view , event.team ) } );
        return I18n.get( event.team === view.you % 2 ? "events.game_over_us" : "events.game_over_them" );
      default:
        return "";
    }
  },
};
