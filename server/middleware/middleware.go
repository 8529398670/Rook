// Package middleware holds the cross-cutting protections that apply to every
// request no matter which route answers it. Keeping them here means a route
// handler never has to remember a header or a throttle -- if it is registered
// on the app, it is covered.
package middleware

import (
	errors "errors"
	log "log"
	netip "net/netip"
	path "path"
	strings "strings"
	sync "sync"
	atomic "sync/atomic"
	time "time"

	fiber "github.com/gofiber/fiber/v3"
	helmet "github.com/gofiber/fiber/v3/middleware/helmet"
	limiter "github.com/gofiber/fiber/v3/middleware/limiter"
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"

	config "rook/server/config"
	logs "rook/server/logs"
)

// SecurityHeaders assumes everything the page needs is same-origin, which is
// true for this template on purpose: libraries are downloaded once and served
// from static/vendor/ rather than pulled from a CDN at runtime. That is what
// lets the CSP stay this tight, and it is why adding a <script src> pointing
// at some other origin will visibly break -- the fix is to vendor the file,
// not to loosen the policy.
func SecurityHeaders( cfg *config.Config ) ( handler fiber.Handler ) {
	content_security_policy := "default-src 'self'; " +
		"script-src 'self'; " +
		"style-src 'self'; " +
		"img-src 'self' data:; " +
		"font-src 'self'; " +
		"connect-src 'self'; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"form-action 'self'; " +
		"frame-ancestors 'none'"

	hsts_max_age := 0
	if cfg.SecureCookies {
		// Only claim HTTPS-only when the deployment actually is. Sending HSTS
		// from a local http:// run would pin the browser to https for
		// localhost and break every other project on that port.
		hsts_max_age = 63072000
	}

	handler = helmet.New( helmet.Config{
		ContentSecurityPolicy:     content_security_policy,
		XFrameOptions:             "DENY",
		ContentTypeNosniff:        "nosniff",
		ReferrerPolicy:            "same-origin",
		PermissionPolicy:          "geolocation=(), microphone=(), camera=(), payment=(), usb=()",
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginResourcePolicy: "same-origin",
		XDNSPrefetchControl:       "off",
		HSTSMaxAge:                hsts_max_age,
		HSTSExcludeSubdomains:     false,
	} )
	return
}

// RateLimit is a per-process, in-memory limiter. That is exactly right for
// the single container dockerRun.sh builds. Run more than one replica and the
// counters stop being shared, so each replica would allow the full budget --
// move to a shared store (limiter.Config.Storage) at that point rather than
// assuming the limit still holds.
//
// Only the API and login links count against it. The page's own files are
// static and cheap, and one page load fetches some forty of them -- counted,
// two or three loads from one household's IP (signing up through a link
// reloads the app) used the whole budget and locked everyone there out.
//
// A budget belongs to an address, or for IPv6 to its /64 (rateKey). It writes
// to logger (nil is fine) and the server's own log: the first 429 an address
// gets in a window, so a lockout can be seen, and once per process when a
// request carries a proxy's address header while server.trust_proxy is off
// (warnProxyHeaders).
func RateLimit( cfg *config.Config , logger *logs.Logger ) ( handler fiber.Handler ) {
	refusals := &limitNotices{ last: map[ string ]time.Time{} }
	var warned atomic.Bool
	limited := limiter.New( limiter.Config{
		Max:        cfg.RateLimitMax,
		Expiration: cfg.RateLimitWindow,
		Next: func( c fiber.Ctx ) ( skip bool ) {
			request_path := c.Path()
			skip = strings.HasPrefix( request_path , "/api/" ) == false && strings.HasPrefix( request_path , "/login/" ) == false
			return
		},
		KeyGenerator: func( c fiber.Ctx ) ( result string ) {
			result = rateKey( c.IP() )
			return
		},
		LimitReached: func( c fiber.Ctx ) ( err error ) {
			refusals.note( c , logger , "general" , rateKey( c.IP() ) , cfg.RateLimitWindow )
			err = c.Status( fiber.StatusTooManyRequests ).JSON( fiber.Map{ "error": "too many requests" } )
			return
		},
	} )
	handler = func( c fiber.Ctx ) ( err error ) {
		if warned.Load() == false { warnProxyHeaders( c , cfg , logger , &warned ) }
		err = limited( c )
		return
	}
	return
}

// normalisedPath is a request path the way the router matches it: in any
// case, and with or without a trailing slash. Whatever decides by path (the
// request log's skip list) reads this, so no spelling of a path gets a
// different answer from the route it reaches.
func normalisedPath( request_path string ) ( normalised string ) {
	normalised = strings.ToLower( request_path )
	if len( normalised ) > 1 {
		normalised = strings.TrimRight( normalised , "/" )
	}
	return
}

// rateKey is who a budget belongs to: an IPv4 address as it is, an IPv6
// address by its /64. Anything that does not parse is used as it is.
func rateKey( ip string ) ( key string ) {
	key = ip
	address , err := netip.ParseAddr( ip )
	if err != nil { return }
	address = address.Unmap()
	if address.Is4() {
		key = address.String()
		return
	}
	prefix , err := address.WithZone( "" ).Prefix( 64 )
	if err != nil { return }
	key = prefix.String()
	return
}

// proxyHeaders are the headers a proxy names the visitor's address in.
var proxyHeaders = []string{ "CF-Connecting-IP" , "X-Forwarded-For" , "X-Real-IP" }

// warnProxyHeaders says, once per process, that a request came through a
// proxy the server does not trust. Then every visitor has the proxy's address,
// so they all share one budget: 60 sign-in attempts a minute from anyone
// would lock everyone out. That is the default behind Cloudflare Tunnel or a
// local proxy until server.trust_proxy is set.
func warnProxyHeaders( c fiber.Ctx , cfg *config.Config , logger *logs.Logger , warned *atomic.Bool ) {
	if cfg.TrustProxy { return }
	for _ , name := range proxyHeaders {
		if c.Get( name ) == "" { continue }
		if warned.Swap( true ) { return }
		message := "requests carry " + name + " but server.trust_proxy is off, so every visitor is counted as the proxy's address " +
			c.IP() + " and they share one rate limit; see server.trust_proxy in config.yaml"
		log.Printf( "[rate-limit] %s" , message )
		if logger != nil { logger.System( logs.LevelWarn , message , map[string]any{ "header": name , "peer": c.IP() } ) }
		return
	}
}

// limitNotices remembers when each budget last said it was used up, so a
// flood of refused requests writes one line per budget per window, not one
// each.
type limitNotices struct {
	lock sync.Mutex
	last map[ string ]time.Time
}

func ( notices *limitNotices ) note( c fiber.Ctx , logger *logs.Logger , budget string , key string , window time.Duration ) {
	now := time.Now()
	bucket := budget + ":" + key
	notices.lock.Lock()
	if now.Sub( notices.last[ bucket ] ) < window {
		notices.lock.Unlock()
		return
	}
	if len( notices.last ) > 10000 {
		for name , when := range notices.last {
			if now.Sub( when ) >= window { delete( notices.last , name ) }
		}
	}
	notices.last[ bucket ] = now
	notices.lock.Unlock()
	log.Printf( "[rate-limit] %s budget used up for %s (%s %s); more 429s for it this window are not logged" , budget , key , c.Method() , c.Path() )
	if logger != nil {
		logger.System( logs.LevelWarn , "rate limit reached" , map[string]any{ "budget": budget , "key": key , "method": c.Method() , "path": c.Path() , "window_seconds": int( window.Seconds() ) } )
	}
}

// Recover keeps one panicking handler from taking the process down with it.
// Stack traces stay off by default: they are enormously useful in a terminal
// and a gift to an attacker if they ever reach a response body.
func Recover( cfg *config.Config ) ( handler fiber.Handler ) {
	handler = recoverer.New( recoverer.Config{ EnableStackTrace: cfg.SecureCookies == false } )
	return
}

// errorStatus is the status a handler's error turns into: a fiber.Error's
// own code, 500 for anything else. The activity log records it.
func errorStatus( err error ) ( code int ) {
	code = fiber.StatusInternalServerError
	var fiber_err *fiber.Error
	if errors.As( err , &fiber_err ) {
		code = fiber_err.Code
	}
	return
}

// staticAsset is a request for one of the page's own files (/js, /css, ...),
// which the activity log leaves out unless logs.static_assets is on.
func staticAsset( method string , request_path string ) ( result bool ) {
	if method != fiber.MethodGet && method != fiber.MethodHead { return }
	for _ , prefix := range []string{ "/api/" , "/login/" , "/join/" } {
		if strings.HasPrefix( request_path , prefix ) { return }
	}
	result = path.Ext( request_path ) != ""
	return
}
