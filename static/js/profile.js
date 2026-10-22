// Your avatar (an icon and a color) and your optional PIN, on the Settings
// screen. The icon and color picker is shared with the admin's "edit
// player" sheet (settings.js), which picks for somebody else.
const Profile = {
  me: null,
  options: null,
  mine: null,

  init( me ) {
    this.me = me;
    Dom.get( "pin-form" ).addEventListener( "submit" , this.onPin.bind( this ) );
    Dom.get( "pin-remove" ).addEventListener( "click" , this.removePin.bind( this ) );
  },

  async enter() {
    this.renderPreview();
    this.renderPin();
    if ( this.mine ) return;
    if ( !await this.loadOptions() ) return;
    this.mine = this.picker( Dom.get( "icon-grid" ) , Dom.get( "color-grid" ) , {
      person: function () { return Profile.me; },
      preview: function ( person ) { Art.avatar( Dom.get( "avatar-preview" ) , person ); },
      save: function ( icon , color ) { return Profile.save( icon , color ); },
    } );
  },

  // The server's lists of icons and colors, fetched once.
  async loadOptions() {
    if ( this.options ) return true;
    try {
      this.options = await Api.profileOptions();
      return true;
    } catch ( error ) {
      Toast.error( error );
      return false;
    }
  },

  renderPreview() {
    Art.avatar( Dom.get( "avatar-preview" ) , this.me );
    Art.avatar( Dom.get( "open-settings" ) , this.me );
  },

  // Fill an icon grid and a color grid for one person. handlers.person()
  // is who is being edited ({ display_name, icon, color }), preview( person )
  // draws a trial look while the hue slider moves, and save( icon , color )
  // stores a choice (resolving once saved). handlers.blank( person ), when
  // given, is what the "no icon" choice shows instead of the initials.
  // Returns { mark } to redraw which choice is checked.
  picker( icons , colors , handlers ) {
    const current = function () { return handlers.person() || {}; };
    const blank = handlers.blank || function ( person ) { return Art.initials( person.display_name ); };
    const choose = async function ( icon , color ) {
      await handlers.save( icon , color );
      mark();
    };

    Dom.clear( icons );
    // The first choice is "no icon": the person's initials.
    [ "" ].concat( this.options.icons ).forEach( function ( icon ) {
      const button = Dom.el( "button" , {
        class: "icon-choice" + ( icon === "" ? " is-initials" : "" ),
        text: icon || blank( current() ),
        attrs: { type: "button" , role: "radio" , "aria-label": icon || I18n.get( "profile.initials" ) },
        on: { click: function () { choose( icon , current().color || "" ); } },
      } );
      button.setAttribute( "data-icon" , icon );
      icons.appendChild( button );
    } );

    Dom.clear( colors );
    const grid = Dom.el( "div" , { class: "color-grid" , attrs: { role: "radiogroup" , "aria-label": I18n.get( "profile.color" ) } } );
    [ "" ].concat( this.options.colors ).forEach( function ( color ) {
      const button = Dom.el( "button" , {
        class: "color-choice" + ( color === "" ? " is-auto" : "" ),
        attrs: { type: "button" , role: "radio" , "aria-label": I18n.get( "profile.color_" + ( color || "auto" ) ) },
        on: { click: function () { choose( current().icon || "" , color ); } },
      } );
      button.setAttribute( "data-color" , color );
      if ( color ) Dom.vars( button , { hue: Art.avatarHues[ color ] } );
      grid.appendChild( button );
    } );

    // Any hue at all, on a slider. Only the hue is free: saturation and
    // lightness stay the stylesheet's, so every avatar reads in both themes.
    const swatch = Dom.el( "span" , { class: "color-choice hue-swatch" , attrs: { "aria-hidden": "true" } } );
    const slider = Dom.el( "input" , { class: "hue-slider" , attrs: {
      type: "range" , min: "0" , max: "359" , step: "1" , "aria-label": I18n.get( "profile.hue_label" ),
    } } );
    const custom = Dom.el( "div" , { class: "hue-row" , children: [
      swatch,
      Dom.el( "label" , { class: "hue-label" , children: [
        Dom.el( "span" , { text: I18n.get( "profile.color_custom" ) } ),
        slider,
      ] } ),
    ] } );
    slider.addEventListener( "input" , function () {
      Dom.vars( swatch , { hue: slider.value } );
      handlers.preview( Object.assign( {} , current() , { color: "h" + slider.value } ) );
    } );
    slider.addEventListener( "change" , function () { choose( current().icon || "" , "h" + slider.value ); } );
    colors.appendChild( grid );
    colors.appendChild( custom );

    const mark = function () {
      const person = current();
      Dom.all( ".icon-choice" , icons ).forEach( function ( button ) {
        button.setAttribute( "aria-checked" , String( button.getAttribute( "data-icon" ) === ( person.icon || "" ) ) );
        if ( button.classList.contains( "is-initials" ) ) Dom.text( button , blank( person ) );
      } );
      Dom.all( ".color-choice[data-color]" , grid ).forEach( function ( button ) {
        button.setAttribute( "aria-checked" , String( button.getAttribute( "data-color" ) === ( person.color || "" ) ) );
      } );
      // The slider starts from the person's color, whatever it is.
      const hue = Art.colorHue( person.color );
      slider.value = String( hue === null ? Art.hue( person.display_name ) : hue );
      Dom.vars( swatch , { hue: slider.value } );
      custom.classList.toggle( "is-chosen" , /^h\d+$/.test( person.color || "" ) );
      handlers.preview( person );
    };
    mark();
    return { mark: mark };
  },

  async save( icon , color ) {
    try {
      await Api.setAvatar( icon , color );
      this.me.icon = icon;
      this.me.color = color;
      this.renderPreview();
    } catch ( error ) {
      Toast.error( error );
    }
  },

  renderPin() {
    Dom.text( Dom.get( "pin-state" ) , I18n.get( this.me.has_pin ? "pin.is_set" : "pin.not_set" ) );
    Dom.show( Dom.get( "pin-remove" ) , this.me.has_pin );
  },

  async onPin( event ) {
    event.preventDefault();
    const pin = Dom.get( "pin-input" ).value.trim();
    try {
      await Api.setPin( pin );
      this.me.has_pin = true;
      Dom.get( "pin-input" ).value = "";
      this.renderPin();
      Toast.show( I18n.get( "pin.saved" ) );
    } catch ( error ) {
      Toast.error( error );
    }
  },

  async removePin() {
    try {
      await Api.setPin( "" );
      this.me.has_pin = false;
      this.renderPin();
    } catch ( error ) {
      Toast.error( error );
    }
  },
};
