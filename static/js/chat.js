// Table chat, the global chat and the game log, three tabs of one sheet.
// Chat lines are plain text from other players and are only ever set with
// textContent. The global chat's lines come from global.js; the log is the
// game record, drawn by record.js; eventText below still turns single events
// into sentences for the screen reader.
const Chat = {
  tab: "chat",
  seenChat: 0,
  primed: false,
  handlers: null,

  // The docked chats: small chat windows in the hand's row, left or right
  // of your cards, on screens wide enough to spare the room -- one for the
  // table and one for the global chat, never both on one side. Chosen per
  // device, as is whether each chat shows at all, so they live in
  // localStorage (wrapped, like the theme: blocked storage just means the
  // choice lasts for this page view).
  dockKeys: { table: "rook.chat_dock" , global: "rook.global_dock" },
  shownKeys: { table: "rook.table_chat_shown" , global: "rook.global_chat_shown" },
  dockSides: [ "off" , "left" , "right" ],
  dockWide: window.matchMedia ? window.matchMedia( "(min-width: 1100px)" ) : null,
  dockControls: [],
  dockShown: null,
  dockState: null,
  unsavedPrefs: {},
  unread: false,

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

  pref( key ) {
    try {
      const value = window.localStorage.getItem( key );
      if ( value !== null ) return value;
    } catch ( error ) {
      // Storage refused: fall back to this page view's choice.
    }
    return this.unsavedPrefs[ key ] || null;
  },

  setPref( key , value ) {
    this.unsavedPrefs[ key ] = value;
    try {
      window.localStorage.setItem( key , value );
    } catch ( error ) {
      // Storage refused: the choice still applies for this page view.
    }
  },

  // Whether this player wants a chat ("table" or "global") at all.
  shown( kind ) {
    return this.pref( this.shownKeys[ kind ] ) !== "off";
  },

  setShown( kind , flag ) {
    this.setPref( this.shownKeys[ kind ] , flag ? "on" : "off" );
    this.refresh();
  },

  // The table's chat shows unless this player hid it or an admin switched
  // it off.
  tableChatOn( view ) {
    return Boolean( view ) && !view.chat_off && this.shown( "table" );
  },

  dockSide( kind ) {
    const side = this.pref( this.dockKeys[ kind ] );
    return side === "left" || side === "right" ? side : "off";
  },

  // Putting one chat on the side the other has moves the other across, so
  // a wide screen can have one left of your cards and one right.
  setDockSide( kind , side ) {
    const other = kind === "table" ? "global" : "table";
    if ( side !== "off" && this.dockSide( other ) === side ) {
      this.setPref( this.dockKeys[ other ] , side === "left" ? "right" : "left" );
    }
    this.setPref( this.dockKeys[ kind ] , side );
    this.refresh();
  },

  // Where a dock actually shows right now: nowhere on a narrow screen, or
  // when its chat is hidden or switched off.
  dockedSide( kind ) {
    if ( !this.dockWide || !this.dockWide.matches ) return "off";
    const on = kind === "table" ? this.tableChatOn( Table.view ) : Global.shown();
    return on ? this.dockSide( kind ) : "off";
  },

  docked() {
    return this.dockedSide( "table" ) !== "off";
  },

  // The stylesheet places each dock from its data-side, and narrows the
  // hand to make room whenever the screen has data-chat-dock; the hand lays
  // itself out again when its box changes. Only sets attributes -- refresh
  // redraws.
  applyDock() {
    const table = this.dockedSide( "table" );
    const global = this.dockedSide( "global" );
    const screen = Dom.get( "screen-table" );
    if ( table === "off" && global === "off" ) {
      screen.removeAttribute( "data-chat-dock" );
    } else {
      screen.setAttribute( "data-chat-dock" , "on" );
    }
    Dom.show( Dom.get( "menu-table-dock" ) , !( Table.view && Table.view.chat_off ) );
    Dom.show( Dom.get( "menu-global-dock" ) , Global.available() );
    Dom.get( "chat-dock" ).setAttribute( "data-side" , table );
    Dom.get( "global-dock" ).setAttribute( "data-side" , global );
    this.dockControls.forEach( function ( button ) {
      const kind = button.getAttribute( "data-kind" );
      button.setAttribute( "aria-checked" , String( button.getAttribute( "data-side" ) === Chat.dockSide( kind ) ) );
    } );
    Dom.all( "[data-chat-shown]" ).forEach( function ( input ) {
      input.checked = Chat.shown( input.getAttribute( "data-chat-shown" ) );
    } );
    const state = table + "|" + global;
    if ( state !== this.dockState ) {
      this.dockState = state;
      this.dockShown = null;
    }
  },

  // After any change to what shows where: place the docks, then redraw
  // the table's chat and the global chat.
  refresh() {
    this.applyDock();
    if ( Table.view ) this.update( Table.view );
    if ( Sheet.isOpen( "sheet-chat" ) && Table.view ) this.render( Table.view );
    Global.render();
  },

  // The dots on the menu button and the menu's chat row: anything new in
  // either chat.
  showUnread() {
    const any = this.unread || Global.unread();
    Dom.show( Dom.get( "chat-unread" ) , any );
    Dom.show( Dom.get( "menu-chat-unread" ) , any );
  },

  // Fill a [data-chat-dock-controls] group with Off / Left / Right, for the
  // chat its data-kind names (the table's by default).
  bindDock( container ) {
    const kind = container.getAttribute( "data-kind" ) || "table";
    Chat.dockSides.forEach( function ( side ) {
      const button = Dom.el( "button" , {
        text: I18n.get( "chat_dock." + side ),
        attrs: { type: "button" , role: "radio" , "data-side": side , "data-kind": kind },
        on: { click: function () { Chat.setDockSide( kind , side ); } },
      } );
      container.appendChild( button );
      Chat.dockControls.push( button );
    } );
  },

  init( handlers ) {
    this.handlers = handlers;
    Dom.all( "[data-chat-dock-controls]" ).forEach( this.bindDock );
    Dom.all( "[data-chat-shown]" ).forEach( function ( input ) {
      input.addEventListener( "change" , function () {
        Chat.setShown( input.getAttribute( "data-chat-shown" ) , input.checked );
      } );
    } );
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
      this.dockWide.addEventListener( "change" , function () { Chat.refresh(); } );
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
    this.applyDock();
    this.renderGuest( view );
    if ( Sheet.isOpen( "sheet-chat" ) ) this.render( view );
    if ( this.docked() ) this.renderDock( view );
    this.unread = this.tableChatOn( view ) && latest > this.seenChat;
    this.showUnread();
  },

  // The sheet's tabs that apply: each chat unless hidden or switched off,
  // and always the log.
  tabs( view ) {
    const tabs = [];
    if ( this.tableChatOn( view ) ) tabs.push( "chat" );
    if ( Global.shown() ) tabs.push( "global" );
    tabs.push( "log" );
    return tabs;
  },

  render( view ) {
    if ( !view ) return;
    const tabs = this.tabs( view );
    if ( tabs.indexOf( this.tab ) < 0 ) this.tab = tabs[ 0 ];
    Dom.all( "[data-chat-tab]" ).forEach( function ( button ) {
      const tab = button.getAttribute( "data-chat-tab" );
      button.hidden = tabs.indexOf( tab ) < 0;
      button.setAttribute( "aria-selected" , String( tab === Chat.tab ) );
    } );
    const chatList = Dom.get( "chat-list" );
    const logList = Dom.get( "log-list" );
    chatList.hidden = this.tab !== "chat";
    logList.hidden = this.tab !== "log";
    Dom.get( "global-list" ).hidden = this.tab !== "global";
    // Everyone at the table may chat: players, and anyone watching.
    Dom.show( Dom.get( "chat-form" ) , this.tab === "chat" );
    Dom.show( Dom.get( "global-form" ) , this.tab === "global" );
    if ( this.tab === "global" ) Global.render();
    Dom.all( "#sheet-chat [data-guest-as]" ).forEach( function ( line ) {
      if ( Chat.tab !== "chat" ) line.hidden = true;
    } );
    if ( this.tab === "chat" ) this.renderGuest( view );

    if ( this.tab === "chat" && !this.busy( chatList ) ) {
      Dom.clear( chatList );
      if ( view.chat.length === 0 ) chatList.appendChild( Dom.el( "li" , { class: "chat-empty" , text: I18n.get( "chat.empty" ) } ) );
      view.chat.forEach( function ( line ) { chatList.appendChild( Chat.lineEl( view , line ) ); } );
      if ( view.chat.length ) this.seenChat = view.chat[ view.chat.length - 1 ].seq;
      chatList.scrollTop = chatList.scrollHeight;
      this.unread = false;
      this.showUnread();
    } else if ( this.tab === "chat" ) {
      // Someone is editing a line: leave the list be until they finish.
    } else if ( this.tab === "log" ) {
      // The log is the game record (record.js): hands, then tricks.
      if ( logList.childElementCount === 0 || !Record.data ) Record.render( view , logList );
      Record.refresh( view , logList );
    }
  },

  // Whether a line is the viewer's own: sent from their account, or -- for
  // a line from before lines recorded that -- their seat's, or one sent
  // under their name from the rail.
  mine( view , line ) {
    if ( line.user_id ) return Boolean( Table.me && line.user_id === Table.me.id );
    if ( line.seat >= 0 ) return line.seat === view.you;
    if ( !line.watcher || view.you >= 0 ) return false;
    if ( line.guest ) return !view.signed_in && line.name === this.guestName();
    return Boolean( view.signed_in && Table.me && line.name === Table.me.display_name );
  },

  // Only lines sent from your own account can be edited or deleted. A
  // guest's have no account behind them, so they stay as written -- except
  // to the owner, who may edit or delete anyone's line.
  editable( line ) {
    if ( Table.me && Table.me.owner ) return true;
    return Boolean( line.user_id && Table.me && line.user_id === Table.me.id );
  },

  // What a list shows, as a string: changes when a line is added, edited or
  // deleted, so a redraw happens then and only then.
  digest( lines ) {
    return lines.map( function ( line ) { return line.seq + ( line.edited ? "*" : "" ) + ":" + line.text; } ).join( "\n" );
  },

  // A list where someone is editing a line or confirming a delete is left
  // alone, so a snapshot arriving does not wipe what they are typing.
  busy( list ) {
    return Boolean( list && list.querySelector( ".chat-line.is-editing" ) );
  },

  // The line's words, with "(edited)" after them once they were changed.
  textEl( line ) {
    const text = Dom.el( "span" , { class: "chat-text" , text: line.text } );
    if ( line.edited ) text.appendChild( Dom.el( "span" , { class: "chat-edited" , text: " " + I18n.get( "chat.edited" ) } ) );
    return text;
  },

  // Edit and delete on a line of your own, for the table chat and the
  // global chat alike. actions: save( seq , text ) and remove( seq ), which
  // reject on failure, and redraw() to draw the list afresh afterwards.
  ownActions( item , line , actions ) {
    const bar = Dom.el( "span" , { class: "chat-actions" } );
    const button = function ( key , text , run ) {
      return Dom.el( "button" , {
        class: "chat-action",
        text: text,
        attrs: { type: "button" , "aria-label": I18n.get( key ) , title: I18n.get( key ) },
        on: { click: run },
      } );
    };
    const settle = function () {
      item.classList.remove( "is-editing" );
      actions.redraw();
    };
    const attempt = async function ( run ) {
      try {
        await run();
      } catch ( error ) {
        Toast.error( error );
      }
      settle();
    };

    const startEdit = function () {
      const text = item.querySelector( ".chat-text" );
      const input = Dom.el( "input" , { class: "chat-edit-input" , attrs: { type: "text" , maxlength: "280" , enterkeyhint: "done" , "aria-label": I18n.get( "chat.edit_label" ) } } );
      input.value = line.text;
      let done = false;
      const finish = function ( save ) {
        if ( done ) return;
        done = true;
        const value = input.value.trim();
        if ( save && value && value !== line.text ) {
          attempt( function () { return actions.save( line.seq , value ); } );
        } else {
          settle();
        }
      };
      input.addEventListener( "keydown" , function ( event ) {
        if ( event.key === "Enter" ) {
          event.preventDefault();
          finish( true );
        } else if ( event.key === "Escape" ) {
          event.preventDefault();
          event.stopPropagation();
          finish( false );
        }
      } );
      input.addEventListener( "blur" , function () { finish( false ); } );
      item.classList.add( "is-editing" );
      text.replaceWith( input );
      bar.hidden = true;
      input.focus();
    };

    // Deleting asks once, in the line itself. Left unanswered, the question
    // goes away again rather than holding the list still for good.
    const askDelete = function () {
      item.classList.add( "is-editing" );
      Dom.clear( bar );
      const timer = window.setTimeout( settle , 6000 );
      bar.appendChild( Dom.el( "span" , { class: "chat-confirm" , text: I18n.get( "chat.delete_confirm" ) } ) );
      bar.appendChild( button( "chat.yes" , I18n.get( "chat.yes" ) , function () {
        window.clearTimeout( timer );
        attempt( function () { return actions.remove( line.seq ); } );
      } ) );
      bar.appendChild( button( "chat.no" , I18n.get( "chat.no" ) , function () {
        window.clearTimeout( timer );
        settle();
      } ) );
    };

    bar.appendChild( button( "chat.edit" , "✎" , startEdit ) );
    bar.appendChild( button( "chat.delete" , "🗑" , askDelete ) );
    return bar;
  },

  // Redraw the table chat everywhere it shows, after an edit or delete.
  redraw() {
    this.dockShown = null;
    if ( !Table.view ) return;
    if ( Sheet.isOpen( "sheet-chat" ) ) this.render( Table.view );
    if ( this.docked() ) this.renderDock( Table.view );
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
    const item = Dom.el( "li" , { class: "chat-line" + ( this.mine( view , line ) ? " is-mine" : "" ) + ( line.watcher ? " is-watcher" : "" ) , children: [
      name,
      this.textEl( line ),
    ] } );
    if ( this.editable( line ) ) {
      item.appendChild( this.ownActions( item , line , {
        save: async function ( seq , text ) { Table.apply( await Api.editChat( Table.id , seq , text ) ); },
        remove: async function ( seq ) { Table.apply( await Api.deleteChat( Table.id , seq ) ); },
        redraw: function () { Chat.redraw(); },
      } ) );
    }
    return item;
  },

  // The docked window shows the same lines as the chat sheet. It is redrawn
  // only when something in it changed, so typing is never interrupted and
  // the list does not jump while you scroll back through it.
  renderDock( view ) {
    const latest = view.chat.length ? view.chat[ view.chat.length - 1 ].seq : 0;
    const shown = latest + "|" + view.you + "|" + this.digest( view.chat );
    Dom.show( Dom.get( "chat-dock-form" ) , true );
    this.seenChat = Math.max( this.seenChat , latest );
    const list = Dom.get( "chat-dock-list" );
    if ( this.dockShown === shown || this.busy( list ) ) return;
    this.dockShown = shown;
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
      case "reshuffle":
        return I18n.format( "events.reshuffle" , { name: name , hand: event.amount } );
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
