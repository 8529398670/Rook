// Your hand, fanned along the bottom of the screen.
//
// Cards are keyed by id and reused between renders, so when one is played the
// rest slide together (a CSS transition on transform) instead of the whole
// row being rebuilt. Positions are computed here and handed to the
// stylesheet as custom properties; the stylesheet does the drawing.
//
// Two ways to play a card, both one-handed:
//   - tap it to lift it, tap it again to play it;
//   - pick it up and throw it at the table.
// Picking up: press a card you can play and it lifts; move and it comes off
// the hand into your fingers (fling.js), pinned where you grabbed it and
// swinging as you move, growing toward table size as it nears the pile. Let
// go over the table -- or flick it upward from anywhere -- and it flies with
// your hand's speed and settles into your place on the pile; let go short of
// the table and it springs back into the hand. The pile adopts the card
// where it lands (trick.js predict), so nothing waits on the server; if the
// server refuses the move, the card comes back. While burying, a tap toggles
// the card in or out of the five.
const Hand = {
  element: null,
  nodes: new Map(),
  order: [],
  selected: null,
  bury: new Set(),
  legal: new Set(),
  mode: "idle",
  handNumber: 0,
  press: null,
  handlers: null,
  // Cards on their way to the table: hidden here and left out of the fan
  // until the server's answer removes them (or a refusal brings them back).
  launching: new Set(),

  init( element , handlers ) {
    this.element = element;
    this.handlers = handlers;
    element.addEventListener( "pointerdown" , this.onDown.bind( this ) );
    // Moves and the release are followed on the window: a held card travels
    // far outside the hand, and a redraw must not be able to lose it.
    window.addEventListener( "pointermove" , this.onMove.bind( this ) );
    window.addEventListener( "pointerup" , this.onUp.bind( this ) );
    window.addEventListener( "pointercancel" , this.cancelPress.bind( this ) );
    // Keyboard: Enter or Space on a focused card arrives as a click with
    // detail 0. Pointer taps are handled in onUp, so those clicks are ignored.
    element.addEventListener( "click" , function ( event ) {
      const card = event.target.closest( ".playing-card" );
      if ( card && event.detail === 0 ) Hand.tap( card.getAttribute( "data-card" ) );
    } );
    // Lay out again whenever the hand's own box changes size -- rotation,
    // a resized window, the tray growing -- rather than on window "resize",
    // which can fire before the new width has settled.
    if ( typeof ResizeObserver === "function" ) {
      new ResizeObserver( function () { Hand.layout(); } ).observe( element );
    } else {
      window.addEventListener( "resize" , function () { Hand.layout(); } );
    }
  },

  reset() {
    if ( this.press && this.press.body ) Fling.remove( this.press.body );
    this.press = null;
    this.feltState( false , false );
    this.nodes.forEach( function ( node ) { node.remove(); } );
    this.nodes = new Map();
    this.order = [];
    this.selected = null;
    this.bury = new Set();
    this.launching = new Set();
    this.handNumber = 0;
  },

  render( view ) {
    const game = view.game;
    // A watcher sees the hand of the seat they watch from, to look at only.
    const ids = game ? game.hand : [];
    const fresh = game ? game.hand_number !== this.handNumber : false;
    if ( game ) this.handNumber = game.hand_number;
    if ( fresh ) {
      this.bury = new Set();
      this.selected = null;
    }

    const live = view.status === "playing";
    if ( game && live && game.phase === "kitty" && game.bidder === view.you ) {
      this.mode = "bury";
    } else if ( game && live && game.phase === "playing" && game.turn === view.you ) {
      this.mode = "play";
    } else {
      this.mode = "idle";
    }
    if ( this.mode !== "bury" ) this.bury = new Set();
    this.legal = new Set( game ? game.legal : [] );
    const pickedUp = new Set( game ? game.picked_up : [] );

    const keep = new Set( ids );
    this.nodes.forEach( function ( node , id ) {
      if ( !keep.has( id ) ) {
        node.remove();
        Hand.nodes.delete( id );
      }
    } );
    this.bury.forEach( function ( id ) { if ( !keep.has( id ) ) Hand.bury.delete( id ); } );
    this.launching.forEach( function ( id ) { if ( !keep.has( id ) ) Hand.launching.delete( id ); } );
    if ( this.selected && ( !keep.has( this.selected ) || this.mode !== "play" || !this.legal.has( this.selected ) ) ) this.selected = null;

    ids.forEach( function ( id , index ) {
      let node = Hand.nodes.get( id );
      if ( !node ) {
        node = Cards.el( id , { tag: "button" } );
        Hand.nodes.set( id , node );
        if ( fresh || pickedUp.has( id ) ) {
          node.classList.add( "is-dealt" );
          Dom.vars( node , { deal: index } );
          node.addEventListener( "animationend" , function () { node.classList.remove( "is-dealt" ); } , { once: true } );
        }
      }
      // Into sorted order -- but only when out of place, so a card being
      // held is never pulled out of the page mid-gesture.
      if ( Hand.element.children[ index ] !== node ) Hand.element.insertBefore( node , Hand.element.children[ index ] || null );
    } );
    this.order = ids.filter( function ( id ) { return !Hand.launching.has( id ); } );
    this.paint();
    this.layout();
  },

  // Classes only -- cheap enough to run after every tap.
  paint() {
    const mode = this.mode;
    if ( mode === "idle" ) this.nodes.forEach( function ( node ) { node.classList.remove( "is-hinted" ); } );
    const pickedUp = Table.view && Table.view.game ? new Set( Table.view.game.picked_up ) : new Set();
    this.nodes.forEach( function ( node , id ) {
      node.classList.toggle( "is-playable" , mode === "play" && Hand.legal.has( id ) );
      node.classList.toggle( "is-blocked" , mode === "play" && !Hand.legal.has( id ) );
      node.classList.toggle( "is-selected" , mode === "play" && Hand.selected === id );
      node.classList.toggle( "is-buried" , mode === "bury" && Hand.bury.has( id ) );
      node.classList.toggle( "is-picked-up" , mode === "bury" && pickedUp.has( id ) );
      node.classList.toggle( "is-launched" , Hand.launching.has( id ) );
      node.setAttribute( "aria-pressed" , String( Hand.selected === id || Hand.bury.has( id ) ) );
    } );
  },

  layout() {
    const count = this.order.length;
    const width = this.element.clientWidth;
    if ( count === 0 || width === 0 ) return;
    const sample = this.nodes.get( this.order[ 0 ] );
    const cardWidth = sample.offsetWidth || 64;
    const cardHeight = cardWidth * 1.4;
    const gutter = 12;

    // One fanned row normally. The 18 cards you hold while burying get two
    // rows on a narrow screen, so every card stays wide enough to tap.
    const twoRows = count > 13 && ( width - gutter * 2 - cardWidth ) / ( count - 1 ) < cardWidth * 0.36;
    this.element.classList.toggle( "two-rows" , twoRows );
    const rows = twoRows ? [ this.order.slice( 0 , Math.ceil( count / 2 ) ) , this.order.slice( Math.ceil( count / 2 ) ) ] : [ this.order ];

    let z = 1;
    rows.forEach( function ( row , rowIndex ) {
      const span = row.length;
      const step = span > 1 ? Math.min( cardWidth * 0.62 , ( width - gutter * 2 - cardWidth ) / ( span - 1 ) ) : 0;
      const total = cardWidth + step * ( span - 1 );
      const left = ( width - total ) / 2;
      const rowOffset = twoRows && rowIndex === 0 ? -cardHeight * 0.52 : 0;
      row.forEach( function ( id , index ) {
        // A gentle arc: tilt and drop toward the ends, like a hand of cards.
        const t = span > 1 ? ( index - ( span - 1 ) / 2 ) / ( ( span - 1 ) / 2 ) : 0;
        Dom.vars( Hand.nodes.get( id ) , {
          x: ( left + step * index ).toFixed( 1 ) + "px",
          y: ( rowOffset + t * t * ( twoRows ? 3 : 8 ) ).toFixed( 1 ) + "px",
          r: ( t * ( twoRows ? 3 : 7 ) ).toFixed( 2 ) + "deg",
          z: z,
        } );
        z += 1;
      } );
    } );
  },

  // The hint's "show me" step: lift the suggested card and mark it.
  highlight( id ) {
    this.nodes.forEach( function ( node , card ) { node.classList.toggle( "is-hinted" , card === id ); } );
    if ( this.mode === "play" && this.legal.has( id ) ) this.selected = id;
    this.paint();
  },

  rectOf( id ) {
    const node = this.nodes.get( id );
    return node ? node.getBoundingClientRect() : null;
  },

  playable( id ) {
    return this.mode === "play" && this.legal.has( id ) && !this.launching.has( id );
  },

  onDown( event ) {
    const card = event.target.closest( ".playing-card" );
    if ( !card || event.button > 0 || this.press ) return;
    const id = card.getAttribute( "data-card" );
    if ( this.launching.has( id ) ) return;
    this.press = { id: id , card: card , pointer: event.pointerId , x: event.clientX , y: event.clientY , samples: [] , body: null };
    this.sample( this.press , event );
    // A card you could play lifts the moment you touch it.
    if ( this.playable( id ) ) card.classList.add( "is-pressed" );
    try { card.setPointerCapture( event.pointerId ); } catch ( error ) { /* a synthetic event: fine */ }
  },

  sample( press , event ) {
    press.samples.push( { x: event.clientX , y: event.clientY , t: event.timeStamp || performance.now() } );
    if ( press.samples.length > 16 ) press.samples.shift();
  },

  // Pointer velocity in px/s over the last ~70ms: what a throw carries, and
  // what the held card leans into.
  velocity( press ) {
    const samples = press.samples;
    if ( samples.length < 2 ) return { x: 0 , y: 0 };
    const last = samples[ samples.length - 1 ];
    let first = samples[ 0 ];
    for ( let index = samples.length - 2; index >= 0; index -= 1 ) {
      first = samples[ index ];
      if ( last.t - first.t > 70 ) break;
    }
    const dt = Math.max( 8 , last.t - first.t ) / 1000;
    return { x: ( last.x - first.x ) / dt , y: ( last.y - first.y ) / dt };
  },

  // Above the top edge of the hand is "over the table".
  overTable( clientY ) {
    return clientY < this.element.getBoundingClientRect().top;
  },

  feltState( dragging , ready ) {
    const felt = document.getElementById( "felt" );
    if ( !felt ) return;
    felt.classList.toggle( "is-card-drag" , dragging );
    felt.classList.toggle( "is-drop-ready" , dragging && ready );
  },

  tiltOf( node ) {
    return parseFloat( node.style.getPropertyValue( "--r" ) ) || 0;
  },

  // Your place on the pile, in the coordinates a thrown card aims for.
  slot() {
    const view = Table.view;
    return Trick.slotTarget( view , view ? view.you : 0 );
  },

  onMove( event ) {
    const press = this.press;
    if ( !press || event.pointerId !== press.pointer ) return;
    this.sample( press , event );
    if ( !press.body ) {
      if ( Math.hypot( event.clientX - press.x , event.clientY - press.y ) < 6 ) return;
      if ( !this.playable( press.id ) ) return;
      this.pickUp( press );
    }
    const body = press.body;
    const speed = this.velocity( press );
    // Lean into the direction of travel, like a card held by one corner.
    const lean = Math.max( -16 , Math.min( 16 , speed.x * 0.012 ) );
    // Grow toward its size on the pile as it nears the pile.
    const top = this.element.getBoundingClientRect().top;
    const reach = Math.max( 1 , top - press.slot.cy );
    const near = Math.max( 0 , Math.min( 1 , ( top + 20 - event.clientY ) / reach ) );
    const landed = press.slot.width / body.w;
    const scale = 1.12 + ( landed * 1.04 - 1.12 ) * near;
    Fling.follow( body , event.clientX , event.clientY , lean , scale );
    this.feltState( true , this.overTable( event.clientY ) );
  },

  // The card comes off the hand into your fingers. What stays behind is a
  // faint outline of the place it will go back to.
  pickUp( press ) {
    const node = press.card;
    node.classList.remove( "is-pressed" );
    press.slot = this.slot();
    press.body = Fling.create( press.id , node.getBoundingClientRect() , node.offsetWidth , this.tiltOf( node ) );
    Fling.grab( press.body , press.x , press.y );
    node.classList.add( "is-held-out" );
    this.selected = null;
    this.paint();
  },

  onUp( event ) {
    const press = this.press;
    if ( !press || event.pointerId !== press.pointer ) return;
    this.press = null;
    press.card.classList.remove( "is-pressed" );
    if ( !press.body ) {
      this.tap( press.id );
      return;
    }
    this.feltState( false , false );
    const speed = this.velocity( press );
    const thrown = this.overTable( event.clientY ) || speed.y < -650;
    if ( thrown && this.playable( press.id ) ) {
      this.throwCard( press.id , press.body , speed );
    } else {
      this.returnCard( press.id , press.body , speed );
    }
  },

  cancelPress( event ) {
    const press = this.press;
    if ( !press || ( event && event.pointerId !== press.pointer ) ) return;
    this.press = null;
    press.card.classList.remove( "is-pressed" );
    this.feltState( false , false );
    if ( press.body ) this.returnCard( press.id , press.body , { x: 0 , y: 0 } );
  },

  // Short of the table: it springs back into its place in the fan.
  returnCard( id , body , speed ) {
    const node = this.nodes.get( id );
    if ( !node ) {
      Fling.remove( body );
      return;
    }
    const rect = node.getBoundingClientRect();
    Fling.throwTo( body , { cx: rect.left + rect.width / 2 , cy: rect.top + rect.height / 2 , r: this.tiltOf( node ) , s: 1 } , speed ).then( function () {
      Fling.remove( body );
      node.classList.remove( "is-held-out" );
    } );
  },

  // Into the pile. The fan closes up behind it straight away; the move goes
  // to the server while the card is still in the air.
  async throwCard( id , body , speed ) {
    const node = this.nodes.get( id );
    if ( node ) node.classList.remove( "is-held-out" );
    this.selected = null;
    this.launching.add( id );
    this.order = this.order.filter( function ( card ) { return card !== id; } );
    this.paint();
    this.layout();
    const view = Table.view;
    const slot = this.slot();
    const landed = Fling.throwTo( body , { cx: slot.cx , cy: slot.cy , r: slot.r , s: slot.width / body.w } , speed );
    Trick.predict( view , view ? view.you : 0 , id , body , landed );
    const ok = await this.handlers.play( id );
    if ( ok ) return;
    this.restore( id , Trick.unpredict( id ) );
  },

  tap( id ) {
    if ( !id ) return;
    if ( this.mode === "bury" ) {
      if ( this.bury.has( id ) ) {
        this.bury.delete( id );
      } else if ( this.bury.size < 5 ) {
        this.bury.add( id );
      } else {
        Toast.show( I18n.get( "kitty.only_five" ) );
      }
      this.paint();
      this.handlers.buryChanged();
      return;
    }
    if ( this.mode !== "play" ) return;
    if ( !this.legal.has( id ) ) {
      const node = this.nodes.get( id );
      Motion.animate( node , [ { translate: "0 0" } , { translate: "-5px 0" } , { translate: "5px 0" } , { translate: "0 0" } ] , { duration: 260 } );
      Toast.show( I18n.get( this.blockReason() ) );
      return;
    }
    if ( this.selected === id ) {
      this.play( id );
      return;
    }
    this.selected = id;
    this.paint();
  },

  // Why the tapped card cannot be played: following the color led is the
  // usual reason; the bird on a first trick that forbids it is the other.
  blockReason() {
    const game = Table.view && Table.view.game;
    if ( game && game.trick.plays.length === 0 ) return "hand.rook_first_trick";
    return "hand.must_follow";
  },

  play( id ) {
    this.launch( id );
  },

  // A tap-tap (or keyboard) play: the card is tossed from where it sits in
  // the fan, the same flight as a throw.
  launch( id ) {
    const node = this.nodes.get( id );
    if ( !node || !this.playable( id ) ) return;
    const body = Fling.create( id , node.getBoundingClientRect() , node.offsetWidth , this.tiltOf( node ) );
    const slot = this.slot();
    const rect = node.getBoundingClientRect();
    const dx = slot.cx - ( rect.left + rect.width / 2 );
    const dy = slot.cy - ( rect.top + rect.height / 2 );
    const length = Math.max( 1 , Math.hypot( dx , dy ) );
    this.throwCard( id , body , { x: dx / length * 1500 , y: dy / length * 1500 } );
  },

  restore( id , fromRect ) {
    if ( !this.launching.delete( id ) ) return;
    const node = this.nodes.get( id );
    if ( !node ) return;
    // Back into sorted order: the snapshot's hand, minus anything else
    // still on its way.
    const game = Table.view && Table.view.game;
    const ids = game ? game.hand : [];
    this.order = ids.filter( function ( card ) { return Hand.nodes.has( card ) && !Hand.launching.has( card ); } );
    this.paint();
    this.layout();
    if ( !fromRect ) return;
    const to = node.getBoundingClientRect();
    Motion.animate( node , [
      { translate: ( fromRect.left - to.left ) + "px " + ( fromRect.top - to.top ) + "px" },
      { translate: "0 0" },
    ] , { duration: 300 , easing: "cubic-bezier(.2,.8,.25,1)" } );
  },
};
