package model

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChannelTestAlertCooldown struct {
	ChannelID        int    `gorm:"primaryKey;autoIncrement:false"`
	LastAt           int64  `gorm:"not null;default:0"`
	PendingAt        int64  `gorm:"not null;default:0"`
	ReservationToken string `gorm:"type:varchar(64);not null;default:''"`
}

type ChannelTestAlertReservation struct {
	ChannelID        int
	PendingAt        int64
	ReservationToken string
}

func ReserveChannelTestAlertCooldown(ctx context.Context, channelID int, cooldown, pendingTTL time.Duration, token string) (*ChannelTestAlertReservation, bool, error) {
	if DB == nil || channelID <= 0 || cooldown <= 0 || pendingTTL <= 0 || token == "" || len(token) > 64 {
		return nil, false, fmt.Errorf("invalid channel alert reservation or unavailable database")
	}
	var reservation *ChannelTestAlertReservation
	err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		seed := ChannelTestAlertCooldown{ChannelID: channelID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
			return err
		}
		var record ChannelTestAlertCooldown
		if err := lockForUpdate(tx).First(&record, "channel_id = ?", channelID).Error; err != nil {
			return err
		}
		now, err := alertCooldownDBNowMilli(tx)
		if err != nil {
			return err
		}
		if (record.LastAt > 0 && now-record.LastAt < cooldown.Milliseconds()) || (record.PendingAt > 0 && now-record.PendingAt < pendingTTL.Milliseconds()) {
			return nil
		}
		result := tx.Model(&ChannelTestAlertCooldown{}).Where("channel_id = ?", channelID).Updates(map[string]any{"pending_at": now, "reservation_token": token})
		if result.Error != nil {
			return result.Error
		}
		reservation = &ChannelTestAlertReservation{ChannelID: channelID, PendingAt: now, ReservationToken: token}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return reservation, reservation != nil, nil
}

func CommitChannelTestAlertCooldown(ctx context.Context, reservation *ChannelTestAlertReservation) error {
	if DB == nil || reservation == nil {
		return fmt.Errorf("channel alert reservation is unavailable")
	}
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, err := alertCooldownDBNowMilli(tx)
		if err != nil {
			return err
		}
		result := tx.Model(&ChannelTestAlertCooldown{}).
			Where("channel_id = ? AND pending_at = ? AND reservation_token = ?", reservation.ChannelID, reservation.PendingAt, reservation.ReservationToken).
			Updates(map[string]any{"last_at": now, "pending_at": int64(0), "reservation_token": ""})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("channel alert reservation is no longer owned")
		}
		return nil
	})
}

func RollbackChannelTestAlertCooldown(ctx context.Context, reservation *ChannelTestAlertReservation) error {
	if DB == nil || reservation == nil {
		return nil
	}
	return DB.WithContext(ctx).Model(&ChannelTestAlertCooldown{}).
		Where("channel_id = ? AND pending_at = ? AND reservation_token = ?", reservation.ChannelID, reservation.PendingAt, reservation.ReservationToken).
		Updates(map[string]any{"pending_at": int64(0), "reservation_token": ""}).Error
}
