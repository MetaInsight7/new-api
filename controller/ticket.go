package controller

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

var validDepartments = map[string]bool{"technical": true, "billing": true, "account": true, "other": true}
var validPriorities = map[string]bool{"high": true, "medium": true, "low": true}
var allowedImageTypes = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true}
var mimeToExt = map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}

// Dependency seams for testing. Override in tests; restore after.
var r2Enabled = func() bool { return service.IsR2Enabled() }
var r2Upload = func(ctx context.Context, key string, data io.Reader, ct string) error { return service.R2Upload(ctx, key, data, ct) }
var r2Delete = func(ctx context.Context, key string) error { return service.R2Delete(ctx, key) }

const maxImageSize = 5 * 1024 * 1024

type createTicketRequest struct {
	Subject    string `json:"subject"`
	Department string `json:"department"`
	Priority   string `json:"priority"`
	Content    string `json:"content"`
}

type replyRequest struct {
	Content string `json:"content"`
}

type statusRequest struct {
	Status string `json:"status"`
}

// --- User endpoints ---

func GetUserTickets(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	status := c.Query("status")
	keyword := c.Query("keyword")
	tickets, total, err := model.GetUserTicketsPaged(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), status, keyword)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tickets)
	common.ApiSuccess(c, pageInfo)
}

func GetUserTicket(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的工单 ID")
		return
	}
	ticket, err := model.GetTicketById(id)
	if err != nil {
		common.ApiErrorMsg(c, "工单不存在")
		return
	}
	userId := c.GetInt("id")
	if ticket.UserId != userId {
		common.ApiErrorMsg(c, "无权访问此工单")
		return
	}
	if ticket.Unread {
		_ = ticket.SetUnread(false)
		ticket.Unread = false
	}
	common.ApiSuccess(c, ticket)
}

func CreateTicket(c *gin.Context) {
	var req createTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "请求参数错误")
		return
	}
	if utf8.RuneCountInString(req.Subject) == 0 || utf8.RuneCountInString(req.Subject) > 120 {
		common.ApiErrorMsg(c, "工单主题长度应在 1-120 字之间")
		return
	}
	if utf8.RuneCountInString(req.Content) == 0 || utf8.RuneCountInString(req.Content) > 5000 {
		common.ApiErrorMsg(c, "问题描述长度应在 1-5000 字之间")
		return
	}
	if !validDepartments[req.Department] {
		req.Department = "technical"
	}
	if !validPriorities[req.Priority] {
		req.Priority = "medium"
	}

	userId := c.GetInt("id")
	openCount, _ := model.CountUserOpenTickets(userId)
	if openCount >= 5 {
		common.ApiErrorMsg(c, "未关闭工单已达上限（5 个），请先关闭已解决的工单")
		return
	}
	lastCreated, _ := model.GetUserLastTicketTime(userId)
	if time.Now().Unix()-lastCreated < 60 {
		common.ApiErrorMsg(c, "创建过于频繁，请 1 分钟后再试")
		return
	}

	now := time.Now().Unix()
	ticket := model.Ticket{
		UserId:     userId,
		Username:   c.GetString("username"),
		Subject:    req.Subject,
		Department: req.Department,
		Priority:   req.Priority,
		Status:     "waiting_support",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	msg := model.TicketMessage{
		Sender:    "user",
		Author:    c.GetString("username"),
		Content:   req.Content,
		CreatedAt: now,
	}
	if err := ticket.InsertWithMessage(&msg); err != nil {
		common.ApiError(c, err)
		return
	}
	ticket.Messages = []model.TicketMessage{msg}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": ticket})
}

func ReplyTicket(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的工单 ID")
		return
	}
	ticket, err := model.GetTicketById(id)
	if err != nil {
		common.ApiErrorMsg(c, "工单不存在")
		return
	}
	userId := c.GetInt("id")
	if ticket.UserId != userId {
		common.ApiErrorMsg(c, "无权操作此工单")
		return
	}
	if ticket.Status == "closed" || ticket.Status == "resolved" {
		common.ApiErrorMsg(c, "工单已关闭，无法回复")
		return
	}
	var req replyRequest
	if err := c.ShouldBindJSON(&req); err != nil || utf8.RuneCountInString(req.Content) == 0 {
		common.ApiErrorMsg(c, "回复内容不能为空")
		return
	}
	if utf8.RuneCountInString(req.Content) > 5000 {
		common.ApiErrorMsg(c, "回复内容不能超过 5000 字")
		return
	}
	recentCount, _ := model.CountRecentMessages(ticket.Id, 60)
	if recentCount >= 10 {
		common.ApiErrorMsg(c, "回复过于频繁，请稍后再试")
		return
	}

	now := time.Now().Unix()
	msg := model.TicketMessage{
		Sender:    "user",
		Author:    c.GetString("username"),
		Content:   req.Content,
		CreatedAt: now,
	}
	if err := model.InsertReply(&msg, ticket.Id, "waiting_support", false); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": msg})
}

func CloseTicket(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的工单 ID")
		return
	}
	ticket, err := model.GetTicketById(id)
	if err != nil {
		common.ApiErrorMsg(c, "工单不存在")
		return
	}
	userId := c.GetInt("id")
	if ticket.UserId != userId {
		common.ApiErrorMsg(c, "无权操作此工单")
		return
	}
	if ticket.Status == "closed" {
		common.ApiErrorMsg(c, "工单已关闭")
		return
	}
	now := time.Now().Unix()
	if err := model.DB.Model(ticket).Updates(map[string]interface{}{"status": "closed", "updated_at": now}).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

// --- Admin endpoints ---

func AdminGetAllTickets(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	status := c.Query("status")
	priority := c.Query("priority")
	keyword := c.Query("keyword")
	tickets, total, err := model.GetAllTickets(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), status, priority, keyword)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tickets)
	common.ApiSuccess(c, pageInfo)
}

func AdminGetTicket(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的工单 ID")
		return
	}
	ticket, err := model.GetTicketById(id)
	if err != nil {
		common.ApiErrorMsg(c, "工单不存在")
		return
	}
	common.ApiSuccess(c, ticket)
}

func AdminReplyTicket(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的工单 ID")
		return
	}
	ticket, err := model.GetTicketById(id)
	if err != nil {
		common.ApiErrorMsg(c, "工单不存在")
		return
	}
	if ticket.Status == "closed" || ticket.Status == "resolved" {
		common.ApiErrorMsg(c, "工单已关闭，无法回复")
		return
	}
	var req replyRequest
	if err := c.ShouldBindJSON(&req); err != nil || utf8.RuneCountInString(req.Content) == 0 {
		common.ApiErrorMsg(c, "回复内容不能为空")
		return
	}
	if utf8.RuneCountInString(req.Content) > 5000 {
		common.ApiErrorMsg(c, "回复内容不能超过 5000 字")
		return
	}

	now := time.Now().Unix()
	msg := model.TicketMessage{
		Sender:    "support",
		Author:    c.GetString("username"),
		Content:   req.Content,
		CreatedAt: now,
	}
	if err := model.InsertReply(&msg, ticket.Id, "waiting_user", true); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": msg})
}

func AdminUpdateTicketStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的工单 ID")
		return
	}
	if _, err := model.GetTicketById(id); err != nil {
		common.ApiErrorMsg(c, "工单不存在")
		return
	}
	var req statusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "请求参数错误")
		return
	}
	validStatuses := map[string]bool{"waiting_support": true, "waiting_user": true, "resolved": true, "closed": true}
	if !validStatuses[req.Status] {
		common.ApiErrorMsg(c, "无效的状态值")
		return
	}
	now := time.Now().Unix()
	if err := model.DB.Model(&model.Ticket{}).Where("id = ?", id).Updates(map[string]interface{}{"status": req.Status, "updated_at": now}).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

// --- Image upload ---

func UploadMessageImage(c *gin.Context) {
	if !r2Enabled() {
		common.ApiErrorMsg(c, "图片上传服务未配置")
		return
	}
	ticketId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的工单 ID")
		return
	}
	messageId, err := strconv.Atoi(c.Param("msgId"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的消息 ID")
		return
	}

	// Pre-check: verify permission before parsing body (prevent abuse)
	userId := c.GetInt("id")
	role := c.GetInt("role")
	isAdmin := role >= common.RoleAdminUser
	msg, err := model.GetMessageById(messageId)
	if err != nil || msg.TicketId != ticketId {
		common.ApiErrorMsg(c, "消息不存在或不属于此工单")
		return
	}
	if isAdmin {
		if msg.Sender != "support" {
			common.ApiErrorMsg(c, "管理员只能上传到自己的回复")
			return
		}
	} else {
		if msg.Sender != "user" {
			common.ApiErrorMsg(c, "只能上传到自己的消息")
			return
		}
		owner, err := model.GetTicketOwnerId(ticketId)
		if err != nil || owner != userId {
			common.ApiErrorMsg(c, "无权操作此工单")
			return
		}
	}
	if time.Now().Unix()-msg.CreatedAt > 15*60 {
		common.ApiErrorMsg(c, "上传窗口已过期（15 分钟），请通过新回复上传")
		return
	}

	// Limit request body size to prevent memory abuse
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImageSize+1024*10)

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		common.ApiErrorMsg(c, "请选择文件或文件过大")
		return
	}
	defer file.Close()

	if header.Size > maxImageSize {
		common.ApiErrorMsg(c, "图片大小不能超过 5MB")
		return
	}

	// Real MIME detection
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	contentType := http.DetectContentType(buf[:n])
	if _, err := file.Seek(0, 0); err != nil {
		common.ApiErrorMsg(c, "读取文件失败")
		return
	}
	if !allowedImageTypes[contentType] {
		common.ApiErrorMsg(c, "只支持 JPG、PNG、WebP 格式")
		return
	}

	// Determine extension from real MIME
	ext := mimeToExt[contentType]

	// Generate storage key
	randBytes := make([]byte, 8)
	if _, err := rand.Read(randBytes); err != nil {
		common.ApiErrorMsg(c, "上传失败")
		return
	}
	storageKey := fmt.Sprintf("tickets/%d/%d/%s%s", ticketId, messageId, hex.EncodeToString(randBytes), ext)

	// Upload to R2 first
	uploadCtx, uploadCancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer uploadCancel()
	if err := r2Upload(uploadCtx, storageKey, file, contentType); err != nil {
		common.ApiErrorMsg(c, "上传失败，请重试")
		return
	}

	// DB transaction: validate + insert with slot

	// Safe filename: use only MIME-derived extension
	safeFilename := "image" + ext

	img := model.TicketImage{
		Filename:    safeFilename,
		StorageKey:  storageKey,
		ContentType: contentType,
		Size:        header.Size,
		CreatedAt:   time.Now().Unix(),
	}

	slot, err := model.InsertImageWithSlot(&img, ticketId, messageId, userId, isAdmin)
	if err != nil {
		// DB failed — best-effort delete R2
		delCtx, delCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if delErr := r2Delete(delCtx, storageKey); delErr != nil {
			common.SysError(fmt.Sprintf("R2 orphan: key=%s, reason: %s", storageKey, delErr.Error()))
		}
		delCancel()
		if errors.Is(err, model.ErrTicketImageSlotFull) {
			common.ApiErrorMsg(c, "该消息已有 3 张图片")
			return
		}
		if errors.Is(err, model.ErrTicketImageUploadExpired) {
			common.ApiErrorMsg(c, "上传窗口已过期（15 分钟），请通过新回复上传")
			return
		}
		if errors.Is(err, model.ErrTicketImagePermission) {
			common.ApiErrorMsg(c, "无权上传到此消息")
			return
		}
		common.ApiErrorMsg(c, "上传失败")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{
		"id":       img.Id,
		"slot":     slot,
		"filename": img.Filename,
		"size":     img.Size,
	}})
}

func GetTicketImage(c *gin.Context) {
	imageId, err := strconv.Atoi(c.Param("image_id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	img, err := model.GetTicketImageById(imageId)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	userId := c.GetInt("id")
	role := c.GetInt("role")
	if role < common.RoleAdminUser {
		owner, err := model.GetTicketOwnerId(img.TicketId)
		if err != nil || owner != userId {
			c.Status(http.StatusForbidden)
			return
		}
	}

	getCtx, getCancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer getCancel()
	reader, ct, err := service.R2Get(getCtx, img.StorageKey)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	if ct == "" {
		ct = img.ContentType
	}
	c.Header("Content-Type", ct)
	c.Header("Cache-Control", "private, max-age=86400")
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", strings.ReplaceAll(img.Filename, "\"", "")))
	io.Copy(c.Writer, reader)
}
