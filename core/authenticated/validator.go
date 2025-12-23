package authenticated

import (
	"aidanwoods.dev/go-paseto"
)

// domain seperation key. can be anything. if you change it, past tokens will become invalid.
const Implicit = "PixivFE is the best Pixiv frontend"

func NewSecretKeyHex() string {
	return paseto.NewV4AsymmetricSecretKey().ExportHex()
}

// v4.public validator
type Validator struct {
	SecretKey paseto.V4AsymmetricSecretKey
}

func (psk *Validator) LoadSecretKeyFromHex(hex string) (err error) {
	psk.SecretKey, err = paseto.NewV4AsymmetricSecretKeyFromHex(hex)
	if err != nil {
		return
	}
	// public key can be derived efficiently from SecretKey, so it's not calculated here
	return
}

// TODO: more functions that isn't tied to a global state can be moved here.
// hint: add a helper function to the package 'request_context' to quickly create a sign a token.

// func (psk *Validator) Sign(data any) (encodedToken string, err error) {
// 	token := paseto.NewToken()
// 	token.SetSubject()
// }
