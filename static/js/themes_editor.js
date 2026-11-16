// The Display screen (#/display): the zoom sliders, then a list of themes
// beside the one being edited. Rook's version of the go-webserver template's editor, drawn with
// Rook's own buttons, cards and segmented controls rather than Bootstrap's.
//
// Everyone gets "My themes": pick one to use, add, duplicate, rename, delete,
// and change any color. Use opens that theme in the editor, and the editor's
// Light/Dark switch puts the theme being edited to use. Until someone changes something they follow the
// app's defaults; the first change saves the whole list to their account and
// from then on it is theirs ("Go back to the defaults" undoes that).
//
// Admins also get "Defaults for everyone": the same editor over the list that
// everyone without their own sees, and that new accounts start from.
//
// Each theme has a mode, four base colors and a button style (corners,
// weight, depth, letters), and keeps a set of colors for each mode: switching
// it to dark and back gets its light colors back. Its Light/Dark is the only
// light/dark switch in the app. It also sets the outlines on the trick's
// led card and winning card. Everything else is worked
// out by theme.js at readable contrast, and shown under "Fine-tune": any of
// those can be set by hand, and Reset puts the worked-out color back. A color
// that leaves text hard to read is flagged where it is, and in a box above.
//
// Rook has no undo for settings, so deleting a theme and going back to the
// defaults ask first; every other change saves the whole list at once.
// Dragging in a color picker paints as it moves and saves once, when the
// picker closes.
const ThemeEditor = {
  screenId: "screen-appearance",
  editors: [],
  // The admin's saved defaults; null while the built-in ones apply.
  defaults: null,
  me: null,
  tab: "mine",
  confirmed: null,

  // The base colors, then the worked-out ones. Labels: theme.color_<key>.
  baseFields: [ "accent" , "second" , "background" , "surface" ],
  fineFields: [ "text" , "background" , "surface" , "nested" , "chrome" , "border" , "link" , "navLink" ,
    "accentFg" , "secondFg" ],
  labelKeys: {
    text: "text" , background: "background" , surface: "surface" , nested: "nested" , chrome: "chrome" ,
    border: "border" , link: "link" , navLink: "nav_link" , accentFg: "accent_fg" , secondFg: "second_fg" ,
    ledRing: "led_ring" , winRing: "win_ring" ,
  },
  // The trick's outlines, in their own section.
  cardFields: [ "ledRing" , "winRing" ],

  // A label for a color of theme.js: its own key, or for a button variant's
  // fill or label, one sentence with the variant's name in it.
  colorLabel( field ) {
    const variant = Theme.variants.find( ( name ) => field.indexOf( Theme.buttonField( name ) ) === 0 );
    if ( !variant ) return I18n.get( "theme.color_" + this.labelKeys[ field ] );
    const part = field.slice( Theme.buttonField( variant ).length ) || "Fill";
    return I18n.get( "theme.color_btn_" + part.toLowerCase() ).replace( "{variant}" , () => I18n.get( "theme.variant_" + variant ) );
  },

  // A line of the hard-to-read box.
  checkLabel( name ) {
    const match = /^button_(.+)$/.exec( name );
    if ( !match ) return I18n.get( "theme.check_" + name );
    return I18n.get( "theme.check_button_variant" ).replace( "{variant}" , () => I18n.get( "theme.variant_" + match[ 1 ] ) );
  },

  init( me ) {
    this.me = me;
    Theme.load( me );
    Dom.get( "appearance-back" ).addEventListener( "click" , function () { App.go( "#/" ); } );
    Dom.get( "theme-confirm-button" ).addEventListener( "click" , () => {
      const resolve = this.confirmed;
      this.confirmed = null;
      Sheet.close( true );
      if ( resolve ) resolve( true );
    } );
    Dom.all( "[data-theme-tab]" ).forEach( ( button ) => {
      button.addEventListener( "click" , () => this.showTab( button.getAttribute( "data-theme-tab" ) ) );
    } );

    const mine = this.create( Dom.get( "theme-editor-mine" ) , {
      kind: "mine",
      themes: () => Theme.list,
      inUse: () => Theme.current,
      use: ( id ) => Theme.pick( id ),
      save: ( next , id ) => Theme.store( next , id ),
      restore: () => ( Theme.custom ? {
        label: "theme.back_to_defaults",
        confirm: "theme.back_to_defaults_confirm",
        run: () => Theme.store( null , Theme.current ),
      } : null ),
      // Paint the page as a color is dragged, when it is the theme in use.
      live: ( theme ) => { if ( theme.id === Theme.current ) Theme.apply( theme ); },
      settle: () => Theme.apply( Theme.find( Theme.current ) ),
    } );
    Theme.onChange.push( () => mine.render() );
    mine.render();

    // The server lets any admin set the defaults (role=ADMIN on the route).
    if ( me.role !== "admin" ) return;
    Dom.show( Dom.get( "theme-tabs" ) , true );
    const defaults = this.create( Dom.get( "theme-editor-defaults" ) , {
      kind: "defaults",
      themes: () => ( this.defaults || Theme.builtIn.map( ( theme ) => Theme.named( theme ) ) ),
      inUse: () => null,
      save: ( next ) => this.saveDefaults( next ),
      restore: () => ( this.defaults ? {
        label: "theme.restore_built_in",
        confirm: "theme.restore_built_in_confirm",
        run: () => this.saveDefaults( null ),
      } : null ),
      live: () => {},
      settle: () => {},
    } );
    defaults.render();
    return Api.themeDefaults().then( ( reply ) => {
      this.defaults = this.savedDefaults( reply );
      defaults.render();
    } ).catch( ( error ) => Toast.error( error ) );
  },

  enter() {
    this.showTab( this.tab );
    document.title = I18n.get( "theme.heading" ) + " · " + I18n.get( "app.title" );
  },

  leave() {
    // A picker dragged and left without closing: back to what is saved.
    Theme.apply( Theme.find( Theme.current ) );
  },

  showTab( tab ) {
    this.tab = tab === "defaults" && this.me && this.me.role === "admin" ? "defaults" : "mine";
    Dom.all( "[data-theme-tab]" ).forEach( ( button ) => {
      button.setAttribute( "aria-selected" , String( button.getAttribute( "data-theme-tab" ) === this.tab ) );
    } );
    Dom.show( Dom.get( "theme-pane-mine" ) , this.tab === "mine" );
    Dom.show( Dom.get( "theme-pane-defaults" ) , this.tab === "defaults" );
  },

  // Asks before something that cannot be taken back. Resolves true or false.
  confirm( heading , body , button ) {
    if ( this.confirmed ) this.confirmed( false );
    Dom.text( Dom.get( "theme-confirm-heading" ) , heading );
    Dom.text( Dom.get( "theme-confirm-body" ) , body );
    Dom.text( Dom.get( "theme-confirm-button" ) , button );
    return new Promise( ( resolve ) => {
      this.confirmed = resolve;
      Sheet.open( "sheet-theme-confirm" , { onClose: () => {
        const waiting = this.confirmed;
        this.confirmed = null;
        if ( waiting ) waiting( false );
      } } );
    } );
  },

  async saveDefaults( next ) {
    const reply = await Api.setThemeDefaults( next , Theme.builtInsSeen() );
    this.defaults = this.savedDefaults( reply );
    // Someone following the defaults sees the new ones at once, admin included.
    if ( !Theme.custom ) Theme.load( { themes: this.defaults , themes_custom: false , themes_builtins: Theme.builtInsSeen() , theme_id: Theme.current } );
    this.editors.forEach( ( editor ) => editor.render() );
  },

  // The admin's saved list from a reply, with any built-in theme added since
  // it was saved; null while the built-in themes apply.
  savedDefaults( reply ) {
    return reply.themes ? Theme.withNewBuiltIns( reply.themes.map( ( theme ) => Theme.copy( theme ) ) , reply.builtins || 0 ) : null;
  },

  newID() {
    return "t" + Math.random().toString( 36 ).slice( 2 , 10 );
  },

  // "#abc", "abc" or "#AABBCC" as #aabbcc; "" for anything else.
  hex( value ) {
    let text = String( value || "" ).trim().toLowerCase();
    if ( text.charAt( 0 ) !== "#" ) text = "#" + text;
    if ( /^#[0-9a-f]{3}$/.test( text ) ) text = "#" + text.slice( 1 ).split( "" ).map( ( c ) => c + c ).join( "" );
    return /^#[0-9a-f]{6}$/.test( text ) ? text : "";
  },

  // One editor over one list. source says where the list comes from and goes.
  create( root , source ) {
    const editor = { root: root , source: source , selected: null };
    const self = this;

    const themes = () => source.themes();
    const selectedTheme = () => {
      const list = themes();
      return list.find( ( theme ) => theme.id === editor.selected ) || list.find( ( theme ) => theme.id === source.inUse() ) || list[ 0 ];
    };
    // A save, with any refusal said and the screen put back as it was.
    const attempt = async ( work ) => {
      try {
        await work();
      } catch ( error ) {
        Toast.error( error );
        source.settle();
        editor.render();
      }
    };
    // The whole list with one theme replaced by change( copy ).
    const edited = ( theme , change ) => themes().map( ( item ) => {
      if ( item.id !== theme.id ) return Theme.copy( item );
      const copy = Theme.copy( item );
      change( copy );
      return copy;
    } );
    const save = ( next , id ) => attempt( () => source.save( next , id === undefined ? source.inUse() : id ) );

    const actions = {
      add() {
        const theme = Theme.named( Theme.builtIn[ 0 ] );
        theme.id = self.newID();
        theme.name = I18n.get( "theme.new_name" );
        editor.selected = theme.id;
        return save( themes().map( ( item ) => Theme.copy( item ) ).concat( theme ) );
      },
      duplicate( theme ) {
        const copy = Theme.copy( theme );
        copy.id = self.newID();
        copy.name = I18n.get( "theme.copy_name" ).replace( "{name}" , () => theme.name ).slice( 0 , 60 );
        const next = [];
        themes().forEach( ( item ) => {
          next.push( Theme.copy( item ) );
          if ( item.id === theme.id ) next.push( copy );
        } );
        editor.selected = copy.id;
        return save( next );
      },
      async remove( theme ) {
        const next = themes().filter( ( item ) => item.id !== theme.id ).map( ( item ) => Theme.copy( item ) );
        if ( next.length === 0 ) return;
        const asked = await self.confirm( I18n.get( "theme.delete_heading" ) ,
          I18n.get( "theme.delete_confirm" ).replace( "{name}" , () => theme.name ) , I18n.get( "theme.delete" ) );
        if ( !asked ) return;
        const inUse = source.inUse();
        if ( editor.selected === theme.id ) editor.selected = null;
        await save( next , inUse === theme.id ? next[ 0 ].id : inUse );
      },
      rename( theme , name ) {
        return save( edited( theme , ( copy ) => { copy.name = name; } ) );
      },
      set( theme , field , value ) {
        return save( edited( theme , ( copy ) => {
          if ( value ) copy[ field ] = value;
          else delete copy[ field ];
        } ) );
      },
      // Light or dark: each keeps its own colors (Theme.setMode), so switching
      // back finds the colors from before. In My themes it also puts the theme
      // to use, so the switch is seen on the page at once (one undo step).
      mode( theme , value ) {
        return save( edited( theme , ( copy ) => { Theme.setMode( copy , value ); } ) , source.use ? theme.id : undefined );
      },
      override( theme , field , value ) {
        return save( edited( theme , ( copy ) => {
          if ( value ) copy.overrides[ field ] = value;
          else delete copy.overrides[ field ];
        } ) );
      },
      button( theme , key , value ) {
        return save( edited( theme , ( copy ) => {
          copy.button = Object.assign( Theme.button( copy ) , { [ key ]: value } );
        } ) );
      },
      resetFine( theme ) {
        return save( edited( theme , ( copy ) => { copy.overrides = {}; } ) );
      },
    };

    // On a phone the editor sits under the list: bring it into view.
    const showEditor = () => {
      const pane = root.querySelector( ".theme-edit" );
      if ( pane && pane.getBoundingClientRect().top > window.innerHeight * 0.6 ) pane.scrollIntoView( { behavior: "smooth" , block: "start" } );
    };

    // The row for one theme in the list: pick it to edit it, and its buttons.
    const listItem = ( theme , selected , inUse ) => {
      const children = [ self.swatch( theme ) , Dom.el( "span" , { class: "theme-item-name" , text: theme.name } ) ];
      if ( inUse ) children.push( Dom.el( "span" , { class: "theme-in-use" , text: I18n.get( "theme.in_use" ) } ) );
      const pick = Dom.el( "button" , {
        class: "theme-item-pick",
        attrs: { type: "button" , "aria-pressed": selected ? "true" : "false" , "data-focus": "pick:" + theme.id ,
          "aria-label": I18n.get( "theme.edit" ).replace( "{name}" , () => theme.name ) },
        children: children,
        on: { click: () => {
          editor.selected = theme.id;
          editor.render();
          const focus = root.querySelector( "[data-focus='pick:" + theme.id + "']" );
          if ( focus ) focus.focus( { preventScroll: true } );
        } },
      } );
      const buttons = [];
      if ( source.use && !inUse ) {
        // Using a theme opens it in the editor too.
        buttons.push( Dom.el( "button" , { class: "small" , text: I18n.get( "theme.use" ) ,
          attrs: { type: "button" , "data-focus": "use:" + theme.id } , on: { click: async () => {
            editor.selected = theme.id;
            editor.render();
            showEditor();
            await attempt( () => source.use( theme.id ) );
          } } } ) );
      }
      buttons.push( Dom.el( "button" , { class: "small secondary" , text: I18n.get( "theme.duplicate" ) ,
        attrs: { type: "button" , "data-focus": "copy:" + theme.id } , on: { click: () => actions.duplicate( theme ) } } ) );
      if ( themes().length > 1 ) {
        buttons.push( Dom.el( "button" , { class: "small secondary danger-text" , text: I18n.get( "theme.delete" ) ,
          attrs: { type: "button" , "aria-label": I18n.get( "theme.delete_label" ).replace( "{name}" , () => theme.name ) } ,
          on: { click: () => actions.remove( theme ) } } ) );
      }
      return Dom.el( "li" , {
        class: "theme-item" + ( selected ? " selected" : "" ),
        children: [ pick , Dom.el( "div" , { class: "theme-item-actions" , children: buttons } ) ],
      } );
    };

    // A color: its label over the picker, its hex code, and Reset where there
    // is something to go back to. auto: nothing chosen, so the value shown is
    // worked out.
    const colorRow = ( options ) => {
      const id = root.id + "-" + options.key;
      const picker = Dom.el( "input" , { attrs: { type: "color" , id: id , "data-focus": "color:" + options.key } } );
      picker.value = options.value;
      const hex = Dom.el( "input" , { class: "theme-hex" ,
        attrs: { type: "text" , maxlength: "7" , spellcheck: "false" , autocomplete: "off" ,
          "aria-label": options.label + " " + I18n.get( "theme.hex" ) , "data-focus": "hex:" + options.key } } );
      hex.value = options.value;
      picker.addEventListener( "input" , () => {
        hex.value = picker.value;
        options.preview( picker.value );
      } );
      picker.addEventListener( "change" , () => options.save( picker.value ) );
      hex.addEventListener( "change" , () => {
        const value = self.hex( hex.value );
        if ( !value ) {
          hex.setAttribute( "aria-invalid" , "true" );
          return;
        }
        hex.removeAttribute( "aria-invalid" );
        if ( value !== options.value ) options.save( value );
        else hex.value = value;
      } );
      const controls = [ picker , hex ];
      if ( options.reset ) {
        controls.push( Dom.el( "button" , { class: "small secondary" , text: I18n.get( "theme.reset" ) ,
          attrs: { type: "button" , title: I18n.get( "theme.reset_title" ) , "data-focus": "reset:" + options.key } ,
          on: { click: () => options.reset() } } ) );
      } else if ( options.auto ) {
        controls.push( Dom.el( "span" , { class: "muted" , text: I18n.get( "theme.auto" ) } ) );
      }
      const kind = "theme-color-row" + ( options.auto ? " auto" : "" ) + ( options.warn ? " warn" : "" );
      return Dom.el( "div" , { class: kind , children: [
        Dom.el( "label" , { text: options.label , attrs: { for: id } } ),
        Dom.el( "div" , { class: "theme-color-input" , children: controls } ),
      ] } );
    };

    // A segmented row of choices, like the light/dark one.
    const choiceGroup = ( key , label , values , current , change ) => {
      const group = Dom.el( "div" , { class: "segmented" , attrs: { role: "radiogroup" , "aria-label": label } } );
      values.forEach( ( value ) => {
        group.appendChild( Dom.el( "button" , {
          text: I18n.get( "theme." + key + "_" + value ),
          attrs: { type: "button" , role: "radio" , "aria-checked": String( current === value ) , "data-focus": key + ":" + value },
          on: { click: () => { if ( current !== value ) change( value ); } },
        } ) );
      } );
      return Dom.el( "div" , { class: "field theme-choice" , children: [ Dom.el( "span" , { class: "field-label" , text: label } ) , group ] } );
    };

    const editPane = ( theme ) => {
      const colors = Theme.colors( theme );
      const derived = Theme.derived( theme );
      const mode = Theme.modes[ theme.mode ];
      const problems = Theme.check( theme );
      const flagged = new Set();
      problems.forEach( ( problem ) => { flagged.add( problem.front ); flagged.add( problem.back ); } );
      let preview = self.preview( theme );
      // Repaints the preview (and the page, if it is the theme in use) while a
      // picker is dragged; nothing is saved until it closes.
      const previewWith = ( change ) => {
        const copy = Theme.copy( theme );
        change( copy );
        const next = self.preview( copy );
        preview.replaceWith( next );
        preview = next;
        source.live( copy );
      };

      const nameId = root.id + "-name";
      const name = Dom.el( "input" , { attrs: { id: nameId , maxlength: "60" , required: "" , autocomplete: "off" , "data-focus": "name" } } );
      name.value = theme.name || "";
      name.addEventListener( "change" , () => {
        const value = name.value.trim();
        if ( value === "" ) {
          name.value = theme.name || "";
          return;
        }
        if ( value !== theme.name ) actions.rename( theme , value );
      } );

      const modeGroup = choiceGroup( "mode" , I18n.get( "theme.mode" ) , [ "light" , "dark" ] , theme.mode ,
        ( value ) => actions.mode( theme , value ) );

      const base = Dom.el( "div" , { class: "theme-colors" } );
      self.baseFields.forEach( ( field ) => {
        const optional = field === "background" || field === "surface";
        const chosen = theme[ field ] || "";
        base.appendChild( colorRow( {
          key: "base-" + field,
          label: I18n.get( "theme.color_base_" + field ),
          value: chosen || ( optional ? mode[ field ] : "#000000" ),
          auto: optional && !chosen,
          warn: false,
          preview: ( value ) => previewWith( ( copy ) => { copy[ field ] = value; } ),
          save: ( value ) => actions.set( theme , field , value ),
          reset: optional && chosen ? () => actions.set( theme , field , null ) : null,
        } ) );
      } );

      const fineRow = ( field ) => {
        const overridden = Boolean( theme.overrides && theme.overrides[ field ] );
        return colorRow( {
          key: "fine-" + field,
          label: self.colorLabel( field ),
          value: colors[ field ],
          auto: !overridden,
          warn: flagged.has( field ),
          preview: ( value ) => previewWith( ( copy ) => { copy.overrides[ field ] = value; } ),
          save: ( value ) => ( value === derived[ field ] && !overridden ? null : actions.override( theme , field , value ) ),
          reset: overridden ? () => actions.override( theme , field , null ) : null,
        } );
      };
      const fine = Dom.el( "div" , { class: "theme-colors" , children: self.fineFields.map( fineRow ) } );

      // The button's style, then each button color: fill and label.
      const button = Theme.button( theme );
      const choices = Dom.el( "div" , { class: "theme-choices" } );
      Object.keys( Theme.buttonChoices ).forEach( ( key ) => {
        choices.appendChild( choiceGroup( "button_" + key , I18n.get( "theme.button_" + key ) , Object.keys( Theme.buttonChoices[ key ] ) ,
          button[ key ] , ( value ) => {
            previewWith( ( copy ) => { copy.button = Object.assign( Theme.button( copy ) , { [ key ]: value } ); } );
            actions.button( theme , key , value );
          } ) );
      } );
      const buttonColors = Dom.el( "div" , { class: "theme-colors" } );
      Theme.variants.forEach( ( variant ) => {
        const field = Theme.buttonField( variant );
        [ field , field + "Fg" ].forEach( ( one ) => buttonColors.appendChild( fineRow( one ) ) );
      } );

      const cards = Dom.el( "div" , { class: "theme-colors" , children: self.cardFields.map( fineRow ) } );

      const resetAll = Object.keys( theme.overrides || {} ).length > 0 ? Dom.el( "button" , { class: "small secondary" ,
        text: I18n.get( "theme.reset_all" ) , attrs: { type: "button" , "data-focus": "reset-all" } ,
        on: { click: () => actions.resetFine( theme ) } } ) : null;

      const aside = [ preview ];
      if ( problems.length > 0 ) {
        const items = problems.map( ( problem ) => Dom.el( "li" , {
          text: self.checkLabel( problem.name ) + " — " + I18n.get( "theme.ratio" ).replace( "{ratio}" , problem.ratio.toFixed( 1 ) ),
        } ) );
        const box = [ Dom.el( "p" , { text: I18n.get( "theme.hard_to_read" ) } ) , Dom.el( "ul" , { children: items } ) ];
        if ( resetAll ) box.push( resetAll );
        aside.push( Dom.el( "div" , { class: "notice notice-warn theme-problems" , attrs: { role: "status" } , children: box } ) );
      }

      const fineHeader = [ Dom.el( "h3" , { text: I18n.get( "theme.fine_heading" ) } ) ];
      if ( resetAll && problems.length === 0 ) fineHeader.push( resetAll );

      return Dom.el( "div" , { class: "card theme-edit" , children: [
        Dom.el( "div" , { class: "theme-edit-head" , children: [
          Dom.el( "div" , { class: "field" , children: [
            Dom.el( "label" , { text: I18n.get( "theme.name" ) , attrs: { for: nameId } } ),
            name,
          ] } ),
          modeGroup,
        ] } ),
        Dom.el( "div" , { class: "theme-edit-aside" , children: aside } ),
        Dom.el( "h3" , { text: I18n.get( "theme.base_heading" ) } ),
        base,
        Dom.el( "h3" , { text: I18n.get( "theme.buttons_heading" ) } ),
        Dom.el( "p" , { class: "muted" , text: I18n.get( "theme.buttons_help" ) } ),
        choices,
        buttonColors,
        Dom.el( "h3" , { text: I18n.get( "theme.cards_heading" ) } ),
        Dom.el( "p" , { class: "muted" , text: I18n.get( "theme.cards_help" ) } ),
        cards,
        Dom.el( "div" , { class: "theme-fine-head" , children: fineHeader } ),
        Dom.el( "p" , { class: "muted" , text: I18n.get( "theme.fine_help" ) } ),
        fine,
      ] } );
    };

    // Redraws from the list, keeping the focus on the same control.
    editor.render = () => {
      const active = document.activeElement;
      const focus = active && root.contains( active ) ? active.getAttribute( "data-focus" ) : null;
      const list = themes();
      const theme = selectedTheme();
      editor.selected = theme ? theme.id : null;
      const footer = [ Dom.el( "button" , { class: "small" , text: I18n.get( "theme.new" ) ,
        attrs: { type: "button" , "data-focus": "new" } , on: { click: () => actions.add() } } ) ];
      const restore = source.restore();
      if ( restore ) {
        footer.push( Dom.el( "button" , { class: "small secondary" , text: I18n.get( restore.label ) ,
          attrs: { type: "button" , "data-focus": "restore" } , on: { click: async () => {
            const asked = await self.confirm( I18n.get( restore.label ) , I18n.get( restore.confirm ) , I18n.get( restore.label ) );
            if ( asked ) await attempt( restore.run );
          } } } ) );
      }
      const side = Dom.el( "div" , { class: "card theme-list-card" , children: [
        Dom.el( "ul" , { class: "theme-list" , children: list.map( ( item ) => listItem( item , item.id === editor.selected , item.id === source.inUse() ) ) } ),
        Dom.el( "div" , { class: "theme-list-actions" , children: footer } ),
      ] } );
      Dom.clear( root );
      root.appendChild( side );
      if ( theme ) root.appendChild( editPane( theme ) );
      if ( focus ) {
        const again = root.querySelector( "[data-focus='" + focus + "']" );
        if ( again ) again.focus( { preventScroll: true } );
      }
    };

    this.editors.push( editor );
    return editor;
  },

  swatch( theme ) {
    const colors = Theme.colors( theme );
    const swatch = Dom.el( "span" , { class: "theme-swatch" , attrs: { "aria-hidden": "true" } } );
    Dom.vars( swatch , { "swatch-bg": colors.background , "swatch-accent": colors.accent , "swatch-second": colors.second } );
    return swatch;
  },

  // The page in miniature, in this theme's colors: the sidebar with the
  // current item, a card, a card inside it with a link, and Rook's buttons.
  // It carries the theme's own custom properties (Theme.apply on the
  // element), so style.css draws it as the page will be.
  preview( theme ) {
    const span = ( key , kind ) => Dom.el( "span" , { class: kind || "" , text: I18n.get( key ) } );
    const side = Dom.el( "div" , { class: "theme-preview-side" , children: [
      span( "theme.preview_nav" , "theme-preview-current" ),
      span( "theme.preview_nav_other" , "theme-preview-link" ),
    ] } );
    const nested = Dom.el( "div" , { class: "theme-preview-nested" , children: [
      span( "theme.preview_text" ),
      document.createTextNode( " " ),
      span( "theme.preview_link" , "theme-preview-a" ),
    ] } );
    const buttons = Dom.el( "div" , { class: "theme-preview-buttons" , children: [
      Dom.el( "button" , { class: "small" , text: I18n.get( "theme.preview_button" ) , attrs: { type: "button" , tabindex: "-1" } } ),
      Dom.el( "button" , { class: "small secondary" , text: I18n.get( "theme.preview_secondary" ) , attrs: { type: "button" , tabindex: "-1" } } ),
    ].concat( Theme.variants.map( ( variant ) => Dom.el( "button" , { class: "small " + variant ,
      text: I18n.get( "theme.variant_" + variant ) , attrs: { type: "button" , tabindex: "-1" } } ) ) ) } );
    const card = Dom.el( "div" , { class: "theme-preview-card" , children: [
      Dom.el( "strong" , { text: I18n.get( "theme.preview_card" ) } ),
      Dom.el( "span" , { class: "theme-preview-muted" , text: I18n.get( "theme.preview_muted" ) } ),
      nested,
      buttons,
    ] } );
    // The trick's two outlines, on two blank cards.
    const trick = Dom.el( "div" , { class: "theme-preview-trick" , children: [
      Dom.el( "span" , { class: "theme-preview-playcard is-led" , attrs: { title: I18n.get( "theme.color_led_ring" ) } } ),
      Dom.el( "span" , { class: "theme-preview-playcard is-winner" , attrs: { title: I18n.get( "theme.color_win_ring" ) } } ),
    ] } );
    const page = Dom.el( "div" , { class: "theme-preview-page" , children: [ card , trick ] } );
    const preview = Dom.el( "div" , { class: "theme-preview" , attrs: { "aria-hidden": "true" , inert: "" } , children: [ side , page ] } );
    Theme.apply( theme , preview );
    return preview;
  },
};
