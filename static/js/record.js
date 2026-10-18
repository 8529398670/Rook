// The game log tab: the whole game so far, one section per hand, each hand
// with its auction, trump and every trick laid out as four small cards. The
// server builds it by replaying the move history (GET .../record), so it
// covers the whole game, not just the last few events a snapshot carries.
//
// It is fetched when the tab opens and again whenever the table changes
// while it is open. Which hands are folded is remembered for the visit, so a
// refresh never snaps a section you opened (or closed) back.
const Record = {
  data: null,
  version: -1,
  loading: false,
  folded: new Map(), // hand number -> true (folded) / false (open)

  reset() {
    this.data = null;
    this.version = -1;
    this.folded = new Map();
  },

  async refresh( view , list ) {
    if ( this.loading || view.version === this.version ) return;
    this.loading = true;
    const version = view.version;
    try {
      this.data = Table.target && Table.target.watch ? await Api.watchRecord( Table.target.watch ) : await Api.record( view.id );
      this.version = version;
    } catch ( error ) {
      this.loading = false;
      Toast.error( error );
      return;
    }
    this.loading = false;
    // Only draw if the log is still what is on screen.
    if ( !list.hidden ) this.render( Table.view || view , list );
  },

  render( view , list ) {
    const scrolled = list.scrollTop;
    const atBottom = list.scrollHeight - list.scrollTop - list.clientHeight < 24;
    Dom.clear( list );
    if ( !this.data ) {
      list.appendChild( Dom.el( "li" , { class: "chat-empty" , text: I18n.get( "record.loading" ) } ) );
      return;
    }
    const hands = this.data.hands;
    if ( hands.length === 0 ) {
      list.appendChild( Dom.el( "li" , { class: "chat-empty" , text: I18n.get( "chat.log_empty" ) } ) );
      return;
    }
    const latest = hands[ hands.length - 1 ].hand;
    hands.forEach( function ( hand ) { list.appendChild( Record.handSection( view , hand , hand.hand === latest ) ); } );
    // Follow the game as it grows, unless you scrolled back to read.
    list.scrollTop = atBottom ? list.scrollHeight : scrolled;
  },

  // Us / them from where you sit; a watcher sees teams 1 and 2.
  us( view ) {
    return view.you >= 0 ? view.you % 2 : view.perspective % 2;
  },

  signed( number ) {
    return ( number > 0 ? "+" : "" ) + number;
  },

  handSection( view , hand , isLatest ) {
    const name = function ( seat ) { return Seats.name( view.seats[ seat ] ); };
    const us = this.us( view );
    const details = Dom.el( "details" , { class: "record-hand" + ( isLatest ? " is-current" : "" ) } );
    const folded = this.folded.has( hand.hand ) ? this.folded.get( hand.hand ) : !isLatest;
    details.open = !folded;
    details.addEventListener( "toggle" , function () { Record.folded.set( hand.hand , !details.open ); } );

    // The header: hand number, contract and trump, and how it came out.
    const contract = Dom.el( "span" , { class: "record-contract" } );
    if ( hand.bidder >= 0 ) {
      contract.appendChild( Dom.el( "span" , { text: I18n.format( "record.contract" , { name: name( hand.bidder ) , bid: hand.bid } ) } ) );
      if ( hand.trump ) contract.appendChild( Art.glyphIcon( hand.trump , "glyph inline-glyph trump-" + hand.trump ) );
    } else {
      contract.appendChild( Dom.el( "span" , { text: I18n.get( "record.bidding_open" ) } ) );
    }
    const head = Dom.el( "summary" , { class: "record-hand-head" , children: [
      Dom.el( "span" , { class: "record-hand-number" , text: I18n.format( "record.hand" , { hand: hand.hand } ) } ),
      contract,
    ] } );
    if ( hand.result ) {
      const bidderTeam = hand.bidder >= 0 ? hand.bidder % 2 : 0;
      head.appendChild( Dom.el( "span" , {
        class: "record-result " + ( hand.result.made ? "is-made" : "is-set" ) + ( bidderTeam === us ? " team-us" : " team-them" ),
        text: I18n.get( hand.result.made ? "record.made" : "record.set" ) + " · " + I18n.format( "record.result" , {
          us: Summary.teamName( view , us ) , us_scored: this.signed( hand.result.scored[ us ] ),
          them: Summary.teamName( view , 1 - us ) , them_scored: this.signed( hand.result.scored[ 1 - us ] ),
        } ),
      } ) );
    }
    details.appendChild( head );

    const body = Dom.el( "div" , { class: "record-hand-body" } );
    if ( hand.partial ) body.appendChild( Dom.el( "p" , { class: "record-note" , text: I18n.get( "record.partial" ) } ) );

    if ( hand.bids.length ) {
      const bids = Dom.el( "div" , { class: "record-bids" } );
      hand.bids.forEach( function ( bid ) {
        bids.appendChild( Dom.el( "span" , {
          class: "record-bid" + ( bid.amount ? "" : " is-pass" ) + ( bid.seat === hand.bidder && bid.amount === hand.bid ? " is-winning" : "" ),
          children: [
            Dom.el( "span" , { class: "record-bid-name" , text: name( bid.seat ) } ),
            Dom.el( "span" , { class: "record-bid-amount" , text: bid.amount ? String( bid.amount ) : I18n.get( "record.pass" ) } ),
          ],
        } ) );
      } );
      body.appendChild( Dom.el( "div" , { class: "record-row" , children: [
        Dom.el( "span" , { class: "record-label" , text: I18n.get( "record.bidding" ) } ),
        bids,
      ] } ) );
    }
    if ( hand.trump && hand.bidder >= 0 && !hand.partial ) {
      body.appendChild( Dom.el( "p" , { class: "record-trump trump-" + hand.trump , children: [
        Art.glyphIcon( hand.trump , "glyph inline-glyph" ),
        Dom.el( "span" , { text: I18n.format( "record.trump" , { name: name( hand.bidder ) , color: I18n.get( "colors." + hand.trump ) } ) } ),
      ] } ) );
    }

    hand.tricks.forEach( function ( trick ) { body.appendChild( Record.trickBlock( view , trick , us ) ); } );

    if ( hand.result && !hand.partial ) {
      body.appendChild( Dom.el( "p" , { class: "record-totals" , text: I18n.format( "record.totals" , {
        us: Summary.teamName( view , us ) , us_total: hand.result.totals[ us ],
        them: Summary.teamName( view , 1 - us ) , them_total: hand.result.totals[ 1 - us ],
      } ) } ) );
    }
    details.appendChild( body );
    return Dom.el( "li" , { class: "record-item" , children: [ details ] } );
  },

  trickBlock( view , trick , us ) {
    const name = function ( seat ) { return Seats.name( view.seats[ seat ] ); };
    const status = trick.done
      ? I18n.format( "record.took" , { name: name( trick.winner ) , points: trick.points } )
      : I18n.get( "record.in_play" );
    const team = trick.done ? ( trick.winner % 2 === us ? " team-us" : " team-them" ) : " is-live";
    const cards = Dom.el( "div" , { class: "record-cards" } );
    trick.plays.forEach( function ( play ) {
      cards.appendChild( Dom.el( "figure" , { class: "record-play" + ( trick.done && play.seat === trick.winner ? " is-winner" : "" ) , children: [
        Cards.el( play.card , { class: "record-card" } ),
        Dom.el( "figcaption" , { text: name( play.seat ) } ),
      ] } ) );
    } );
    return Dom.el( "section" , { class: "record-trick" + team , children: [
      Dom.el( "header" , { class: "record-trick-head" , children: [
        Dom.el( "span" , { class: "record-trick-number" , text: I18n.format( "record.trick" , { number: trick.number } ) } ),
        Dom.el( "span" , { class: "record-trick-status" , text: status } ),
      ] } ),
      cards,
    ] } );
  },
};
