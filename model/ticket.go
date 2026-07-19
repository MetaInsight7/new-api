package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrTicketImageSlotFull = errors.New("message already has 3 images")
var ErrTicketImageUploadExpired = errors.New("upload window expired (15 minutes)")
var ErrTicketImagePermission = errors.New("no permission to upload to this message")

type Ticket struct {
	Id         int    `json:"id"`
	UserId     int    `json:"user_id" gorm:"index"`
	Username   string `json:"username" gorm:"type:varchar(64);default:''"`
	TicketNo   string `json:"ticket_no" gorm:"type:varchar(32);index"`
	Subject    string `json:"subject" gorm:"type:varchar(255)"`
	Department string `json:"department" gorm:"type:varchar(32);default:'technical'"`
	Priority   string `json:"priority" gorm:"type:varchar(16);default:'medium'"`
	Status     string `json:"status" gorm:"type:varchar(32);default:'waiting_support';index"`
	Unread     bool   `json:"unread" gorm:"default:false"`
	CreatedAt  int64  `json:"created_at" gorm:"bigint;index"`
	UpdatedAt  int64  `json:"updated_at" gorm:"bigint"`

	Messages []TicketMessage `json:"messages,omitempty" gorm:"-:all"`
}

type TicketMessage struct {
	Id        int    `json:"id"`
	TicketId  int    `json:"ticket_id" gorm:"index"`
	Sender    string `json:"sender" gorm:"type:varchar(16)"`
	Author    string `json:"author" gorm:"type:varchar(64)"`
	Content   string `json:"content" gorm:"type:text"`
	CreatedAt int64  `json:"created_at" gorm:"bigint"`

	Images []TicketImage `json:"images,omitempty" gorm:"-:all"`
}

type TicketImage struct {
	Id          int    `json:"id"`
	TicketId    int    `json:"ticket_id" gorm:"index"`
	MessageId   int    `json:"message_id" gorm:"uniqueIndex:idx_msg_slot"`
	Slot        int    `json:"slot" gorm:"uniqueIndex:idx_msg_slot"`
	UserId      int    `json:"user_id" gorm:"index"`
	Filename    string `json:"filename" gorm:"type:varchar(255)"`
	StorageKey  string `json:"-" gorm:"type:varchar(255)"`
	ContentType string `json:"-" gorm:"type:varchar(64)"`
	Size        int64  `json:"size"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
}

func GenerateTicketNo(ticketId int) string {
	now := time.Now()
	return fmt.Sprintf("TK-%s%04d", now.Format("20060102"), ticketId)
}

// InsertWithMessage creates ticket + first message in one transaction.
func (t *Ticket) InsertWithMessage(msg *TicketMessage) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(t).Error; err != nil {
			return err
		}
		t.TicketNo = GenerateTicketNo(t.Id)
		if err := tx.Model(t).Update("ticket_no", t.TicketNo).Error; err != nil {
			return err
		}
		msg.TicketId = t.Id
		return tx.Create(msg).Error
	})
}

// InsertReply creates a reply message and updates ticket status/unread in one transaction.
// Returns error if ticket doesn't exist or is already closed/resolved.
func InsertReply(msg *TicketMessage, ticketId int, newStatus string, unread bool) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		// Verify ticket exists and is not closed/resolved (with lock on row)
		var ticket Ticket
		lockQ := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND status NOT IN ?", ticketId, []string{"closed", "resolved"})
		if err := lockQ.First(&ticket).Error; err != nil {
			return fmt.Errorf("工单不存在或已关闭")
		}
		msg.TicketId = ticketId
		if err := tx.Create(msg).Error; err != nil {
			return err
		}
		now := time.Now().Unix()
		updates := map[string]interface{}{
			"status":     newStatus,
			"updated_at": now,
			"unread":     unread,
		}
		result := tx.Model(&Ticket{}).Where("id = ?", ticketId).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("工单状态更新失败")
		}
		return nil
	})
}

// InsertImageWithSlot validates conditions and inserts image within a transaction.
// Only retries on SQLite busy/locked or unique constraint violations (bounded, 3 attempts).
func InsertImageWithSlot(img *TicketImage, ticketId int, messageId int, userId int, isAdmin bool) (int, error) {
	var slot int
	var lastErr error

	for attempt := 0; attempt < 3; attempt++ {
		// Use a copy to avoid GORM reusing mutated Id/Slot
		imgCopy := *img
		slot, lastErr = tryInsertImageSlot(&imgCopy, ticketId, messageId, userId, isAdmin)
		if lastErr == nil {
			// Backfill original
			img.Id = imgCopy.Id
			img.Slot = imgCopy.Slot
			img.TicketId = imgCopy.TicketId
			img.MessageId = imgCopy.MessageId
			img.UserId = imgCopy.UserId
			return slot, nil
		}
		if errors.Is(lastErr, ErrTicketImageSlotFull) ||
			errors.Is(lastErr, ErrTicketImageUploadExpired) ||
			errors.Is(lastErr, ErrTicketImagePermission) {
			return 0, lastErr
		}
		// Only retry if it looks like a busy/lock/constraint error
		errLower := strings.ToLower(lastErr.Error())
		if !strings.Contains(errLower, "database is locked") &&
			!strings.Contains(errLower, "database table is locked") &&
			!strings.Contains(errLower, "unique constraint") &&
			!strings.Contains(errLower, "duplicate entry") &&
			!strings.Contains(errLower, "duplicate key") {
			return 0, lastErr
		}
		time.Sleep(time.Duration(50*(attempt+1)) * time.Millisecond)
	}
	return 0, lastErr
}

func tryInsertImageSlot(img *TicketImage, ticketId int, messageId int, userId int, isAdmin bool) (int, error) {
	var slot int
	err := DB.Transaction(func(tx *gorm.DB) error {
		// Lock the message row (MySQL/PG row lock; SQLite serializes via write txn)
		var msg TicketMessage
		lockClause := clause.Locking{Strength: "UPDATE"}
		query := tx.Clauses(lockClause).Where("id = ? AND ticket_id = ?", messageId, ticketId)
		if err := query.First(&msg).Error; err != nil {
			return ErrTicketImagePermission
		}

		// Verify sender role
		if isAdmin {
			if msg.Sender != "support" {
				return ErrTicketImagePermission
			}
		} else {
			if msg.Sender != "user" {
				return ErrTicketImagePermission
			}
			var ticket Ticket
			if err := tx.Select("user_id").First(&ticket, ticketId).Error; err != nil {
				return ErrTicketImagePermission
			}
			if ticket.UserId != userId {
				return ErrTicketImagePermission
			}
		}

		// Verify 15-minute window
		if time.Now().Unix()-msg.CreatedAt > 15*60 {
			return ErrTicketImageUploadExpired
		}

		// Find smallest available slot
		var existing []TicketImage
		tx.Where("message_id = ?", messageId).Select("slot").Find(&existing)
		usedSlots := make(map[int]bool)
		for _, e := range existing {
			usedSlots[e.Slot] = true
		}
		assigned := 0
		for s := 1; s <= 3; s++ {
			if !usedSlots[s] {
				assigned = s
				break
			}
		}
		if assigned == 0 {
			return ErrTicketImageSlotFull
		}

		img.TicketId = ticketId
		img.MessageId = messageId
		img.Slot = assigned
		img.UserId = userId
		slot = assigned
		return tx.Create(img).Error
	})
	return slot, err
}

func GetUserTicketsPaged(userId int, startIdx int, num int, status string, keyword string) ([]*Ticket, int64, error) {
	var tickets []*Ticket
	var total int64
	query := DB.Model(&Ticket{}).Where("user_id = ?", userId)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		kw := "%" + keyword + "%"
		query = query.Where("(subject LIKE ? OR ticket_no LIKE ? OR department LIKE ?)", kw, kw, kw)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("updated_at desc").Limit(num).Offset(startIdx).Find(&tickets).Error
	return tickets, total, err
}

func GetTicketById(id int) (*Ticket, error) {
	var ticket Ticket
	if err := DB.First(&ticket, "id = ?", id).Error; err != nil {
		return nil, err
	}
	var messages []TicketMessage
	if err := DB.Where("ticket_id = ?", id).Order("created_at asc").Find(&messages).Error; err != nil {
		return nil, err
	}
	if len(messages) > 0 {
		msgIds := make([]int, len(messages))
		for i, m := range messages {
			msgIds[i] = m.Id
		}
		imageMap, _ := GetImagesByMessageIds(msgIds)
		for i := range messages {
			messages[i].Images = imageMap[messages[i].Id]
		}
	}
	ticket.Messages = messages
	return &ticket, nil
}

func GetAllTickets(startIdx int, num int, status string, priority string, keyword string) ([]*Ticket, int64, error) {
	var tickets []*Ticket
	var total int64
	query := DB.Model(&Ticket{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if priority != "" {
		query = query.Where("priority = ?", priority)
	}
	if keyword != "" {
		kw := "%" + keyword + "%"
		query = query.Where("(subject LIKE ? OR ticket_no LIKE ? OR department LIKE ? OR username LIKE ?)", kw, kw, kw, kw)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("updated_at desc").Limit(num).Offset(startIdx).Find(&tickets).Error
	return tickets, total, err
}

func (t *Ticket) SetUnread(unread bool) error {
	return DB.Model(t).Update("unread", unread).Error
}

func GetMessageById(id int) (*TicketMessage, error) {
	var msg TicketMessage
	err := DB.First(&msg, "id = ?", id).Error
	return &msg, err
}

func GetTicketOwnerId(ticketId int) (int, error) {
	var ticket Ticket
	err := DB.Select("user_id").First(&ticket, "id = ?", ticketId).Error
	return ticket.UserId, err
}

func GetTicketImageById(id int) (*TicketImage, error) {
	var img TicketImage
	err := DB.First(&img, "id = ?", id).Error
	return &img, err
}

func GetImagesByMessageIds(messageIds []int) (map[int][]TicketImage, error) {
	var images []TicketImage
	err := DB.Where("message_id IN ?", messageIds).Order("slot asc").Find(&images).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int][]TicketImage)
	for _, img := range images {
		result[img.MessageId] = append(result[img.MessageId], img)
	}
	return result, nil
}

func CountUserOpenTickets(userId int) (int64, error) {
	var count int64
	err := DB.Model(&Ticket{}).
		Where("user_id = ? AND status NOT IN ?", userId, []string{"closed", "resolved"}).
		Count(&count).Error
	return count, err
}

func GetUserLastTicketTime(userId int) (int64, error) {
	var ticket Ticket
	err := DB.Where("user_id = ?", userId).Order("created_at desc").First(&ticket).Error
	if err != nil {
		return 0, err
	}
	return ticket.CreatedAt, nil
}

func CountRecentMessages(ticketId int, seconds int64) (int64, error) {
	var count int64
	since := time.Now().Unix() - seconds
	err := DB.Model(&TicketMessage{}).
		Where("ticket_id = ? AND sender = 'user' AND created_at >= ?", ticketId, since).
		Count(&count).Error
	return count, err
}
