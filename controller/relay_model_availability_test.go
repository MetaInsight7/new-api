package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRejectOfficiallyUnsupportedModel(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ModelAvailabilityState{}))

	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	require.NoError(t, db.Create(&model.ModelAvailabilityState{
		ModelName: "retired-model",
		Status:    model.ModelAvailabilityOfficialUnsupported,
		Reason:    "official_model_unsupported",
	}).Error)

	apiErr := rejectOfficiallyUnsupportedModel(&relaycommon.RelayInfo{OriginModelName: "retired-model"})
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeModelOfficiallyUnsupported, apiErr.GetErrorCode())
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.True(t, types.IsSkipRetryError(apiErr))
	assert.False(t, types.IsRecordErrorLog(apiErr))
	assert.Contains(t, apiErr.Error(), "retired-model")
	assert.Contains(t, apiErr.Error(), "official_model_unsupported")
}

func TestRejectOfficiallyUnsupportedModelAllowsUnconfirmedStates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ModelAvailabilityState{}))

	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	for _, status := range []string{
		model.ModelAvailabilityAvailable,
		model.ModelAvailabilityTemporaryFailure,
		model.ModelAvailabilityUnknownFailure,
	} {
		t.Run(status, func(t *testing.T) {
			modelName := "model-" + status
			require.NoError(t, db.Create(&model.ModelAvailabilityState{
				ModelName: modelName,
				Status:    status,
			}).Error)
			assert.Nil(t, rejectOfficiallyUnsupportedModel(&relaycommon.RelayInfo{OriginModelName: modelName}))
		})
	}
}

func TestRejectOfficiallyUnsupportedModelAllowsMissingState(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.ModelAvailabilityState{}))

	previousDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	assert.Nil(t, rejectOfficiallyUnsupportedModel(&relaycommon.RelayInfo{OriginModelName: "unseen-model"}))
	assert.Nil(t, rejectOfficiallyUnsupportedModel(nil))
}
