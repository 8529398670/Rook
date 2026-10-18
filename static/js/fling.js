// A card you are holding or have thrown: one loose element on top of
// everything, moved by springs so it feels like a real card.
//
// While held, the card stays pinned under your finger at the exact point you
// grabbed it, and swings a little around that point as you move -- leaning
// into the direction of travel, like a card held by one corner. When you let
// go it keeps the speed your hand had and springs into wherever it is going
// (the pile, or back into your hand), overshooting and settling instead of
// sliding there on a fixed curve.
//
// Position, tilt and size live in custom properties (--fx, --fy, --fr, --fs,
// --fox, --foy) that the stylesheet turns into a transform: the CSP forbids
// inline style attributes, but CSSOM custom properties are fine.
const Fling = {
  // Springs: stiffness and damping per unit mass. Slightly under-damped, so
  // a throw lands with a small, quick settle rather than a bounce.
  move: { k: 240 , c: 25 },
  tilt: { k: 320 , c: 22 },
  size: { k: 380 , c: 30 },

  // card: an id for Cards.el; rect: where it is now (a client rect);
  // width: its unrotated width; rotation: its current tilt in degrees.
  create( card , rect , width , rotation ) {
    const height = width * 1.4;
    const el = Cards.el( card , { class: "held-card" } );
    const body = {
      el: el , w: width , h: height ,
      // Top-left of the unscaled, unrotated box, so that its centre sits
      // at the centre of rect.
      x: rect.left + rect.width / 2 - width / 2 , y: rect.top + rect.height / 2 - height / 2 ,
      r: rotation || 0 , s: 1 ,
      vx: 0 , vy: 0 , vr: 0 , vs: 0 ,
      ox: width / 2 , oy: height / 2 , // transform origin, within the box
      mode: "held" , target: null , lean: 0 , scale: 1.1 ,
      frame: 0 , last: 0 , done: null ,
    };
    Dom.vars( el , { fw: width + "px" } );
    document.body.appendChild( el );
    this.draw( body );
    return body;
  },

  draw( body ) {
    Dom.vars( body.el , {
      fx: body.x.toFixed( 2 ) + "px" , fy: body.y.toFixed( 2 ) + "px" ,
      fr: body.r.toFixed( 2 ) + "deg" , fs: body.s.toFixed( 4 ) ,
      fox: body.ox.toFixed( 1 ) + "px" , foy: body.oy.toFixed( 1 ) + "px" ,
    } );
  },

  // Move the pivot without the card visibly moving: shift the box so the
  // rotated, scaled card stays exactly where it was drawn.
  pivot( body , ox , oy ) {
    const dx = ox - body.ox;
    const dy = oy - body.oy;
    const a = body.r * Math.PI / 180;
    const cos = Math.cos( a ) * body.s;
    const sin = Math.sin( a ) * body.s;
    body.x += -dx + ( cos * dx - sin * dy );
    body.y += -dy + ( sin * dx + cos * dy );
    body.ox = ox;
    body.oy = oy;
  },

  // Start holding at a pointer position: the grab point becomes the pivot.
  grab( body , pointerX , pointerY ) {
    this.pivot( body , pointerX - body.x , pointerY - body.y );
    body.el.classList.add( "is-held" );
    this.run( body );
  },

  // Follow the pointer. lean is the tilt to swing toward (degrees); scale
  // the size to grow toward.
  follow( body , pointerX , pointerY , lean , scale ) {
    body.x = pointerX - body.ox;
    body.y = pointerY - body.oy;
    body.lean = lean;
    body.scale = scale;
  },

  // Let go toward target {cx, cy, r, s} (centre, tilt, scale) with the
  // hand's velocity {x, y} in px/s. Resolves once it has settled there.
  throwTo( body , target , velocity ) {
    this.pivot( body , body.w / 2 , body.h / 2 );
    body.mode = "flying";
    body.el.classList.remove( "is-held" );
    body.el.classList.add( "is-flying" );
    body.target = { x: target.cx - body.w / 2 , y: target.cy - body.h / 2 , r: target.r || 0 , s: target.s || 1 };
    const speed = Math.hypot( velocity.x , velocity.y );
    const cap = speed > 4000 ? 4000 / speed : 1;
    body.vx = velocity.x * cap;
    body.vy = velocity.y * cap;
    // A sideways throw spins the card a little; the spring takes it out.
    body.vr += Math.max( -500 , Math.min( 500 , velocity.x * cap * 0.12 ) );
    if ( Motion.reduced() ) {
      Object.assign( body , { x: body.target.x , y: body.target.y , r: body.target.r , s: body.target.s } );
      this.draw( body );
      this.stop( body );
      return Promise.resolve();
    }
    return new Promise( function ( resolve ) {
      body.done = resolve;
      body.started = performance.now();
      Fling.run( body );
    } );
  },

  run( body ) {
    if ( body.frame ) return;
    body.last = performance.now();
    const tick = function ( now ) {
      body.frame = 0;
      if ( !body.el.isConnected ) {
        Fling.finish( body );
        return;
      }
      let dt = Math.min( 0.05 , ( now - body.last ) / 1000 );
      body.last = now;
      // Small fixed steps keep a stiff spring stable on a slow frame.
      while ( dt > 0 ) {
        const step = Math.min( dt , 1 / 240 );
        Fling.step( body , step );
        dt -= step;
      }
      Fling.draw( body );
      if ( body.mode === "flying" && ( Fling.settled( body ) || now - body.started > 1400 ) ) {
        const target = body.target;
        Object.assign( body , { x: target.x , y: target.y , r: target.r , s: target.s } );
        Fling.draw( body );
        Fling.finish( body );
        return;
      }
      if ( body.mode === "held" || body.mode === "flying" ) body.frame = requestAnimationFrame( tick );
    };
    body.frame = requestAnimationFrame( tick );
  },

  spring( value , velocity , target , spring , dt ) {
    const accel = -spring.k * ( value - target ) - spring.c * velocity;
    velocity += accel * dt;
    return [ value + velocity * dt , velocity ];
  },

  step( body , dt ) {
    let pair;
    if ( body.mode === "flying" ) {
      const t = body.target;
      pair = this.spring( body.x , body.vx , t.x , this.move , dt ); body.x = pair[ 0 ]; body.vx = pair[ 1 ];
      pair = this.spring( body.y , body.vy , t.y , this.move , dt ); body.y = pair[ 0 ]; body.vy = pair[ 1 ];
      pair = this.spring( body.r , body.vr , t.r , this.tilt , dt ); body.r = pair[ 0 ]; body.vr = pair[ 1 ];
      pair = this.spring( body.s , body.vs , t.s , this.size , dt ); body.s = pair[ 0 ]; body.vs = pair[ 1 ];
      return;
    }
    pair = this.spring( body.r , body.vr , body.lean , this.tilt , dt ); body.r = pair[ 0 ]; body.vr = pair[ 1 ];
    pair = this.spring( body.s , body.vs , body.scale , this.size , dt ); body.s = pair[ 0 ]; body.vs = pair[ 1 ];
  },

  settled( body ) {
    const t = body.target;
    return Math.abs( body.x - t.x ) < 0.4 && Math.abs( body.y - t.y ) < 0.4 &&
      Math.abs( body.vx ) < 12 && Math.abs( body.vy ) < 12 &&
      Math.abs( body.r - t.r ) < 0.3 && Math.abs( body.s - t.s ) < 0.004;
  },

  finish( body ) {
    body.mode = "still";
    body.el.classList.remove( "is-flying" );
    const done = body.done;
    body.done = null;
    if ( done ) done();
  },

  stop( body ) {
    if ( body.frame ) cancelAnimationFrame( body.frame );
    body.frame = 0;
    this.finish( body );
  },

  remove( body ) {
    this.stop( body );
    body.el.remove();
  },
};
