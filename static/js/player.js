// The player menu: tap someone's name -- in "Online now", or on a line of
// the chat -- to message them, or to invite them to play beside you
// (partner) or against you (opponent). An invite holds a seat at the
// waiting table you are sitting at, when you may hand out its seats;
// otherwise it opens a new table for the two of you.
const Player = {
  person: null,

  init() {
    Dom.get( "player-sheet-message" ).addEventListener( "click" , function () {
      const person = Player.person;
      Sheet.close();
      if ( person ) Global.message( person );
    } );
    Dom.get( "player-sheet-partner" ).addEventListener( "click" , function () { Player.invite( "partner" ); } );
    Dom.get( "player-sheet-opponent" ).addEventListener( "click" , function () { Player.invite( "opponent" ); } );
  },

  // A name on a chat line that opens the menu.
  nameButton( person , text ) {
    return Dom.el( "button" , {
      class: "chat-name chat-name-button",
      text: text,
      attrs: { type: "button" , "aria-label": I18n.format( "player.open_label" , { name: text } ) },
      on: { click: function () { Player.open( person ); } },
    } );
  },

  // Someone on the online list, if they are on it: their current name and
  // avatar.
  online( id ) {
    const online = ( Global.view && Global.view.online ) || [];
    return online.find( function ( person ) { return person.id === id; } ) || null;
  },

  open( person ) {
    if ( !person || !Global.me || person.id === Global.me.id ) return;
    const live = this.online( person.id );
    this.person = live || person;
    Art.avatar( Dom.get( "player-sheet-avatar" ) , this.person );
    Dom.text( Dom.get( "player-sheet-name" ) , this.person.display_name );
    Dom.text( Dom.get( "player-sheet-status" ) , I18n.get( live ? "player.online" : "player.offline" ) );
    const here = this.hereTable();
    Dom.text( Dom.get( "player-sheet-where" ) , here ? I18n.format( "player.where_here" , { table: Table.view.name } ) : I18n.get( "player.where_new" ) );
    Sheet.open( "sheet-player" );
  },

  // The table an invite goes to: the one on screen, if you sit there, the
  // game has not started, and you may hand out its seats.
  hereTable() {
    const view = App.screen === Table ? Table.view : null;
    if ( !view || view.status !== "waiting" || view.you < 0 ) return "";
    const me = Global.me || {};
    const host = view.seats[ view.you ] && view.seats[ view.you ].host;
    return host || me.can_invite ? Table.id : "";
  },

  async invite( as ) {
    const person = this.person;
    if ( !person ) return;
    const here = this.hereTable();
    const name = I18n.format( "player.table_name" , { a: Global.me.display_name , b: person.display_name } );
    try {
      const result = await Api.invitePlayer( person.id , as , here , name );
      Sheet.close();
      Toast.show( I18n.format( as === "partner" ? "player.invited_partner" : "player.invited_opponent" , { name: person.display_name } ) );
      if ( !here ) App.go( "#/t/" + result.table_id );
    } catch ( error ) {
      Toast.error( error );
    }
  },
};
