package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountPasswordBlocklistMeetsASVSMinimum(t *testing.T) {
	assert.GreaterOrEqual(t, len(accountPasswordBlocklist), 3000)
}

func TestValidateNewAccountPasswordRejectsCommonAndPredictablePasswords(t *testing.T) {
	for _, password := range []string{
		"password123",
		"Password123!",
		"qwerty123",
		"aaaaaaaaaaaaaaaa",
		"1234567890123456",
	} {
		t.Run(password, func(t *testing.T) {
			require.ErrorIs(t, ValidateNewAccountPassword(password), ErrAccountPasswordCommon)
		})
	}

	assert.NoError(t, ValidateNewAccountPassword("Saffron-river telescope 47"))
	assert.ErrorIs(t, ValidateNewAccountPassword("seven77"), ErrAccountPasswordLength)
	assert.ErrorIs(t, ValidateNewAccountPassword(strings.Repeat("界", 129)), ErrAccountPasswordLength)
}

func TestHistoricalCommonPasswordLoginRemainsCompatible(t *testing.T) {
	hash, err := Password2Hash("123456")
	require.NoError(t, err)

	assert.True(t, ValidatePasswordAndHash("123456", hash))
}
