// The four seat badges around the table. Positions are relative to you:
// you are always at the bottom, your partner across the top, and play runs
// clockwise -- so the next seat to act is on your left. Someone watching sees
// the seat they chose to watch from at the bottom.
const Seats = {
  positions: [ "bottom" , "left" , "top" , "right" ],
  elements: [],

  init( felt ) {
    this.elements = this.positions.map( function ( position ) {
      const element = Dom.el( "div" , { class: "seat pos-" + position , children: [
        Dom.el( "div" , { class: "seat-avatar" } ),
        Dom.el( "div" , { class: "seat-label" , children: [
          Dom.el( "span" , { class: "seat-name" } ),
          Dom.el( "span" , { class: "seat-note" } ),
        ] } ),
        Dom.el( "div" , { class: "seat-bubble" , attrs: { hidden: "" } } ),
        Dom.el( "div" , { class: "seat-hand" , attrs: { "aria-hidden": "true" } } ),
        Dom.el( "div" , { class: "seat-actions" } ),
      ] } );
      felt.appendChild( element );
      return element;
    } );
  },

  name( seat ) {
    if ( !seat || seat.kind === "" ) return I18n.get( "seats.empty" );
    if ( seat.kind === "bot" ) return I18n.get( "bots.name_" + seat.bot ) || I18n.get( "bots.name" );
    return seat.name;
  },

  base( view ) {
    return view.perspective >= 0 ? view.perspective : 0;
  },

  position( view , seat ) {
    return ( seat - this.base( view ) + 4 ) % 4;
  },

  seatAt( view , position ) {
    return ( this.base( view ) + position ) % 4;
  },

  element( view , seat ) {
    return this.elements[ this.position( view , seat ) ];
  },

  // "us" and "them" from where you sit; a spectator sees teams 1 and 2.
  teamClass( view , seat ) {
    return seat % 2 === this.base( view ) % 2 ? "team-us" : "team-them";
  },

  // A short note under the name: what this seat is up to, or why it is idle.
  note( view , seat , info ) {
    const game = view.game;
    if ( info.kind === "" ) return "";
    if ( info.pending ) return I18n.get( "seats.invited" );
    if ( info.kind === "human" && !info.online ) return I18n.get( "seats.away" );
    if ( info.host && view.status === "waiting" ) return I18n.get( "seats.host" );
    if ( game && game.phase === "hand_over" ) return game.ready[ seat ] ? I18n.get( "seats.ready" ) : "";
    return "";
  },

  bubble( view , seat ) {
    const game = view.game;
    if ( !game ) return null;
    if ( game.phase === "bidding" ) {
      if ( game.seats[ seat ].passed ) return { text: I18n.get( "bidding.pass" ) , kind: "pass" };
      if ( game.seats[ seat ].last_bid > 0 ) return { text: String( game.seats[ seat ].last_bid ) , kind: seat === game.high_bidder ? "high" : "bid" };
      return null;
    }
    if ( seat === game.bidder && ( game.phase === "kitty" || game.phase === "playing" ) ) {
      return { text: String( game.bid ) , kind: "contract" , trump: game.trump };
    }
    return null;
  },

  render( view , handlers ) {
    const game = view.game;
    for ( let position = 0; position < 4; position += 1 ) {
      const seat = this.seatAt( view , position );
      const info = view.seats[ seat ];
      const element = this.elements[ position ];
      const turn = game && game.turn === seat && [ "bidding" , "kitty" , "playing" ].indexOf( game.phase ) >= 0 && view.status === "playing";

      element.className = [
        "seat" , "pos-" + this.positions[ position ] , "kind-" + ( info.kind || "empty" ) , this.teamClass( view , seat ) ,
        turn ? "is-turn" : "" , seat === view.you ? "is-you" : "" ,
        info.kind === "human" && !info.online ? "is-away" : "" , info.pending ? "is-pending" : "" ,
        game && seat === game.bidder ? "is-bidder" : "" ,
      ].filter( Boolean ).join( " " );

      const avatar = element.querySelector( ".seat-avatar" );
      if ( info.kind === "human" ) {
        Art.avatar( avatar , info );
      } else {
        Dom.text( avatar , info.kind === "bot" ? I18n.get( "bots.badge" ) : "" );
        avatar.classList.remove( "has-icon" );
      }

      Dom.text( element.querySelector( ".seat-name" ) , seat === view.you ? I18n.format( "seats.you" , { name: this.name( info ) } ) : this.name( info ) );
      Dom.text( element.querySelector( ".seat-note" ) , this.note( view , seat , info ) );

      const bubble = element.querySelector( ".seat-bubble" );
      const content = this.bubble( view , seat );
      Dom.clear( bubble );
      bubble.hidden = !content;
      if ( content ) {
        bubble.className = "seat-bubble bubble-" + content.kind;
        bubble.appendChild( Dom.el( "span" , { text: content.text } ) );
        if ( content.trump ) bubble.appendChild( Art.glyphIcon( content.trump , "glyph trump-" + content.trump ) );
      }

      // Face-down cards for everyone but the bottom seat, so a glance shows
      // who is short.
      const hand = element.querySelector( ".seat-hand" );
      const count = game && seat !== view.perspective ? game.seats[ seat ].cards : 0;
      if ( hand.childElementCount !== Math.min( count , 4 ) || hand.getAttribute( "data-count" ) !== String( count ) ) {
        Dom.clear( hand );
        for ( let index = 0; index < Math.min( count , 4 ); index += 1 ) {
          const back = Cards.el( null , { class: "seat-back" } );
          Dom.vars( back , { i: index } );
          hand.appendChild( back );
        }
        if ( count > 0 ) hand.appendChild( Dom.el( "span" , { class: "seat-count" , text: String( count ) } ) );
        hand.setAttribute( "data-count" , String( count ) );
      }

      this.renderActions( view , seat , info , element.querySelector( ".seat-actions" ) , handlers );
    }
  },

  // Sit / add a bot / invite on an empty seat; remove on a bot. Only while
  // seats are allowed to change: before the deal, while paused, or after the
  // game is over. Someone signed in and watching may ask to take over a bot
  // at any time; the host decides.
  renderActions( view , seat , info , container , handlers ) {
    const open = view.status === "waiting" || view.status === "paused" || view.status === "over";
    const asking = view.my_request === seat;
    const canAsk = info.kind === "bot" && view.you < 0 && view.signed_in;
    // Rebuild only when the choices change. Snapshots arrive often (anyone
    // coming online sends one), and replacing a button between a finger
    // going down and coming up would swallow the tap.
    const signature = [ open , info.kind , view.you < 0 , view.you >= 0 , view.can_invite , canAsk , asking , view.my_request >= 0 ].join( "|" );
    if ( container.getAttribute( "data-signature" ) === signature ) return;
    container.setAttribute( "data-signature" , signature );
    Dom.clear( container );
    const button = function ( key , kind , run ) {
      return Dom.el( "button" , { class: "small " + kind , text: I18n.get( key ) , attrs: { type: "button" } , on: { click: function () { run( seat ); } } } );
    };
    if ( canAsk ) {
      if ( asking ) {
        container.appendChild( Dom.el( "span" , { class: "seat-asked" , text: I18n.get( "takeover.asked" ) } ) );
        container.appendChild( button( "takeover.cancel" , "ghost" , handlers.cancelRequest ) );
      } else {
        container.appendChild( button( "takeover.ask" , view.my_request >= 0 ? "secondary" : "" , handlers.requestSeat ) );
      }
      return;
    }
    if ( !open ) return;
    if ( info.kind === "" ) {
      if ( view.you < 0 ) container.appendChild( button( "seats.sit" , "" , handlers.sit ) );
      if ( view.you >= 0 ) container.appendChild( button( "seats.add_bot" , "secondary" , handlers.addBot ) );
      if ( view.can_invite ) container.appendChild( button( "seats.invite" , "secondary" , handlers.invite ) );
      return;
    }
    if ( info.kind === "bot" && view.you >= 0 ) {
      container.appendChild( button( "seats.remove_bot" , "ghost" , handlers.removeBot ) );
    }
  },

  rect( view , seat ) {
    const element = this.element( view , seat );
    return element ? element.querySelector( ".seat-avatar" ).getBoundingClientRect() : null;
  },
};
