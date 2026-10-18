// Table chat and the game log, two tabs of one sheet. Chat lines are plain
// text from other players and are only ever set with textContent. The log is
// the game record, drawn by record.js; eventText below still turns single
// events into sentences for the screen reader.
const Chat = {
  tab: "chat",
  seenChat: 0,
  primed: false,
  handlers: null,

  // The docked chat: a small chat window in the hand's row, left or right
  // of your cards, on screens wide enough to spare the room. Chosen per
  // device, so it lives in localStorage (wrapped, like the theme: blocked
  // storage just means "off").
  dockKey: "rook.chat_dock",
  dockSides: [ "off" , "left" , "right" ],
  dockWide: window.matchMedia ? window.matchMedia( "(min-width: 1100px)" ) : null,
  dockControls: [],
  dockShown: null,

  // Someone watching through a watch link without signing in chats under a
  // name they type, remembered per device like the dock side. editingName
  // is true while they are changing it.
  guestKey: "rook.guest_name",
  editingName: false,

  guestName() {
    try {
      return ( window.localStorage.getItem( this.guestKey ) || "" ).trim();
    } catch ( error ) {
      return this.unsavedGuestName || "";
    }
  },

  setGuestName( name ) {
    this.unsavedGuestName = name;
    try {
      window.localStorage.setItem( this.guestKey , name );
    } catch ( error ) {
      // Storage refused: the name still holds for this page view.
    }
  },

  // Show the name box, or "Chatting as Cleo · Change", to a guest; neither
  // to anyone signed in.
  renderGuest( view ) {
    const guest = Boolean( view && !view.signed_in );
    const name = this.guestName();
    const asking = guest && ( !name || this.editingName );
    Dom.all( "[data-guest-name]" ).forEach( function ( input ) {
      if ( asking && input.hidden && !input.value ) input.value = name;
      input.hidden = !asking;
    } );
    Dom.all( "[data-guest-as]" ).forEach( function ( line ) {
      line.hidden = !guest || asking;
      Dom.text( line.querySelector( "[data-guest-as-text]" ) , I18n.format( "chat.chatting_as" , { name: name } ) );
    } );
  },

  // Send what is typed in input. A guest's name comes from the form's name
  // box when it is showing; without one, nothing is sent and the box asks.
  submit( input ) {
    const text = input.value.trim();
    if ( !text ) return;
    let name = "";
    if ( Table.view && !Table.view.signed_in ) {
      const box = input.form.querySelector( "[data-guest-name]" );
      name = box && !box.hidden ? box.value.trim().replace( /\s+/g , " " ) : this.guestName();
      if ( !name ) {
        Toast.show( I18n.get( "chat.guest_name_needed" ) , "error" );
        if ( box ) box.focus();
        return;
      }
      if ( name !== this.guestName() ) this.setGuestName( name );
      this.editingName = false;
      this.renderGuest( Table.view );
    }
    input.value = "";
    Chat.handlers.send( text , name );
  },

  dockSide() {
    try {
      const side = window.localStorage.getItem( this.dockKey );
      return side === "left" || side === "right" ? side : "off";
    } catch ( error ) {
      return "off";
    }
  },

  setDockSide( side ) {
    try {
      window.localStorage.setItem( this.dockKey , side );
    } catch ( error ) {
      // Storage refused: the choice still applies for this page view.
    }
    this.applyDock();
  },

  docked() {
    return this.dockSide() !== "off" && Boolean( this.dockWide && this.dockWide.matches );
  },

  // The stylesheet places the dock (and narrows the hand to make room) from
  // this one attribute; the hand lays itself out again when its box changes.
  applyDock() {
    const side = this.dockSide();
    const screen = Dom.get( "screen-table" );
    if ( side === "off" ) {
      screen.removeAttribute( "data-chat-dock" );
    } else {
      screen.setAttribute( "data-chat-dock" , side );
    }
    this.dockControls.forEach( function ( button ) {
      button.setAttribute( "aria-checked" , String( button.getAttribute( "data-side" ) === side ) );
    } );
    this.dockShown = null;
    if ( Table.view ) this.update( Table.view );
  },

  // Fill a [data-chat-dock-controls] group with Off / Left / Right.
  bindDock( container ) {
    Chat.dockSides.forEach( function ( side ) {
      const button = Dom.el( "button" , {
        text: I18n.get( "chat_dock." + side ),
        attrs: { type: "button" , role: "radio" , "data-side": side },
        on: { click: function () { Chat.setDockSide( side ); } },
      } );
      container.appendChild( button );
      Chat.dockControls.push( button );
    } );
  },

  init( handlers ) {
    this.handlers = handlers;
    Dom.all( "[data-chat-dock-controls]" ).forEach( this.bindDock );
    Dom.get( "chat-dock-form" ).addEventListener( "submit" , function ( event ) {
      event.preventDefault();
      Chat.submit( Dom.get( "chat-dock-input" ) );
    } );
    Dom.all( "[data-guest-change]" ).forEach( function ( button ) {
      button.addEventListener( "click" , function () {
        Chat.editingName = true;
        Chat.renderGuest( Table.view );
        const box = button.closest( "[data-guest-as]" ).nextElementSibling.querySelector( "[data-guest-name]" );
        if ( box ) box.focus();
      } );
    } );
    if ( this.dockWide ) {
      this.dockWide.addEventListener( "change" , function () { Chat.applyDock(); } );
    }
    this.applyDock();
    Dom.all( "[data-chat-tab]" ).forEach( function ( button ) {
      button.addEventListener( "click" , function () {
        Chat.tab = button.getAttribute( "data-chat-tab" );
        Chat.render( Table.view );
      } );
    } );
    Dom.get( "chat-form" ).addEventListener( "submit" , function ( event ) {
      event.preventDefault();
      Chat.submit( Dom.get( "chat-input" ) );
    } );
  },

  reset() {
    this.dockShown = null;
    this.seenChat = 0;
    this.primed = false;
    this.tab = "chat";
    this.editingName = false;
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
    this.renderGuest( view );
    if ( Sheet.isOpen( "sheet-chat" ) ) this.render( view );
    if ( this.docked() ) this.renderDock( view );
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
    // Everyone at the table may chat: players, and anyone watching.
    Dom.show( Dom.get( "chat-form" ) , this.tab === "chat" );
    Dom.all( "#sheet-chat [data-guest-as]" ).forEach( function ( line ) {
      if ( Chat.tab !== "chat" ) line.hidden = true;
    } );
    if ( this.tab === "chat" ) this.renderGuest( view );

    if ( this.tab === "chat" ) {
      Dom.clear( chatList );
      if ( view.chat.length === 0 ) chatList.appendChild( Dom.el( "li" , { class: "chat-empty" , text: I18n.get( "chat.empty" ) } ) );
      view.chat.forEach( function ( line ) { chatList.appendChild( Chat.lineEl( view , line ) ); } );
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

  // Whether a line is the viewer's own: their seat's, or -- for someone
  // watching -- one sent under their name from the rail.
  mine( view , line ) {
    if ( line.seat >= 0 ) return line.seat === view.you;
    if ( !line.watcher || view.you >= 0 ) return false;
    if ( line.guest ) return !view.signed_in && line.name === this.guestName();
    return Boolean( view.signed_in && Table.me && line.name === Table.me.display_name );
  },

  lineEl( view , line ) {
    const name = Dom.el( "span" , { class: "chat-name" , text: line.name } );
    // Someone watching is marked as such, and a guest (not signed in) more
    // plainly still, so nobody can pass for a player by typing their name.
    if ( line.watcher ) {
      name.appendChild( Dom.el( "span" , {
        class: "chat-tag" + ( line.guest ? " is-guest" : "" ),
        text: I18n.get( line.guest ? "chat.guest_tag" : "chat.watching_tag" ),
      } ) );
    }
    return Dom.el( "li" , { class: "chat-line" + ( this.mine( view , line ) ? " is-mine" : "" ) + ( line.watcher ? " is-watcher" : "" ) , children: [
      name,
      Dom.el( "span" , { class: "chat-text" , text: line.text } ),
    ] } );
  },

  // The docked window shows the same lines as the chat sheet. It is redrawn
  // only when something in it changed, so typing is never interrupted and
  // the list does not jump while you scroll back through it.
  renderDock( view ) {
    const latest = view.chat.length ? view.chat[ view.chat.length - 1 ].seq : 0;
    const shown = latest + "|" + view.you + "|" + view.chat.length;
    Dom.show( Dom.get( "chat-dock-form" ) , true );
    this.seenChat = Math.max( this.seenChat , latest );
    if ( this.dockShown === shown ) return;
    this.dockShown = shown;
    const list = Dom.get( "chat-dock-list" );
    Dom.clear( list );
    if ( view.chat.length === 0 ) list.appendChild( Dom.el( "li" , { class: "chat-empty" , text: I18n.get( "chat.empty" ) } ) );
    view.chat.forEach( function ( line ) { list.appendChild( Chat.lineEl( view , line ) ); } );
    list.scrollTop = list.scrollHeight;
  },

  // One game event as a sentence, e.g. "Ana bids 85."
  eventText( view , event ) {
    const name = event.seat >= 0 ? Seats.name( view.seats[ event.seat ] ) : "";
    switch ( event.kind ) {
      case "deal":
        return I18n.format( "events.deal" , { hand: event.amount , name: name , dealer: Seats.name( view.seats[ Seats.dealerOf( event.seat ) ] ) } );
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
