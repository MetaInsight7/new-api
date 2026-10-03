package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestValidateChannelGroupPriority(t *testing.T) {
	valid := model.Channel{
		Type:          constant.ChannelTypeOpenAI,
		GroupPriority: common.GetPointer(`{"vip":{"priority":-2,"weight":0}}`),
	}
	require.NoError(t, validateChannel(&valid, false))

	invalidWeight := model.Channel{
		Type:          constant.ChannelTypeOpenAI,
		GroupPriority: common.GetPointer(`{"vip":{"weight":-1}}`),
	}
	require.ErrorContains(t, validateChannel(&invalidWeight, false), "group_priority")

	unknownField := model.Channel{
		Type:          constant.ChannelTypeOpenAI,
		GroupPriority: common.GetPointer(`{"vip":{"route":"fast"}}`),
	}
	require.ErrorContains(t, validateChannel(&unknownField, false), "group_priority")
}
