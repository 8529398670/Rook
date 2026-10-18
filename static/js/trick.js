// The cards in the middle of the table, and the only animated part of a
// snapshot.
//
// Every snapshot carries the game's recent events, numbered. This file
// replays the ones it has not seen -- a "play" flies a card in from its seat,
// a "trick" holds the finished trick a moment and then sweeps it to the
// winner -- through a queue, so a burst of moves (a fast bot, a reconnect)
// still plays out one at a time and in order. Whenever it cannot replay (first
// load, or it missed events while the phone slept), it simply draws the
// current trick as it stands. The snapshot is always the truth; animation is
// only ever a way of arriving at it.
//
// Your own card does not wait for the server: you throw it (hand.js and
// fling.js), predict() makes it part of the pile where it lands, and the
// "play" event that follows simply adopts it.
const Trick = {
  element: null,
  shown: [], // [{ seat, card, node }]
  pending: null, // { seat, card, node, body } -- your card, ahead of the server
  lastSeq: 0,
  queue: Promise.resolve(),
  epoch: 0,

  init( element ) {
    this.element = element;
  },

  reset() {
    this.epoch += 1; // abandons anything still queued
    this.queue = Promise.resolve();
    this.lastSeq = 0;
    this.clear();
    this.dropPending();
  },

  clear() {
    this.shown.forEach( function ( entry ) { entry.node.remove(); } );
    this.shown = [];
  },

  dropPending() {
    const pending = this.pending;
    this.pending = null;
    if ( !pending ) return null;
    let rect = null;
    if ( pending.node ) {
      rect = pending.node.getBoundingClientRect();
      pending.node.remove();
    }
    if ( pending.body ) {
      if ( !rect ) rect = pending.body.el.getBoundingClientRect();
      Fling.remove( pending.body );
    }
    return rect;
  },

  // Where a card for seat lands on the pile -- centre, tilt and width -- so
  // a thrown card can be aimed at it before it exists.
  slotTarget( view , seat ) {
    const probe = Cards.el( "B1" , { class: this.slotClass( view , seat ) + " is-probe" } );
    const tilt = ( ( seat * 37 ) % 9 - 4 );
    Dom.vars( probe , { tilt: tilt + "deg" } );
    this.element.appendChild( probe );
    const rect = probe.getBoundingClientRect();
    const width = probe.offsetWidth;
    probe.remove();
    return { cx: rect.left + rect.width / 2 , cy: rect.top + rect.height / 2 , r: tilt , width: width };
  },

  // Your card, played: it is already flying (body, from fling.js) and
  // lands -- landed resolves -- before the server has answered. It becomes
  // a real card of the trick at once, unless the last trick is still being
  // swept away: then it hovers where it landed until the table is clear.
  // The "play" event that follows simply adopts it.
  predict( view , seat , card , body , landed ) {
    if ( !view || !body ) return;
    this.dropPending();
    const pending = { seat: seat , card: card , node: null , body: body , landed: Promise.resolve() };
    this.pending = pending;
    const epoch = this.epoch;
    let busy = true;
    landed.then( function () { if ( busy && Trick.pending === pending ) body.el.classList.add( "is-hovering" ); } );
    this.queue = this.queue.then( function () {
      busy = false;
      return landed;
    } ).then( function () {
      if ( epoch !== Trick.epoch || Trick.pending !== pending ) return null;
      const from = body.el.getBoundingClientRect();
      const node = Cards.el( card , { class: Trick.slotClass( view , seat ) + " is-pending" } );
      Dom.vars( node , { tilt: ( ( seat * 37 ) % 9 - 4 ) + "deg" } );
      Trick.element.appendChild( node );
      pending.node = node;
      Fling.remove( body );
      pending.body = null;
      const to = node.getBoundingClientRect();
      const dx = from.left + from.width / 2 - ( to.left + to.width / 2 );
      const dy = from.top + from.height / 2 - ( to.top + to.height / 2 );
      // Usually it is already exactly there; if it hovered, it drops in.
      pending.landed = Math.abs( dx ) + Math.abs( dy ) < 2 ? Promise.resolve() : Motion.animate( node , [
        { translate: dx + "px " + dy + "px" , scale: "1.04" },
        { translate: "0 0" , scale: "1" },
      ] , { duration: 220 , easing: "cubic-bezier(.3,.8,.3,1)" } );
      return pending.landed;
    } ).catch( function () {} );
  },

  // The server said no: take the predicted card off the pile and say where
  // it was, so the hand can bring it back from there.
  unpredict( card ) {
    if ( !this.pending || this.pending.card !== card ) return null;
    return this.dropPending();
  },

  slotClass( view , seat ) {
    return "trick-card slot-" + Seats.positions[ Seats.position( view , seat ) ];
  },

  place( view , seat , card ) {
    const node = Cards.el( card , { class: this.slotClass( view , seat ) } );
    Dom.vars( node , { tilt: ( ( seat * 37 ) % 9 - 4 ) + "deg" } );
    this.element.appendChild( node );
    this.shown.push( { seat: seat , card: card , node: node } );
    return node;
  },

  // Draw view's current trick with no animation.
  sync( view ) {
    this.clear();
    this.dropPending();
    if ( !view.game || view.game.phase !== "playing" ) return;
    view.game.trick.plays.forEach( function ( play ) { Trick.place( view , play.seat , play.card ); } );
  },

  // startRects: where cards you just played were in your hand, captured
  // before the hand redraws without them.
  update( view , startRects ) {
    const game = view.game;
    if ( !game ) {
      this.reset();
      return;
    }
    const events = game.events || [];
    const newest = events.length ? events[ events.length - 1 ].seq : 0;
    const fresh = events.filter( function ( event ) { return event.seq > Trick.lastSeq; } );

    // Nothing seen yet, or a gap: draw the present instead of replaying.
    if ( this.lastSeq === 0 || ( fresh.length && fresh[ 0 ].seq > this.lastSeq + 1 ) || newest < this.lastSeq ) {
      this.epoch += 1;
      this.queue = Promise.resolve();
      this.lastSeq = newest;
      this.sync( view );
      return;
    }
    if ( fresh.length === 0 ) return;
    this.lastSeq = newest;

    const epoch = this.epoch;
    const run = function ( step ) {
      Trick.queue = Trick.queue.then( function () {
        if ( epoch !== Trick.epoch ) return null;
        return step();
      } ).catch( function () {} );
    };

    fresh.forEach( function ( event ) {
      if ( event.kind === "play" ) {
        const start = startRects[ event.card ] || Seats.rect( view , event.seat );
        run( function () { return Trick.fly( view , event.seat , event.card , start ); } );
      } else if ( event.kind === "trick" ) {
        run( function () { return Trick.collect( view , event.seat , event.card ); } );
      } else if ( event.kind === "deal" ) {
        run( function () { Trick.clear(); } );
      }
    } );
    // Finish on the snapshot these events led to, whatever happened above.
    run( function () { Trick.reconcile( view ); } );
  },

  // Make what is shown match view's trick exactly, without animating.
  reconcile( view ) {
    const plays = view.game && view.game.phase === "playing" ? view.game.trick.plays : [];
    const matches = plays.length === this.shown.length && plays.every( function ( play , index ) {
      return Trick.shown[ index ].card === play.card;
    } );
    if ( !matches ) this.sync( view );
  },

  fly( view , seat , card , start ) {
    const pending = this.pending;
    if ( pending && pending.card === card && pending.node ) {
      // Already on the pile: just make it part of the trick.
      this.pending = null;
      pending.node.classList.remove( "is-pending" );
      this.shown.push( { seat: seat , card: card , node: pending.node } );
      return pending.landed;
    }
    const node = this.place( view , seat , card );
    if ( !start ) return Promise.resolve();
    const end = node.getBoundingClientRect();
    const dx = start.left + start.width / 2 - ( end.left + end.width / 2 );
    const dy = start.top + start.height / 2 - ( end.top + end.height / 2 );
    const scale = Math.max( 0.35 , Math.min( 1.4 , start.width / ( end.width || 1 ) ) );
    return Motion.animate( node , [
      { translate: dx + "px " + dy + "px" , scale: String( scale ) , opacity: 0.6 },
      { translate: "0 0" , scale: "1" , opacity: 1 },
    ] , { duration: 320 , easing: "cubic-bezier(.2,.8,.25,1)" } );
  },

  async collect( view , winner , winningCard ) {
    const entries = this.shown.slice();
    entries.forEach( function ( entry ) {
      entry.node.classList.toggle( "is-winner" , entry.card === winningCard );
    } );
    // Long enough to see who took it before the cards move.
    await Motion.wait( 1100 );
    const target = Seats.rect( view , winner );
    if ( target ) {
      await Promise.all( entries.map( function ( entry ) {
        const from = entry.node.getBoundingClientRect();
        const dx = target.left + target.width / 2 - ( from.left + from.width / 2 );
        const dy = target.top + target.height / 2 - ( from.top + from.height / 2 );
        return Motion.animate( entry.node , [
          { translate: "0 0" , scale: "1" , opacity: 1 },
          { translate: dx + "px " + dy + "px" , scale: "0.3" , opacity: 0 },
        ] , { duration: 380 , easing: "cubic-bezier(.5,0,.75,0)" , fill: "forwards" } );
      } ) );
    }
    entries.forEach( function ( entry ) { entry.node.remove(); } );
    this.shown = this.shown.filter( function ( entry ) { return entries.indexOf( entry ) < 0; } );
  },
};
