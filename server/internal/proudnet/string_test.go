package proudnet_test

import (
	"testing"

	"go-service-template/internal/proudnet"

	"github.com/stretchr/testify/assert"
)

func TestProudStringRoundTrip(t *testing.T) {
	for _, value := range []string{"", "76561198114269084", "Player69084", "áé走"} {
		encoded := proudnet.WriteProudString(nil, value)
		decoded, next, err := proudnet.ReadProudString(encoded, 0)
		assert.NoError(t, err, value)
		assert.Equal(t, value, decoded, value)
		assert.Equal(t, len(encoded), next, value)
	}
}

func TestReadProudStringTwoInARow(t *testing.T) {
	buf := proudnet.WriteProudString(nil, "code123")
	buf = proudnet.WriteProudString(buf, "76561198114269084")

	first, next, err := proudnet.ReadProudString(buf, 0)
	assert.NoError(t, err)
	assert.Equal(t, "code123", first)

	second, _, err := proudnet.ReadProudString(buf, next)
	assert.NoError(t, err)
	assert.Equal(t, "76561198114269084", second)
}
