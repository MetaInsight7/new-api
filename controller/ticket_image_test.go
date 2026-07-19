package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupImageTestDB(t *testing.T) (cleanup func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	origSQLite := common.UsingSQLite
	origRedis := common.RedisEnabled
	origDB := model.DB

	common.UsingSQLite = true
	common.RedisEnabled = false
	model.DB = db

	require.NoError(t, db.AutoMigrate(&model.Ticket{}, &model.TicketMessage{}, &model.TicketImage{}))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	return func() {
		model.DB = origDB
		common.UsingSQLite = origSQLite
		common.RedisEnabled = origRedis
		_ = sqlDB.Close()
	}
}

type imageTestSetup struct {
	ticket  model.Ticket
	message model.TicketMessage
}

func createImageTestData(t *testing.T, userId int) imageTestSetup {
	t.Helper()
	now := time.Now().Unix()
	ticket := model.Ticket{UserId: userId, Username: "tester", Subject: "img test", Status: "waiting_support", CreatedAt: now, UpdatedAt: now}
	msg := model.TicketMessage{Sender: "user", Author: "tester", Content: "test msg", CreatedAt: now}
	require.NoError(t, ticket.InsertWithMessage(&msg))
	return imageTestSetup{ticket: ticket, message: msg}
}

func buildMultipartRequest(t *testing.T, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return body, writer.FormDataContentType()
}

func makeUploadRequest(t *testing.T, ticketId, msgId, userId, role int, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{
		{Key: "id", Value: fmt.Sprintf("%d", ticketId)},
		{Key: "msgId", Value: fmt.Sprintf("%d", msgId)},
	}
	c.Request = httptest.NewRequest(http.MethodPost, "/", body)
	c.Request.Header.Set("Content-Type", contentType)
	c.Set("id", userId)
	c.Set("role", role)
	c.Set("username", "tester")
	UploadMessageImage(c)
	return w
}

type apiResp struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func TestUploadImage_FakeMIME_Rejected(t *testing.T) {
	cleanup := setupImageTestDB(t)
	defer cleanup()

	origEnabled := r2Enabled
	origUpload := r2Upload
	origDelete := r2Delete
	defer func() { r2Enabled = origEnabled; r2Upload = origUpload; r2Delete = origDelete }()

	r2Enabled = func() bool { return true }
	uploadCalled := false
	r2Upload = func(ctx context.Context, key string, data io.Reader, ct string) error {
		uploadCalled = true
		return nil
	}
	r2Delete = func(ctx context.Context, key string) error { return nil }

	userId := 500
	setup := createImageTestData(t, userId)

	// Create a text file with .jpg extension — real MIME will be text/plain
	fakeContent := []byte("this is not a JPEG, it is plain text pretending to be an image")
	body, ct := buildMultipartRequest(t, "fake.jpg", fakeContent)

	w := makeUploadRequest(t, setup.ticket.Id, setup.message.Id, userId, common.RoleCommonUser, body, ct)

	var resp apiResp
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "JPG")
	assert.False(t, uploadCalled, "R2 upload should NOT be called for invalid MIME")

	// Verify no DB record
	var count int64
	require.NoError(t, model.DB.Model(&model.TicketImage{}).Where("ticket_id = ?", setup.ticket.Id).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestUploadImage_TooLarge_Rejected(t *testing.T) {
	cleanup := setupImageTestDB(t)
	defer cleanup()

	origEnabled := r2Enabled
	origUpload := r2Upload
	origDelete := r2Delete
	defer func() { r2Enabled = origEnabled; r2Upload = origUpload; r2Delete = origDelete }()

	r2Enabled = func() bool { return true }
	uploadCalled := false
	r2Upload = func(ctx context.Context, key string, data io.Reader, ct string) error {
		uploadCalled = true
		return nil
	}
	r2Delete = func(ctx context.Context, key string) error { return nil }

	userId := 501
	setup := createImageTestData(t, userId)

	// JPEG magic bytes + padding to exceed 5MB
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	bigContent := make([]byte, 5*1024*1024+100)
	copy(bigContent, jpegHeader)
	body, ct := buildMultipartRequest(t, "big.jpg", bigContent)

	w := makeUploadRequest(t, setup.ticket.Id, setup.message.Id, userId, common.RoleCommonUser, body, ct)

	var resp apiResp
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.False(t, uploadCalled, "R2 upload should NOT be called for oversized file")
}

func TestUploadImage_R2Fails_NoDB(t *testing.T) {
	cleanup := setupImageTestDB(t)
	defer cleanup()

	origEnabled := r2Enabled
	origUpload := r2Upload
	origDelete := r2Delete
	defer func() { r2Enabled = origEnabled; r2Upload = origUpload; r2Delete = origDelete }()

	r2Enabled = func() bool { return true }
	r2Upload = func(ctx context.Context, key string, data io.Reader, ct string) error {
		return errors.New("R2 network timeout")
	}
	r2Delete = func(ctx context.Context, key string) error { return nil }

	userId := 502
	setup := createImageTestData(t, userId)

	// Valid JPEG content (minimal JPEG magic)
	jpegContent := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
	body, ct := buildMultipartRequest(t, "valid.jpg", jpegContent)

	w := makeUploadRequest(t, setup.ticket.Id, setup.message.Id, userId, common.RoleCommonUser, body, ct)

	var resp apiResp
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Contains(t, resp.Message, "上传失败")

	// No DB record
	var count int64
	require.NoError(t, model.DB.Model(&model.TicketImage{}).Where("ticket_id = ?", setup.ticket.Id).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestUploadImage_R2SuccessDBFails_DeleteCalled(t *testing.T) {
	cleanup := setupImageTestDB(t)
	defer cleanup()

	origEnabled := r2Enabled
	origUpload := r2Upload
	origDelete := r2Delete
	defer func() { r2Enabled = origEnabled; r2Upload = origUpload; r2Delete = origDelete }()

	r2Enabled = func() bool { return true }
	r2Upload = func(ctx context.Context, key string, data io.Reader, ct string) error {
		return nil // R2 succeeds
	}
	deleteCalled := false
	r2Delete = func(ctx context.Context, key string) error {
		deleteCalled = true
		return nil
	}

	userId := 503
	setup := createImageTestData(t, userId)

	// Fill all 3 slots so DB transaction will reject with SlotFull
	for i := 1; i <= 3; i++ {
		img := model.TicketImage{TicketId: setup.ticket.Id, MessageId: setup.message.Id, Slot: i, UserId: userId, Filename: "x.jpg", StorageKey: fmt.Sprintf("pre_%d", i), ContentType: "image/jpeg", Size: 100, CreatedAt: time.Now().Unix()}
		require.NoError(t, model.DB.Create(&img).Error)
	}

	// Valid JPEG content — pre-check passes (message valid, within 15 min, right sender)
	jpegContent := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46}
	body, ct := buildMultipartRequest(t, "valid.jpg", jpegContent)

	w := makeUploadRequest(t, setup.ticket.Id, setup.message.Id, userId, common.RoleCommonUser, body, ct)

	var resp apiResp
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.True(t, deleteCalled, "R2 Delete must be called when DB rejects after successful upload")

	// Still only 3 records (the pre-existing ones)
	var count int64
	require.NoError(t, model.DB.Model(&model.TicketImage{}).Where("ticket_id = ?", setup.ticket.Id).Count(&count).Error)
	assert.Equal(t, int64(3), count)
}
