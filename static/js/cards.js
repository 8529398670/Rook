// The deck, drawn once as SVG <symbol>s and then reused everywhere with
// <use>. Fifty-seven faces and one back are built at startup into a hidden
// sprite; every card on screen after that is a three-node <svg><use/></svg>,
// which keeps a full table cheap to redraw on a phone.
//
// Card ids are the server's wire form: "R14", "G1", "Y5", "B10", "ROOK".
const Cards = {
  width: 250,
  height: 350,
  colors: { R: "red" , B: "black" , Y: "yellow" , G: "green" },
  points: { 1: 15 , 5: 5 , 10: 10 , 14: 10 },

  parse( id ) {
    if ( id === "ROOK" ) return { id: id , color: "rook" , number: 0 , rook: true };
    return { id: id , color: this.colors[ id.charAt( 0 ) ] , number: Number( id.slice( 1 ) ) , rook: false };
  },

  pointsOf( id ) {
    if ( id === "ROOK" ) return 20;
    return this.points[ this.parse( id ).number ] || 0;
  },

  // "Red 14", "the Rook" -- for aria-labels and the game log.
  name( id ) {
    const card = this.parse( id );
    if ( card.rook ) return I18n.get( "cards.rook" );
    return I18n.format( "cards.name" , { color: I18n.get( "colors." + card.color ) , number: card.number } );
  },

  init() {
    if ( document.getElementById( "card-sprite" ) ) return;
    const defs = Dom.svg( "defs" , {} , [
      // Interlocking circles, after the ornament on the original card back.
      Dom.svg( "pattern" , { id: "back-pattern" , width: 96 , height: 96 , patternUnits: "userSpaceOnUse" , x: 29 , y: 31 } , [
        Dom.svg( "rect" , { width: 96 , height: 96 , fill: Cards.back.teal } ),
      ].concat( [ [ 0 , 0 ] , [ 96 , 0 ] , [ 0 , 96 ] , [ 96 , 96 ] , [ 48 , 48 ] ].map( function ( centre ) {
        return Dom.svg( "circle" , { cx: centre[ 0 ] , cy: centre[ 1 ] , r: 48 , fill: "none" , stroke: Cards.back.ornament , "stroke-width": 4 } );
      } ) ) ),
    ] );

    const symbols = [ this.backSymbol() , this.rookSymbol() ];
    Object.keys( this.colors ).forEach( function ( letter ) {
      for ( let number = 1; number <= 14; number += 1 ) {
        symbols.push( Cards.faceSymbol( letter + number ) );
      }
    } );

    const sprite = Dom.svg( "svg" , { id: "card-sprite" , width: 0 , height: 0 , "aria-hidden": "true" , class: "sprite" } , [ defs ].concat( symbols ) );
    document.body.insertBefore( sprite , document.body.firstChild );
  },

  back: { teal: "#40aab8" , ornament: "#56bac6" , edge: "#2b2b2e" },

  symbol( id , children ) {
    return Dom.svg( "symbol" , { id: "card-" + id , viewBox: "0 0 250 350" } , children );
  },

  // White stock with the thin dark edge of the original deck.
  blank() {
    return [
      Dom.svg( "rect" , { x: 1.5 , y: 1.5 , width: 247 , height: 347 , rx: 16 , fill: Art.paper , stroke: Art.paperEdge , "stroke-width": 3 } ),
    ];
  },

  // The numerals are in Comic Sans (see --font-number in style.css; an SVG
  // attribute cannot read a CSS variable, so the list is repeated here).
  // weight thickens the strokes past the font's bold with an outline in the
  // same ink.
  text( value , x , y , size , fill , anchor , weight ) {
    const node = Dom.svg( "text" , {
      x: x , y: y , fill: fill ,
      "font-family": "'Comic Sans MS', 'Comic Sans', 'Comic Neue', system-ui, sans-serif" ,
      "font-weight": 700 , "font-size": size , "text-anchor": anchor || "middle" ,
      stroke: weight ? fill : null , "stroke-width": weight || null , "stroke-linejoin": weight ? "round" : null ,
    } );
    node.textContent = String( value );
    return node;
  },

  // Counter cards carry their point value in a small solid box under the
  // color name, as the original deck does.
  pointsBox( points , x , ink ) {
    const width = points >= 10 ? 30 : 22;
    return Dom.svg( "g" , { class: "points-box" } , [
      Dom.svg( "rect" , { x: x , y: 39 , width: width , height: 21 , rx: 1.5 , fill: ink } ),
      this.text( points , x + width / 2 , 55.5 , 16 , "#ffffff" ),
    ] );
  },

  faceSymbol( id ) {
    const card = this.parse( id );
    const ink = Art.palette[ card.color ];
    const wide = card.number >= 10;
    const points = this.pointsOf( id );

    // Top-left: the number, and beside it the color's name and the points.
    // The number sits in the left-most strip, which is all that shows of a
    // card overlapped in a hand.
    // Comic Sans digits are wide, so a two-digit number is drawn a little
    // tighter and the name moves over to make room.
    const nameX = wide ? 94 : 54;
    const index = function ( transform ) {
      const number = Cards.text( card.number , 12 , 62 , 58 , ink.text , "start" , 2 );
      if ( wide ) number.setAttribute( "letter-spacing" , "-5" );
      const parts = [
        number,
        Cards.text( I18n.get( "colors." + card.color ).toUpperCase() , nameX , 33 , 15 , ink.text , "start" , 0.4 ),
      ];
      if ( points > 0 ) parts.push( Cards.pointsBox( points , nameX , ink.main ) );
      return Dom.svg( "g" , { transform: transform } , parts );
    };

    return this.symbol( id , this.blank().concat( [
      // The thin frame in the middle with the big number inside it.
      Dom.svg( "rect" , { x: 54 , y: 104 , width: 142 , height: 142 , fill: "none" , stroke: ink.main , "stroke-width": 1.6 , opacity: 0.65 } ),
      this.text( card.number , 125 , 213 , wide ? 94 : 108 , ink.text , "middle" , 3 ),
      index( "" ),
      index( "rotate(180 125 175)" ),
    ] ) );
  },

  // The traced art was cut from the original 136x210 card scans and
  // enlarged 8x. scan() places a piece at the spot it was cut from, scaled
  // so the original card's height fills ours and centred across.
  scanScale: 350 / 210,

  scan( name , tones , left , top ) {
    const scale = this.scanScale;
    const x = ( 250 - 136 * scale ) / 2 + left * scale;
    return Dom.svg( "g" , { transform: "translate(" + x + " " + top * scale + ") scale(" + scale / 8 + ")" } , [
      Art.traced( name , tones ),
    ] );
  },

  // The original Rook card: the teal crow perched on the 20, ROOK down two
  // corners, little crows beside them -- all traced from the scan.
  rookSymbol() {
    return this.symbol( "ROOK" , this.blank().concat( [
      this.scan( "rookface" , { ink: Art.palette.rook.main , light: "#8fcbd8" } , 6 , 6 ),
    ] ) );
  },

  // The original back: teal ornament inside a white border, the crow
  // cawing with a fan of cards, and the ROOK logo in white, gold and black.
  backSymbol() {
    return this.symbol( "back" , [
      Dom.svg( "rect" , { x: 1.5 , y: 1.5 , width: 247 , height: 347 , rx: 16 , fill: "#ffffff" , stroke: Cards.back.edge , "stroke-width": 3 } ),
      Dom.svg( "rect" , { x: 17 , y: 17 , width: 216 , height: 316 , rx: 12 , fill: "url(#back-pattern)" } ),
      this.scan( "backcrow" , { ink: "#101c1f" } , 22 , 30 ),
      this.scan( "backlogo" , { ink: "#111111" , gold: "#f5b70f" , white: "#ffffff" } , 14 , 102 ),
    ] );
  },

  // A card as an <svg> referencing the sprite.
  svg( id ) {
    return Dom.svg( "svg" , { viewBox: "0 0 250 350" , class: "card-svg" , "aria-hidden": "true" , focusable: "false" } , [
      Dom.svg( "use" , { href: "#card-" + ( id || "back" ) } ),
    ] );
  },

  // A card element for the table. Face-down when id is null.
  el( id , options ) {
    const settings = options || {};
    const node = Dom.el( settings.tag || "div" , {
      // "playing-card", not "card": the template already uses .card for
      // panels, and the two have nothing in common.
      class: "playing-card" + ( settings.class ? " " + settings.class : "" ) ,
      attrs: id ? { "data-card": id , "aria-label": this.name( id ) } : { "aria-hidden": "true" } ,
      children: [ this.svg( id ) ] ,
    } );
    if ( settings.tag === "button" ) node.type = "button";
    return node;
  },
};
