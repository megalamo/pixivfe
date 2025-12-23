package untrusted

// not a good idea. HTTP Cookie is too wonky to store arbitrary binary data efficiently.

// https://datatracker.ietf.org/doc/html/rfc6265#section-4.1.1
//  cookie-octet      = %x21 / %x23-2B / %x2D-3A / %x3C-5B / %x5D-7E
//                        ; US-ASCII characters excluding CTLs,
//                        ; whitespace DQUOTE, comma, semicolon,
//                        ; and backslash
//
// %x21: 1 character (!)
// %x23-2B: 9 characters (#$%&'()*+)
// %x2D-3A: 14 characters (-./0123456789:)
// %x3C-5B: 32 characters (<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[)
// %x5D-7E: 34 characters (]^_abcdefghijklmnopqrstuvwxyz{|}~`)
//
// Total characters:
// 1+9+14+32+34=90

// import (
// 	"compress/"
// 	"net/http"

// 	"codeberg.org/pixivfe/pixivfe/v3/core/cookie"
// )

// func cookieEscape(raw string) string {
//
// }

// func GetCompressedCookie(r *http.Request, name cookie.CookieName) string {
// 	cookie, err := r.Cookie(string(name))
// 	if err != nil {
// 		return ""
// 	}

// 	value, err :=
// 	if err != nil {
// 		return ""
// 	}

// 	return value
// }

// func SetCompressedCookie(w http.ResponseWriter, r *http.Request, name cookie.CookieName, value string) {
// 	if value == "" {
// 		ClearCookie(w, r, name)
// 	} else {
// 		cookie := createCookieUnencoded(
// 			name, url.QueryEscape(value),
// 			time.Now().Add(cookieMaxAge),
// 			utils.IsConnectionSecure(r))
// 		http.SetCookie(w, &cookie)
// 	}
// }
