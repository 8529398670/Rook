// Table pictures: a photo someone puts on the game table in a theme (the
// Table section of the Display screen, themes_editor.js). The server keeps
// them (server/models/backgrounds.go); this shrinks a photo before it goes,
// so a 12-megapixel phone picture fits the request size limit, and keeps the
// list of pictures this person may use.
//
// The photo is decoded with createImageBitmap, which also turns it the way
// the camera held it, drawn on a canvas no larger than maxSide on its long
// side, and sent as a JPEG, its quality stepped down (and then its size)
// until it is under maxBytes. The server decodes it again and draws its own
// copy, so nothing but the picture is kept.
const Backgrounds = {
  // GET /api/account/backgrounds: own pictures first, then shared ones.
  list: [],
  loaded: false,
  limit: 6,
  sharedLimit: 12,
  maxSide: 2560,
  // Under the server's 1 MiB request limit, with room to spare.
  maxBytes: 950000,
  // What is suggested, and what is warned about once uploaded.
  bestWidth: 2560,
  bestHeight: 1440,
  smallWidth: 1280,
  onChange: [],

  async load() {
    try {
      const reply = await Api.backgrounds();
      this.list = reply.backgrounds || [];
      this.limit = reply.limit || this.limit;
      this.sharedLimit = reply.shared_limit || this.sharedLimit;
      this.maxSide = reply.max_side || this.maxSide;
      this.loaded = true;
    } catch ( error ) {
      this.list = [];
    }
    this.changed();
  },

  changed() {
    this.onChange.forEach( ( listener ) => listener() );
  },

  find( id ) {
    return this.list.find( ( picture ) => picture.id === id ) || null;
  },

  url( id ) {
    return "/api/backgrounds/" + encodeURIComponent( id );
  },

  // The pictures one editor shows: in My themes, own and shared; in the
  // defaults, shared only (no one else could see an own one).
  usable( shared ) {
    return this.list.filter( ( picture ) => ( shared ? picture.shared : true ) );
  },

  // How many more one may upload there.
  room( shared ) {
    const used = this.list.filter( ( picture ) => ( shared ? picture.shared : picture.mine && !picture.shared ) ).length;
    return ( shared ? this.sharedLimit : this.limit ) - used;
  },

  // A file as a JPEG Blob small enough to send.
  async shrink( file ) {
    let bitmap;
    try {
      bitmap = await createImageBitmap( file );
    } catch ( error ) {
      const refused = new Error( "background_format" );
      refused.code = "background_format";
      throw refused;
    }
    let side = Math.min( this.maxSide , Math.max( bitmap.width , bitmap.height ) );
    try {
      for ( let tries = 0 ; tries < 6 ; tries += 1 ) {
        const scale = side / Math.max( bitmap.width , bitmap.height );
        const width = Math.max( 1 , Math.round( bitmap.width * Math.min( 1 , scale ) ) );
        const height = Math.max( 1 , Math.round( bitmap.height * Math.min( 1 , scale ) ) );
        const canvas = document.createElement( "canvas" );
        canvas.width = width;
        canvas.height = height;
        const context = canvas.getContext( "2d" );
        // A transparent PNG goes on black, as the server would draw it.
        context.fillStyle = "#000000";
        context.fillRect( 0 , 0 , width , height );
        context.imageSmoothingQuality = "high";
        context.drawImage( bitmap , 0 , 0 , width , height );
        for ( const quality of [ 0.86 , 0.78 , 0.7 , 0.6 ] ) {
          const blob = await new Promise( ( resolve ) => canvas.toBlob( resolve , "image/jpeg" , quality ) );
          if ( blob && blob.size <= this.maxBytes ) return blob;
        }
        side = Math.round( side * 0.8 );
      }
    } finally {
      if ( bitmap.close ) bitmap.close();
    }
    const refused = new Error( "background_too_big" );
    refused.code = "background_too_big";
    throw refused;
  },

  // Shrinks and uploads a file; answers the new picture.
  async upload( file , shared ) {
    const blob = await this.shrink( file );
    const picture = await Api.uploadBackground( blob , shared );
    this.list = this.list.filter( ( item ) => item.id !== picture.id );
    const own = this.list.filter( ( item ) => !item.shared );
    const others = this.list.filter( ( item ) => item.shared );
    this.list = picture.shared ? own.concat( others , [ picture ] ) : own.concat( [ picture ] , others );
    this.changed();
    return picture;
  },

  async remove( id ) {
    await Api.deleteBackground( id );
    this.list = this.list.filter( ( picture ) => picture.id !== id );
    this.changed();
  },

  // What a picture could do better, as a language.yaml key, or "".
  advice( picture ) {
    if ( !picture ) return "";
    if ( picture.height > picture.width ) return "theme.picture_portrait";
    if ( picture.width < this.smallWidth ) return "theme.picture_small";
    return "";
  },

  // The part of a picture a screen of this shape shows, as fractions of
  // the picture: background-size cover at background-position x% y%.
  frame( picture , aspect , x , y ) {
    const ratio = picture.width / picture.height;
    const width = aspect >= ratio ? 1 : aspect / ratio;
    const height = aspect >= ratio ? ratio / aspect : 1;
    return { left: ( 1 - width ) * x / 100 , top: ( 1 - height ) * y / 100 , width: width , height: height };
  },
};
