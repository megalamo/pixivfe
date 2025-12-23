package cookie

func IsHttpOnly(name CookieName) bool {
	return name != OpenAllButtonCookie
}
