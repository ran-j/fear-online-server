package proudnet

// The client asks permission for these actions and only needs an ack back.
var staticResponses = map[uint16]response{
	0x759A: {answer: 0x75DF}, // RequestLoginLevel
	0x759B: {answer: 0x75E1}, // RequestLoginPVP
	0x759C: {answer: 0x75E3}, // RequestLoginPVE
	0x759D: {answer: 0x75E5}, // RequestLoginHit
	0x759E: {answer: 0x75E7}, // RequestLoginRank
	0x75F9: {answer: 0x762B},
	0x765D: {answer: 0x768F},
	0x765F: {answer: 0x7694},
	0x7725: {answer: 0x7757},
	0x7789: {answer: 0x77BB},
	0x7919: {answer: 0x794B},
	0x797D: {answer: 0x79AF},
	0x9C41: {answer: 0x9CA5},
	0xA411: {answer: 0xA605},
	0xA412: {answer: 0xA607}, // RequestOpenState
	0xA413: {answer: 0xA609}, // RequestCloseState

	// Hangar screen open/close: item (equip/inventory), recipe (craft) and perk menus.
	0x9859: {answer: 0x98BD}, // ItemC2S::RequestOpen
	0x985A: {answer: 0x98BF}, // ItemC2S::RequestClose
	0x99E9: {answer: 0x9A4D}, // RecipeC2S::RequestOpen
	0x99EA: {answer: 0x9A4F}, // RecipeC2S::RequestOpenNew
	0x99EB: {answer: 0x9A51}, // RecipeC2S::RequestClose
	0x9AB1: {answer: 0x9B15}, // PerkC2S::RequestOpen
	0x9AB2: {answer: 0x9B17}, // PerkC2S::RequestOpenNew
	0x9AB3: {answer: 0x9B19}, // PerkC2S::RequestClose
}

var empty = []byte{0x00}

func NewGameActionHandle(server *Server) {
	for request, resp := range staticResponses {
		resp := resp
		server.Handle(request, func(session *Session, _ Message) error {
			if err := session.SendPlain(Message{ID: resp.answer, Body: resp.body}); err != nil {
				return err
			}

			return nil
		})
	}
}
