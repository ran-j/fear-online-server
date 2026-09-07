package launcher

import (
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/go-ozzo/ozzo-validation/is"
)

type oauthLoginRequest struct {
	SteamID       string
	SessionTicket string
}

func (r oauthLoginRequest) Validate() error {
	return validation.ValidateStruct(&r,
		validation.Field(&r.SteamID, validation.Required, validation.Length(1, 20), is.Digit),
	)
}
