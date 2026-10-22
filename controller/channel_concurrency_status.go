package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const channelConcurrencyQueryMaxIDs = 200

type channelConcurrencyStatus struct {
	ChannelID      int  `json:"channel_id"`
	MaxConcurrency int  `json:"max_concurrency"`
	Active         int  `json:"active"`
	Waiting        int  `json:"waiting"`
	CoolingDown    bool `json:"cooling_down"`
}

func GetChannelConcurrencyStatus(c *gin.Context) {
	ids := make([]int, 0)
	fragmentCount := 0
	for fragment := range strings.SplitSeq(c.Query("ids"), ",") {
		fragmentCount++
		if fragmentCount > channelConcurrencyQueryMaxIDs {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "too many channel ids"})
			return
		}
		id, err := strconv.Atoi(strings.TrimSpace(fragment))
		if err == nil && id > 0 {
			ids = append(ids, id)
		}
	}

	if len(ids) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []channelConcurrencyStatus{}})
		return
	}

	channels, err := model.GetChannelsByIds(ids)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	limitedChannels := make([]*model.Channel, 0, len(channels))
	for _, channel := range channels {
		if channel != nil && channel.GetMaxConcurrency() > 0 {
			limitedChannels = append(limitedChannels, channel)
		}
	}
	if len(limitedChannels) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": []channelConcurrencyStatus{}})
		return
	}

	loads, err := service.GetChannelConcurrencyLoadsAuthoritative(c.Request.Context(), limitedChannels)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data":    []channelConcurrencyStatus{},
			"stale":   true,
			"reason":  "concurrency snapshot unavailable",
		})
		return
	}
	statuses := make([]channelConcurrencyStatus, 0, len(limitedChannels))
	for _, channel := range limitedChannels {
		load := loads[channel.Id]
		statuses = append(statuses, channelConcurrencyStatus{
			ChannelID:      channel.Id,
			MaxConcurrency: channel.GetMaxConcurrency(),
			Active:         load.Active,
			Waiting:        load.Waiting,
			CoolingDown:    load.CoolingDown,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": statuses, "stale": false, "reason": ""})
}
