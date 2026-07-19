package model

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testSeq atomic.Int64

func uniqueUserId() int {
	return int(testSeq.Add(1)) + 10000
}

func TestInsertWithMessageTransaction(t *testing.T) {
	uid := uniqueUserId()
	ticket := Ticket{UserId: uid, Username: "test", Subject: "test", Status: "waiting_support"}
	msg := TicketMessage{Sender: "user", Author: "test", Content: "hello"}
	err := ticket.InsertWithMessage(&msg)
	require.NoError(t, err)
	assert.NotZero(t, ticket.Id)
	assert.NotZero(t, msg.Id)
	assert.Equal(t, ticket.Id, msg.TicketId)
	assert.Contains(t, ticket.TicketNo, "TK-")
}

func TestInsertWithMessageRollback(t *testing.T) {
	uid := uniqueUserId()
	// Create a message to occupy a known ID
	setup := Ticket{UserId: uid, Username: "setup", Subject: "setup", Status: "waiting_support"}
	setupMsg := TicketMessage{Sender: "user", Author: "setup", Content: "occupy"}
	require.NoError(t, setup.InsertWithMessage(&setupMsg))
	occupiedMsgId := setupMsg.Id

	// Count tickets before
	var countBefore int64
	DB.Model(&Ticket{}).Where("user_id = ?", uid+1).Count(&countBefore)

	// Force message creation failure by manually inserting a conflicting message ID
	conflict := TicketMessage{Sender: "user", Author: "conflict", Content: "x", CreatedAt: time.Now().Unix()}
	conflict.Id = occupiedMsgId + 1000 // Large ID to not conflict normally
	DB.Create(&conflict)

	// Now try InsertWithMessage with a message that has the conflicting ID set
	badTicket := Ticket{UserId: uid + 1, Username: "bad", Subject: "should-rollback", Status: "waiting_support"}
	badMsg := TicketMessage{Id: conflict.Id, Sender: "user", Author: "bad", Content: "dup"}
	err := badTicket.InsertWithMessage(&badMsg)
	assert.Error(t, err)

	// Verify ticket was NOT created
	var countAfter int64
	DB.Model(&Ticket{}).Where("user_id = ?", uid+1).Count(&countAfter)
	assert.Equal(t, countBefore, countAfter)
}

func TestInsertReplyTransaction(t *testing.T) {
	uid := uniqueUserId()
	ticket := Ticket{UserId: uid, Username: "t", Subject: "s", Status: "waiting_support"}
	firstMsg := TicketMessage{Sender: "user", Author: "t", Content: "init", CreatedAt: time.Now().Unix()}
	require.NoError(t, ticket.InsertWithMessage(&firstMsg))

	reply := TicketMessage{Sender: "support", Author: "admin", Content: "reply", CreatedAt: time.Now().Unix()}
	err := InsertReply(&reply, ticket.Id, "waiting_user", true)
	require.NoError(t, err)
	assert.NotZero(t, reply.Id)

	updated, _ := GetTicketById(ticket.Id)
	assert.Equal(t, "waiting_user", updated.Status)
	assert.True(t, updated.Unread)
}

func TestInsertReplyToClosedTicketFails(t *testing.T) {
	uid := uniqueUserId()
	ticket := Ticket{UserId: uid, Username: "t", Subject: "s", Status: "closed"}
	msg := TicketMessage{Sender: "user", Author: "t", Content: "init", CreatedAt: time.Now().Unix()}
	require.NoError(t, ticket.InsertWithMessage(&msg))

	// Count messages before
	var msgCountBefore int64
	DB.Model(&TicketMessage{}).Where("ticket_id = ?", ticket.Id).Count(&msgCountBefore)

	reply := TicketMessage{Sender: "user", Author: "t", Content: "should fail", CreatedAt: time.Now().Unix()}
	err := InsertReply(&reply, ticket.Id, "waiting_support", false)
	assert.Error(t, err)

	// Verify message count unchanged
	var msgCountAfter int64
	DB.Model(&TicketMessage{}).Where("ticket_id = ?", ticket.Id).Count(&msgCountAfter)
	assert.Equal(t, msgCountBefore, msgCountAfter)
}

func TestInsertReplyToNonexistentTicketFails(t *testing.T) {
	// Count all messages before
	var msgCountBefore int64
	DB.Model(&TicketMessage{}).Where("ticket_id = ?", 99999).Count(&msgCountBefore)

	reply := TicketMessage{Sender: "user", Author: "t", Content: "no ticket", CreatedAt: time.Now().Unix()}
	err := InsertReply(&reply, 99999, "waiting_support", false)
	assert.Error(t, err)

	var msgCountAfter int64
	DB.Model(&TicketMessage{}).Where("ticket_id = ?", 99999).Count(&msgCountAfter)
	assert.Equal(t, msgCountBefore, msgCountAfter)
}

func TestInsertImageWithSlot_Success(t *testing.T) {
	uid := uniqueUserId()
	ticket := Ticket{UserId: uid, Username: "img", Subject: "s", Status: "waiting_support"}
	msg := TicketMessage{Sender: "user", Author: "img", Content: "c", CreatedAt: time.Now().Unix()}
	require.NoError(t, ticket.InsertWithMessage(&msg))

	img := TicketImage{Filename: "a.jpg", StorageKey: fmt.Sprintf("k_%d", uid), ContentType: "image/jpeg", Size: 1000, CreatedAt: time.Now().Unix()}
	slot, err := InsertImageWithSlot(&img, ticket.Id, msg.Id, uid, false)
	require.NoError(t, err)
	assert.Equal(t, 1, slot)
	assert.Equal(t, 1, img.Slot)
}

func TestInsertImageWithSlot_CrossTicketMessage(t *testing.T) {
	uid := uniqueUserId()
	t1 := Ticket{UserId: uid, Username: "u", Subject: "t1", Status: "waiting_support"}
	m1 := TicketMessage{Sender: "user", Author: "u", Content: "c", CreatedAt: time.Now().Unix()}
	require.NoError(t, t1.InsertWithMessage(&m1))

	t2 := Ticket{UserId: uid, Username: "u", Subject: "t2", Status: "waiting_support"}
	m2 := TicketMessage{Sender: "user", Author: "u", Content: "c", CreatedAt: time.Now().Unix()}
	require.NoError(t, t2.InsertWithMessage(&m2))

	img := TicketImage{Filename: "x.jpg", StorageKey: fmt.Sprintf("cross_%d", uid), ContentType: "image/jpeg", Size: 100, CreatedAt: time.Now().Unix()}
	_, err := InsertImageWithSlot(&img, t2.Id, m1.Id, uid, false)
	assert.ErrorIs(t, err, ErrTicketImagePermission)
}

func TestInsertImageWithSlot_WrongSender(t *testing.T) {
	uid := uniqueUserId()
	ticket := Ticket{UserId: uid, Username: "u", Subject: "t", Status: "waiting_support"}
	msg := TicketMessage{Sender: "user", Author: "u", Content: "c", CreatedAt: time.Now().Unix()}
	require.NoError(t, ticket.InsertWithMessage(&msg))

	img := TicketImage{Filename: "x.jpg", StorageKey: fmt.Sprintf("ws_%d", uid), ContentType: "image/jpeg", Size: 100, CreatedAt: time.Now().Unix()}
	_, err := InsertImageWithSlot(&img, ticket.Id, msg.Id, uid+999, true)
	assert.ErrorIs(t, err, ErrTicketImagePermission)
}

func TestInsertImageWithSlot_Expired(t *testing.T) {
	uid := uniqueUserId()
	ticket := Ticket{UserId: uid, Username: "u", Subject: "t", Status: "waiting_support"}
	msg := TicketMessage{Sender: "user", Author: "u", Content: "c", CreatedAt: time.Now().Unix() - 16*60}
	require.NoError(t, ticket.InsertWithMessage(&msg))

	img := TicketImage{Filename: "x.jpg", StorageKey: fmt.Sprintf("exp_%d", uid), ContentType: "image/jpeg", Size: 100, CreatedAt: time.Now().Unix()}
	_, err := InsertImageWithSlot(&img, ticket.Id, msg.Id, uid, false)
	assert.ErrorIs(t, err, ErrTicketImageUploadExpired)
}

func TestInsertImageWithSlot_FourthImage(t *testing.T) {
	uid := uniqueUserId()
	ticket := Ticket{UserId: uid, Username: "u", Subject: "t", Status: "waiting_support"}
	msg := TicketMessage{Sender: "user", Author: "u", Content: "c", CreatedAt: time.Now().Unix()}
	require.NoError(t, ticket.InsertWithMessage(&msg))

	for i := 0; i < 3; i++ {
		img := TicketImage{Filename: "x.jpg", StorageKey: fmt.Sprintf("s4_%d_%d", uid, i), ContentType: "image/jpeg", Size: 100, CreatedAt: time.Now().Unix()}
		_, err := InsertImageWithSlot(&img, ticket.Id, msg.Id, uid, false)
		require.NoError(t, err)
	}

	img := TicketImage{Filename: "x.jpg", StorageKey: fmt.Sprintf("s4_%d_4", uid), ContentType: "image/jpeg", Size: 100, CreatedAt: time.Now().Unix()}
	_, err := InsertImageWithSlot(&img, ticket.Id, msg.Id, uid, false)
	assert.ErrorIs(t, err, ErrTicketImageSlotFull)
}

func TestInsertImageWithSlot_ConcurrentFourth(t *testing.T) {
	uid := uniqueUserId()
	ticket := Ticket{UserId: uid, Username: "conc", Subject: "t", Status: "waiting_support"}
	msg := TicketMessage{Sender: "user", Author: "conc", Content: "c", CreatedAt: time.Now().Unix()}
	require.NoError(t, ticket.InsertWithMessage(&msg))

	var wg sync.WaitGroup
	results := make([]error, 4)
	slots := make([]int, 4)

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			img := TicketImage{Filename: "c.jpg", StorageKey: fmt.Sprintf("conc_%d_%d", uid, idx), ContentType: "image/jpeg", Size: 100, CreatedAt: time.Now().Unix()}
			s, err := InsertImageWithSlot(&img, ticket.Id, msg.Id, uid, false)
			results[idx] = err
			slots[idx] = s
		}(i)
	}
	wg.Wait()

	successCount := 0
	var assignedSlots []int
	for i, err := range results {
		if err == nil {
			successCount++
			assignedSlots = append(assignedSlots, slots[i])
		} else {
			assert.ErrorIs(t, err, ErrTicketImageSlotFull)
		}
	}
	assert.Equal(t, 3, successCount)
	slotSet := make(map[int]bool)
	for _, s := range assignedSlots {
		slotSet[s] = true
	}
	assert.True(t, slotSet[1])
	assert.True(t, slotSet[2])
	assert.True(t, slotSet[3])
}

func TestGetUserTicketsPaged(t *testing.T) {
	uid := uniqueUserId()
	for i := 0; i < 5; i++ {
		tk := Ticket{UserId: uid, Username: "pg", Subject: "paged", Status: "waiting_support"}
		m := TicketMessage{Sender: "user", Author: "pg", Content: "c", CreatedAt: time.Now().Unix()}
		require.NoError(t, tk.InsertWithMessage(&m))
	}
	tickets, total, err := GetUserTicketsPaged(uid, 0, 2, "", "")
	require.NoError(t, err)
	assert.Equal(t, 2, len(tickets))
	assert.Equal(t, int64(5), total)
}

func TestGetUserTicketsPagedKeyword(t *testing.T) {
	uid := uniqueUserId()
	keyword := fmt.Sprintf("unique-kw-%d", uid)
	tk := Ticket{UserId: uid, Username: "kw", Subject: keyword, Status: "waiting_support"}
	m := TicketMessage{Sender: "user", Author: "kw", Content: "c", CreatedAt: time.Now().Unix()}
	require.NoError(t, tk.InsertWithMessage(&m))

	tickets, total, _ := GetUserTicketsPaged(uid, 0, 10, "", keyword)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, 1, len(tickets))

	_, total2, _ := GetUserTicketsPaged(uid, 0, 10, "", "nonexist-never-match")
	assert.Equal(t, int64(0), total2)
}
