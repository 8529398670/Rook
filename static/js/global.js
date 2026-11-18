// The global chat -- one conversation for everyone signed in, at any table
// or none -- the "online now" list, and your direct messages. Its stream
// opens once, at sign-in, and stays open on every screen: having it open is
// what puts you on the online list. It is drawn in three places: the lobby's
// side panel, a dock beside your cards, and the chat sheet. Each place shows
// one conversation at a time: "Everyone", or a direct-message tab of its
// own for each person you are talking to. Lines are plain text from other
// players and only ever set with textContent.
//
// The same snapshot carries the seats other players are holding for you
// (invitations to play), shown as notices that stay until answered.
const Global = {
  me: null,
  view: null,
  stream: null,
  seen: 0,
  primed: false,
  enabled: null,
  // The conversation showing: 0 for "Everyone", or the other person's id.
  active: 0,
  // The last line of each conversation already toasted, so each new
  // message pops up once; and how far each has been marked read.
  toasted: {},
  readSent: {},

  start( me ) {
    this.me = me;
    DMToast.init();
    [ [ "lobby-global-form" , "lobby-global-input" ] , [ "global-dock-form" , "global-dock-input" ] , [ "global-form" , "global-input" ] ].forEach( function ( pair ) {
      Dom.get( pair[ 0 ] ).addEventListener( "submit" , function ( event ) {
        event.preventDefault();
        Global.send( Dom.get( pair[ 1 ] ) );
      } );
    } );
    this.stream = Stream.open( "/api/global/events" , {
      global: function ( view ) { Global.update( view ); },
    } );
  },

  dms() {
    return this.view && this.view.dms ? this.view.dms : [];
  },

  dm( id ) {
    return this.dms().find( function ( dm ) { return dm.user_id === id; } ) || null;
  },

  // The "Everyone" conversation: on unless an admin switched it off or this
  // player hid it. The lobby still offers it while switched off, to say so.
  everyoneOn() {
    return Boolean( this.view && this.view.enabled && Chat.shown( "global" ) );
  },

  everyoneHere() {
    if ( App.screen === Lobby ) return Boolean( this.view ) && Chat.shown( "global" );
    return this.everyoneOn();
  },

  // Whether there is anything to show at all.
  available() {
    return Boolean( this.me && this.view );
  },

  // Whether the dock and the sheet's tabs show: "Everyone" is on, or a
  // direct-message conversation is open.
  shown() {
    return this.available() && ( this.everyoneOn() || this.dms().length > 0 );
  },

  // Keep the conversation showing to one that exists here.
  settle() {
    if ( this.active && !this.dm( this.active ) ) this.active = 0;
    if ( this.active === 0 && !this.everyoneHere() && this.dms().length ) this.active = this.dms()[ 0 ].user_id;
  },

  latest() {
    const chat = this.view ? this.view.chat : [];
    return chat.length ? chat[ chat.length - 1 ].seq : 0;
  },

  unread() {
    const everyone = this.everyoneOn() && this.latest() > this.seen;
    return everyone || this.dms().some( function ( dm ) { return dm.unread > 0; } );
  },

  update( view ) {
    this.view = view;
    if ( !this.primed ) {
      // Signing in should not light up everything said before you arrived.
      this.seen = this.latest();
    }
    this.notifyNew();
    this.primed = true;
    this.renderInvites();
    if ( this.enabled !== view.enabled ) {
      // Switched on or off by an admin: the docks and the sheet's tabs
      // change with it.
      this.enabled = view.enabled;
      Chat.refresh();
      return;
    }
    // A conversation opening or closing changes whether the dock shows.
    const docks = this.shown();
    if ( docks !== this.docksShown ) {
      this.docksShown = docks;
      Chat.refresh();
      return;
    }
    this.render();
  },

  // A direct message that arrives, once each. With no global chat in front
  // of you, it opens beside your cards (where there is room). Unless its
  // conversation was already showing, a quiet toast says who and what, and
  // the browser tab's title counts it while you are on another tab.
  // Everything is judged as things stood when it arrived.
  notifyNew() {
    const me = this.me;
    const arrived = [];
    this.dms().forEach( function ( dm ) {
      let last = null;
      dm.lines.forEach( function ( line ) { if ( line.user_id !== me.id ) last = line; } );
      if ( !last ) return;
      const before = Global.toasted[ dm.user_id ] || 0;
      Global.toasted[ dm.user_id ] = Math.max( before , last.seq );
      if ( !Global.primed || last.seq <= before || last.seq <= dm.read ) return;
      arrived.push( { dm: dm , line: last } );
    } );
    if ( !arrived.length ) return;
    const chatUp = this.onScreen();
    const away = document.hidden;
    arrived.forEach( function ( item ) {
      const dm = item.dm;
      if ( chatUp && Global.active === dm.user_id ) return;
      DMToast.show( { id: dm.user_id , display_name: dm.display_name , icon: dm.icon , color: dm.color } , item.line.text );
      if ( away ) Title.bump();
    } );
    if ( !chatUp ) this.reveal( arrived[ arrived.length - 1 ].dm.user_id );
  },

  // Put conversation id in front of the player without taking focus or
  // covering the cards: at a table on a wide screen, open the dock (on the
  // right, unless the table's chat is there). A narrow screen's chat is a
  // sheet over the cards, so it stays shut; the toast and the menu's dot
  // say enough. Elsewhere nothing moves.
  reveal( id ) {
    if ( App.screen !== Table || !Table.view ) return;
    if ( !Chat.dockWide || !Chat.dockWide.matches ) return;
    this.active = id;
    if ( Chat.dockedSide( "global" ) === "off" ) {
      Chat.setDockSide( "global" , Chat.dockedSide( "table" ) === "right" ? "left" : "right" );
    }
  },

  // Whether the conversation showing is in front of the player right now.
  onScreen() {
    if ( App.screen === Lobby ) return !Dom.get( "lobby-global" ).hidden;
    if ( App.screen === Table ) {
      return Chat.dockedSide( "global" ) !== "off" || ( Sheet.isOpen( "sheet-chat" ) && Chat.tab === "global" );
    }
    return false;
  },

  async send( input ) {
    const text = input.value.trim();
    if ( !text ) return;
    try {
      const view = this.active ? await Api.sendDM( this.active , text ) : await Api.globalChat( text );
      input.value = "";
      this.update( view );
    } catch ( error ) {
      Toast.error( error );
    }
  },

  // Show a conversation: 0 for "Everyone".
  select( id ) {
    this.active = id;
    if ( Sheet.isOpen( "sheet-chat" ) && Table.view ) {
      Chat.tab = "global";
      Chat.render( Table.view );
    }
    this.render();
  },

  // Open (or start) the conversation with person and put the cursor in
  // its box, wherever the chat shows on this screen.
  async message( person ) {
    try {
      this.update( await Api.openDM( person.id ) );
    } catch ( error ) {
      Toast.error( error );
      return;
    }
    this.active = person.id;
    if ( App.screen === Table && Table.view ) {
      // On a wide screen with this chat's dock closed, open the dock, on
      // the side the table's chat is not using.
      if ( Chat.dockWide && Chat.dockWide.matches && Chat.dockedSide( "global" ) === "off" ) {
        Chat.setDockSide( "global" , Chat.dockedSide( "table" ) === "right" ? "left" : "right" );
      }
      if ( Chat.dockedSide( "global" ) !== "off" ) {
        this.render();
        Dom.get( "global-dock-input" ).focus();
      } else {
        Chat.tab = "global";
        Chat.open();
        Dom.get( "global-input" ).focus();
      }
      return;
    }
    if ( App.screen !== Lobby ) App.go( "#/" );
    this.render();
    const panel = Dom.get( "lobby-global" );
    panel.scrollIntoView( { block: "nearest" , behavior: Motion.reduced() ? "auto" : "smooth" } );
    Dom.get( "lobby-global-input" ).focus( { preventScroll: true } );
  },

  async close( id ) {
    if ( this.active === id ) this.active = 0;
    try {
      this.update( await Api.closeDM( id ) );
    } catch ( error ) {
      Toast.error( error );
    }
  },

  // Draw every place the chat shows, and mark lines read where someone can
  // see them.
  render() {
    if ( !this.view ) return;
    this.settle();
    this.renderLobby();
    const dock = Chat.dockedSide( "global" ) !== "off";
    if ( dock ) {
      this.tabsInto( Dom.get( "dock-dm-tabs" ) , true );
      this.fill( Dom.get( "global-dock-list" ) );
      this.placeholder( Dom.get( "global-dock-input" ) );
    }
    const sheet = Sheet.isOpen( "sheet-chat" ) && Chat.tab === "global";
    this.tabsInto( Dom.get( "sheet-dm-tabs" ) , false );
    if ( sheet ) {
      this.fill( Dom.get( "global-list" ) );
      this.placeholder( Dom.get( "global-input" ) );
    }
    const lobby = App.screen === Lobby && !Dom.get( "lobby-global" ).hidden;
    if ( lobby || ( App.screen === Table && ( dock || sheet ) ) ) this.markRead();
    Chat.showUnread();
  },

  // The conversation showing has been seen.
  markRead() {
    if ( this.active === 0 ) {
      this.seen = this.latest();
      return;
    }
    const dm = this.dm( this.active );
    if ( !dm || dm.unread === 0 || !dm.lines.length ) return;
    const last = dm.lines[ dm.lines.length - 1 ].seq;
    if ( ( this.readSent[ dm.user_id ] || 0 ) >= last ) return;
    this.readSent[ dm.user_id ] = last;
    Api.readDM( dm.user_id , last ).then( function ( view ) { Global.update( view ); } , function () {
      delete Global.readSent[ dm.user_id ];
    } );
  },

  placeholder( input ) {
    const dm = this.active ? this.dm( this.active ) : null;
    const text = dm ? I18n.format( "dm.placeholder" , { name: dm.display_name } ) : I18n.get( "global.placeholder" );
    input.setAttribute( "placeholder" , text );
    input.setAttribute( "aria-label" , text );
  },

  // The conversation tabs. In the lobby and the dock they are a strip of
  // their own, "Everyone" first, shown once there is a conversation; in
  // the sheet they sit in the sheet's own tab row, whose "Everyone" button
  // chat.js draws.
  tabsInto( container , withEveryone ) {
    const dms = this.dms();
    const inSheet = !withEveryone;
    const selected = function ( id ) { return Global.active === id && ( !inSheet || Chat.tab === "global" ); };
    const key = dms.map( function ( dm ) { return dm.user_id + ":" + dm.display_name + ":" + dm.unread + ":" + dm.online; } ).join( "|" ) + "#" + ( inSheet ? Chat.tab : "" ) + this.active + ( this.everyoneHere() ? "e" : "" ) + ( this.everyoneOn() && this.latest() > this.seen ? "u" : "" );
    if ( container.getAttribute( "data-shown" ) === key ) return;
    container.setAttribute( "data-shown" , key );
    Dom.clear( container );
    if ( withEveryone ) {
      container.hidden = dms.length === 0;
      if ( container.hidden ) return;
      if ( this.everyoneHere() ) {
        const everyone = Dom.el( "button" , {
          class: "dm-tab-name dm-tab-everyone",
          text: I18n.get( "global.tab" ),
          attrs: { type: "button" , role: "tab" , "aria-selected": String( selected( 0 ) ) },
          on: { click: function () { Global.select( 0 ); } },
        } );
        if ( this.everyoneOn() && this.latest() > this.seen && !selected( 0 ) ) everyone.appendChild( Dom.el( "span" , { class: "badge-dot" } ) );
        container.appendChild( Dom.el( "span" , { class: "dm-tab" + ( selected( 0 ) ? " is-selected" : "" ) , children: [ everyone ] } ) );
      }
    }
    dms.forEach( function ( dm ) {
      const name = Dom.el( "button" , {
        class: "dm-tab-name",
        attrs: {
          type: "button" , role: "tab" , "aria-selected": String( selected( dm.user_id ) ),
          "aria-label": dm.unread > 0 ? I18n.format( "dm.unread" , { name: dm.display_name } ) : dm.display_name,
        },
        children: [
          Dom.el( "span" , { class: "dm-presence" + ( dm.online ? " is-online" : "" ) , attrs: { "aria-hidden": "true" } } ),
          Dom.el( "span" , { class: "dm-tab-label" , text: dm.display_name } ),
        ],
        on: { click: function () { Global.select( dm.user_id ); } },
      } );
      if ( dm.unread > 0 ) name.appendChild( Dom.el( "span" , { class: "badge-dot" } ) );
      const close = Dom.el( "button" , {
        class: "dm-tab-close",
        text: "×",
        attrs: { type: "button" , "aria-label": I18n.format( "dm.close" , { name: dm.display_name } ) , title: I18n.format( "dm.close" , { name: dm.display_name } ) },
        on: { click: function () { Global.close( dm.user_id ); } },
      } );
      container.appendChild( Dom.el( "span" , { class: "dm-tab" + ( selected( dm.user_id ) ? " is-selected" : "" ) , children: [ name , close ] } ) );
    } );
  },

  renderLobby() {
    const online = this.view.online;
    Dom.text( Dom.get( "online-heading" ) , I18n.format( "global.online_heading" , { count: online.length } ) );
    const list = Dom.get( "online-list" );
    Dom.clear( list );
    online.forEach( function ( person ) {
      const you = Boolean( Global.me && person.id === Global.me.id );
      const avatar = Dom.el( "span" , { class: "avatar avatar-small" , attrs: { "aria-hidden": "true" } } );
      Art.avatar( avatar , person );
      const name = Dom.el( "span" , { class: "online-name" , text: person.display_name } );
      // Anyone but you opens the player menu: message them, or invite them.
      const inner = you ? Dom.el( "span" , { class: "online-person-inner" , children: [ avatar , name ] } ) : Dom.el( "button" , {
        class: "online-person-inner",
        attrs: { type: "button" , "aria-label": I18n.format( "player.open_label" , { name: person.display_name } ) },
        children: [ avatar , name ],
        on: { click: function () { Player.open( person ); } },
      } );
      list.appendChild( Dom.el( "li" , { class: "online-person" + ( you ? " is-you" : "" ) , children: [ inner ] } ) );
    } );

    // Hidden by this player, with no conversation open: no panel. Switched
    // off by an admin: say so, rather than leaving an empty box.
    Dom.show( Dom.get( "lobby-global" ) , Chat.shown( "global" ) || this.dms().length > 0 );
    if ( App.screen === Lobby ) this.settle();
    this.tabsInto( Dom.get( "lobby-dm-tabs" ) , true );
    const off = this.active === 0 && !this.view.enabled;
    Dom.show( Dom.get( "lobby-global-off" ) , off );
    Dom.show( Dom.get( "lobby-global-list" ) , !off );
    Dom.show( Dom.get( "lobby-global-form" ) , !off );
    if ( !off ) {
      this.fill( Dom.get( "lobby-global-list" ) );
      this.placeholder( Dom.get( "lobby-global-input" ) );
    }
  },

  // The lines of the conversation showing.
  lines() {
    if ( !this.active ) return this.view.chat;
    const dm = this.dm( this.active );
    return dm ? dm.lines : [];
  },

  // Fill one list with the recent lines. Redrawn only when something in it
  // changed, so it does not jump while you scroll back through it.
  fill( list ) {
    const chat = this.lines();
    const shown = this.active + "|" + Chat.digest( chat );
    if ( list.getAttribute( "data-shown" ) === shown || Chat.busy( list ) ) return;
    list.setAttribute( "data-shown" , shown );
    Dom.clear( list );
    if ( chat.length === 0 ) {
      const dm = this.active ? this.dm( this.active ) : null;
      const empty = dm ? I18n.format( "dm.empty" , { name: dm.display_name } ) : I18n.get( "global.empty" );
      list.appendChild( Dom.el( "li" , { class: "chat-empty" , text: empty } ) );
    }
    chat.forEach( function ( line ) { list.appendChild( Global.lineEl( line ) ); } );
    list.scrollTop = list.scrollHeight;
  },

  // The sender's name with when they said it: a time today, a date and time
  // before that, the full date and time on hover.
  headEl( name , line ) {
    const date = line.at ? new Date( line.at ) : null;
    if ( !date || isNaN( date.getTime() ) || date.getFullYear() < 2000 ) return name;
    const stamp = Dom.el( "time" , {
      class: "chat-time",
      text: Player.when( date ),
      attrs: { datetime: date.toISOString() , title: date.toLocaleString( undefined , { dateStyle: "full" , timeStyle: "short" } ) },
    } );
    return Dom.el( "div" , { class: "chat-head" , children: [ name , stamp ] } );
  },

  lineEl( line ) {
    const mine = Boolean( this.me && line.user_id === this.me.id );
    const other = this.active;
    // In "Everyone", someone else's name opens the player menu.
    const name = !mine && !other && line.user_id ? Player.nameButton( { id: line.user_id , display_name: line.name , icon: line.icon , color: line.color } , line.name ) : Dom.el( "span" , { class: "chat-name" , text: line.name } );
    const item = Dom.el( "li" , { class: "chat-line" + ( mine ? " is-mine" : "" ) , children: [ this.headEl( name , line ) , Chat.textEl( line ) ] } );
    // Your own lines can be edited or deleted, the same way as at a table;
    // the owner may edit or delete anyone's in "Everyone".
    if ( other && mine ) {
      item.appendChild( Chat.ownActions( item , line , {
        save: async function ( seq , text ) { Global.update( await Api.editDM( other , seq , text ) ); },
        remove: async function ( seq ) { Global.update( await Api.deleteDM( other , seq ) ); },
        redraw: function () { Global.redraw(); },
      } ) );
    } else if ( !other && ( mine || ( this.me && this.me.owner ) ) ) {
      item.appendChild( Chat.ownActions( item , line , {
        save: async function ( seq , text ) { Global.update( await Api.editGlobalChat( seq , text ) ); },
        remove: async function ( seq ) { Global.update( await Api.deleteGlobalChat( seq ) ); },
        redraw: function () { Global.redraw(); },
      } ) );
    }
    return item;
  },

  // Draw every global list afresh, after an edit or delete.
  redraw() {
    Dom.all( "#lobby-global-list, #global-dock-list, #global-list" ).forEach( function ( list ) { list.removeAttribute( "data-shown" ); } );
    this.render();
  },

  // The invitations to play: one notice each, until it is answered here,
  // on the lobby card, or by opening the table.
  renderInvites() {
    const box = Dom.get( "invite-notices" );
    const invites = ( this.view && this.view.invites ) || [];
    const key = invites.map( function ( invite ) { return invite.table_id + ":" + invite.seat + ":" + invite.as; } ).join( "|" );
    if ( box.getAttribute( "data-shown" ) === key ) return;
    box.setAttribute( "data-shown" , key );
    Dom.clear( box );
    invites.forEach( function ( invite ) {
      const params = { name: invite.from_name , table: invite.table_name };
      const text = I18n.format( invite.as === "partner" ? "play_invite.as_partner" : invite.as === "opponent" ? "play_invite.as_opponent" : "play_invite.plain" , params );
      const avatar = Dom.el( "span" , { class: "avatar avatar-small" , attrs: { "aria-hidden": "true" } } );
      const from = ( Global.view.online || [] ).find( function ( person ) { return person.id === invite.from_id; } );
      Art.avatar( avatar , from || { display_name: invite.from_name } );
      const card = Dom.el( "div" , { class: "invite-notice" , attrs: { role: "alert" } , children: [
        avatar,
        Dom.el( "p" , { class: "invite-notice-text" , text: text } ),
        Dom.el( "div" , { class: "invite-notice-actions" , children: [
          Dom.el( "button" , { class: "small secondary" , text: I18n.get( "play_invite.decline" ) , attrs: { type: "button" } , on: { click: function () { Global.decline( invite , card ); } } } ),
          Dom.el( "button" , { class: "small" , text: I18n.get( "play_invite.join" ) , attrs: { type: "button" } , on: { click: function () {
            card.remove();
            App.go( "#/t/" + invite.table_id );
          } } } ),
        ] } ),
      ] } );
      box.appendChild( card );
    } );
  },

  async decline( invite , card ) {
    card.remove();
    try {
      await Api.leave( invite.table_id );
      Toast.show( I18n.get( "lobby.declined" ) );
    } catch ( error ) {
      Toast.error( error );
    }
  },
};
