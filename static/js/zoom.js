// How big the cards are drawn, chosen per device in Settings (and in the
// table's menu, where you can see the effect). Like the theme it is loaded
// in <head> and applied at once, so a saved size is there on first paint.
//
// Two numbers, both multipliers on the size the layout would pick anyway:
// the cards on the table (the trick) and the cards in your hand. The
// stylesheet still caps the trick so it never runs into the seats -- a
// bigger number means "as big as fits", not "overflow the felt".
//
// localStorage is a per-browser convenience, so every access is wrapped: a
// private window or blocked storage just means the default size.
const Zoom = {
  storageKey: "rook.zoom",
  ranges: {
    table: { min: 80 , max: 200 , step: 5 },
    hand: { min: 80 , max: 150 , step: 5 },
  },
  controls: [],

  clamp( name , value ) {
    const range = this.ranges[ name ];
    const number = Math.round( Number( value ) );
    if ( !isFinite( number ) ) return 100;
    return Math.min( range.max , Math.max( range.min , number ) );
  },

  get() {
    let saved = {};
    try {
      saved = JSON.parse( window.localStorage.getItem( this.storageKey ) || "{}" ) || {};
    } catch ( error ) {
      saved = {};
    }
    return { table: this.clamp( "table" , saved.table || 100 ) , hand: this.clamp( "hand" , saved.hand || 100 ) };
  },

  apply( values ) {
    const root = document.documentElement.style;
    root.setProperty( "--table-zoom" , String( values.table / 100 ) );
    root.setProperty( "--hand-zoom" , String( values.hand / 100 ) );
  },

  set( name , value ) {
    const values = this.get();
    values[ name ] = this.clamp( name , value );
    try {
      window.localStorage.setItem( this.storageKey , JSON.stringify( values ) );
    } catch ( error ) {
      // Storage refused: the size still applies for this page view.
    }
    this.apply( values );
    this.sync();
    return values;
  },

  reset() {
    try {
      window.localStorage.removeItem( this.storageKey );
    } catch ( error ) {
      // Nothing saved to remove.
    }
    this.apply( this.get() );
    this.sync();
  },

  // Fill a [data-zoom-controls] element with the two sliders and a reset
  // button. Every copy on the page stays in step with the others.
  bind( container ) {
    if ( !container || container.getAttribute( "data-bound" ) ) return;
    container.setAttribute( "data-bound" , "1" );
    [ "table" , "hand" ].forEach( function ( name ) {
      const range = Zoom.ranges[ name ];
      const id = container.id + "-" + name;
      const input = Dom.el( "input" , { attrs: {
        id: id , type: "range" , min: String( range.min ) , max: String( range.max ) , step: String( range.step ),
      } } );
      const value = Dom.el( "output" , { class: "zoom-value" , attrs: { for: id } } );
      input.addEventListener( "input" , function () { Zoom.set( name , input.value ); } );
      container.appendChild( Dom.el( "div" , { class: "zoom-row" , children: [
        Dom.el( "label" , { class: "zoom-label" , text: I18n.get( "zoom." + name ) , attrs: { for: id } } ),
        input,
        value,
      ] } ) );
      Zoom.controls.push( { name: name , input: input , value: value } );
    } );
    container.appendChild( Dom.el( "button" , {
      class: "small secondary zoom-reset",
      text: I18n.get( "zoom.reset" ),
      attrs: { type: "button" },
      on: { click: function () { Zoom.reset(); } },
    } ) );
    this.sync();
  },

  sync() {
    const values = this.get();
    this.controls.forEach( function ( control ) {
      control.input.value = String( values[ control.name ] );
      control.value.textContent = values[ control.name ] + "%";
    } );
  },
};

Zoom.apply( Zoom.get() );
