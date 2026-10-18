// Your avatar (an icon and a color, from the server's list) and your
// optional PIN, on the Settings screen.
const Profile = {
  me: null,
  options: null,

  init( me ) {
    this.me = me;
    Dom.get( "pin-form" ).addEventListener( "submit" , this.onPin.bind( this ) );
    Dom.get( "pin-remove" ).addEventListener( "click" , this.removePin.bind( this ) );
  },

  async enter() {
    this.renderPreview();
    this.renderPin();
    if ( this.options ) return;
    try {
      this.options = await Api.profileOptions();
    } catch ( error ) {
      Toast.error( error );
      return;
    }
    this.renderGrids();
  },

  renderPreview() {
    Art.avatar( Dom.get( "avatar-preview" ) , this.me );
    Art.avatar( Dom.get( "open-settings" ) , this.me );
  },

  renderGrids() {
    const icons = Dom.get( "icon-grid" );
    Dom.clear( icons );
    // The first choice is "no icon": your initials.
    [ "" ].concat( this.options.icons ).forEach( function ( icon ) {
      const button = Dom.el( "button" , {
        class: "icon-choice" + ( icon === "" ? " is-initials" : "" ),
        text: icon || Art.initials( Profile.me.display_name ),
        attrs: { type: "button" , role: "radio" , "aria-label": icon || I18n.get( "profile.initials" ) },
        on: { click: function () { Profile.save( icon , Profile.me.color || "" ); } },
      } );
      button.setAttribute( "data-icon" , icon );
      icons.appendChild( button );
    } );

    const colors = Dom.get( "color-grid" );
    Dom.clear( colors );
    [ "" ].concat( this.options.colors ).forEach( function ( color ) {
      const button = Dom.el( "button" , {
        class: "color-choice" + ( color === "" ? " is-auto" : "" ),
        attrs: { type: "button" , role: "radio" , "aria-label": I18n.get( "profile.color_" + ( color || "auto" ) ) },
        on: { click: function () { Profile.save( Profile.me.icon || "" , color ); } },
      } );
      button.setAttribute( "data-color" , color );
      if ( color ) Dom.vars( button , { hue: Art.avatarHues[ color ] } );
      colors.appendChild( button );
    } );
    this.markChoices();
  },

  markChoices() {
    const me = this.me;
    Dom.all( "#icon-grid .icon-choice" ).forEach( function ( button ) {
      button.setAttribute( "aria-checked" , String( button.getAttribute( "data-icon" ) === ( me.icon || "" ) ) );
    } );
    Dom.all( "#color-grid .color-choice" ).forEach( function ( button ) {
      button.setAttribute( "aria-checked" , String( button.getAttribute( "data-color" ) === ( me.color || "" ) ) );
    } );
  },

  async save( icon , color ) {
    try {
      await Api.setAvatar( icon , color );
      this.me.icon = icon;
      this.me.color = color;
      this.renderPreview();
      this.markChoices();
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
