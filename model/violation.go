package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// ViolationLog 违规审计记录(落 LOG_DB,与 Log 表同库不同表)。
// 记录每次命中违规词库的请求:谁、何时、什么模型、命中了哪些词/分类/分级、
// 系统做了什么(拦截/放行)、以及命中处的截断片段(不存整段 prompt)。
type ViolationLog struct {
	Id           int    `json:"id"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint;index:idx_violation_created_at"`
	UserId       int    `json:"user_id" gorm:"index"`
	Username     string `json:"username" gorm:"index;default:''"`
	TokenId      int    `json:"token_id" gorm:"default:0"`
	TokenName    string `json:"token_name" gorm:"default:''"`
	UserGroup    string `json:"user_group" gorm:"default:''"`
	ModelName    string `json:"model_name" gorm:"index;default:''"`
	ChannelId    int    `json:"channel_id" gorm:"default:0"`
	Ip           string `json:"ip" gorm:"default:''"`
	RequestId    string `json:"request_id" gorm:"type:varchar(64);index:idx_violation_request_id;default:''"`
	Category     string `json:"category" gorm:"index;default:''"` // 命中分类(逗号分隔,可多)
	Severity     string `json:"severity" gorm:"index;default:''"` // 命中最高分级 high|low
	Action       string `json:"action" gorm:"index;default:''"`   // blocked|allowed
	MatchedWords string `json:"matched_words" gorm:"type:text"`   // 命中词(逗号分隔)
	Snippet      string `json:"snippet" gorm:"type:text"`         // 命中处上下文截断片段
	Context      string `json:"context" gorm:"type:text"`         // 当前及前若干轮结构化对话 JSON([{role,text}])
	Other        string `json:"other" gorm:"type:text"`           // 预留 JSON 扩展
}

const (
	ViolationActionBlocked = "blocked"
	ViolationActionAllowed = "allowed"
)

// CreateViolationLog 直接写入一条违规记录(调用方负责异步/错误处理)。
func CreateViolationLog(log *ViolationLog) error {
	return LOG_DB.Create(log).Error
}

// GetViolationLogs 违规记录分页查询(仅管理端使用)。
func GetViolationLogs(startTimestamp, endTimestamp int64, username, modelName, category, severity, action, requestId string, startIdx, num int) (logs []*ViolationLog, total int64, err error) {
	tx := LOG_DB.Model(&ViolationLog{})
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	if category != "" {
		// 分类以逗号分隔存储,用 LIKE 命中包含关系
		tx = tx.Where("category LIKE ?", "%"+category+"%")
	}
	if severity != "" {
		tx = tx.Where("severity = ?", severity)
	}
	if action != "" {
		tx = tx.Where("action = ?", action)
	}
	if requestId != "" {
		tx = tx.Where("request_id = ?", requestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if err = tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = tx.Order("created_at desc, id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	return logs, total, err
}

// ViolationTrendPoint 违规趋势(按天)。
type ViolationTrendPoint struct {
	Day     string `json:"day"`
	Total   int    `json:"total"`
	Blocked int    `json:"blocked"`
	Allowed int    `json:"allowed"`
}

// ViolationNameCount 通用“名称->计数”(Top 用户 / 分类占比)。
type ViolationNameCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// ViolationStat 监控聚合结果。
type ViolationStat struct {
	Total      int64                 `json:"total"`
	Blocked    int64                 `json:"blocked"`
	Allowed    int64                 `json:"allowed"`
	Trend      []ViolationTrendPoint `json:"trend"`      // 按天
	TopUsers   []ViolationNameCount  `json:"top_users"`  // Top 违规用户
	TopWords   []ViolationNameCount  `json:"top_words"`  // Top 命中词
	Categories []ViolationNameCount  `json:"categories"` // 分类占比
}

// GetViolationStat 监控聚合。
// 趋势用 Go 侧按天分桶(避开三库日期函数差异);Top 用户/分类走 GROUP BY 索引列;
// Top 词在内存对 matched_words 拆分统计。窗口由调用方(时间范围)约束,防拉全表。
func GetViolationStat(startTimestamp, endTimestamp int64) (*ViolationStat, error) {
	stat := &ViolationStat{
		Trend:      []ViolationTrendPoint{},
		TopUsers:   []ViolationNameCount{},
		TopWords:   []ViolationNameCount{},
		Categories: []ViolationNameCount{},
	}
	base := LOG_DB.Model(&ViolationLog{})
	if startTimestamp != 0 {
		base = base.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		base = base.Where("created_at <= ?", endTimestamp)
	}

	// 总数 / 拦截 / 放行
	if err := base.Session(&gorm.Session{}).Count(&stat.Total).Error; err != nil {
		return nil, err
	}
	base.Session(&gorm.Session{}).Where("action = ?", ViolationActionBlocked).Count(&stat.Blocked)
	base.Session(&gorm.Session{}).Where("action = ?", ViolationActionAllowed).Count(&stat.Allowed)

	// Top 违规用户(GROUP BY username,取前 10)
	var users []ViolationNameCount
	base.Session(&gorm.Session{}).
		Select("username as name, COUNT(*) as count").
		Where("username <> ''").
		Group("username").Order("count desc").Limit(10).Scan(&users)
	stat.TopUsers = users

	// 拉取窗口内记录用于趋势分桶 + Top 词统计(限量防内存爆)
	type row struct {
		CreatedAt    int64
		Action       string
		Category     string
		MatchedWords string
	}
	var rows []row
	base.Session(&gorm.Session{}).
		Select("created_at, action, category, matched_words").
		Order("created_at desc").Limit(20000).Scan(&rows)

	dayIdx := map[string]int{}
	wordCount := map[string]int{}
	catCount := map[string]int{}
	for _, r := range rows {
		day := time.Unix(r.CreatedAt, 0).Format("2006-01-02")
		if i, ok := dayIdx[day]; ok {
			stat.Trend[i].Total++
			if r.Action == ViolationActionBlocked {
				stat.Trend[i].Blocked++
			} else {
				stat.Trend[i].Allowed++
			}
		} else {
			p := ViolationTrendPoint{Day: day, Total: 1}
			if r.Action == ViolationActionBlocked {
				p.Blocked = 1
			} else {
				p.Allowed = 1
			}
			dayIdx[day] = len(stat.Trend)
			stat.Trend = append(stat.Trend, p)
		}
		for _, w := range strings.Split(r.MatchedWords, ",") {
			if w = strings.TrimSpace(w); w != "" {
				wordCount[w]++
			}
		}
		for _, c := range strings.Split(r.Category, ",") {
			if c = strings.TrimSpace(c); c != "" {
				catCount[c]++
			}
		}
	}
	// Trend 按天升序
	sortTrendAsc(stat.Trend)
	stat.TopWords = topN(wordCount, 15)
	stat.Categories = topN(catCount, 10)
	return stat, nil
}

func sortTrendAsc(t []ViolationTrendPoint) {
	for i := 1; i < len(t); i++ {
		for j := i; j > 0 && t[j-1].Day > t[j].Day; j-- {
			t[j-1], t[j] = t[j], t[j-1]
		}
	}
}

func topN(m map[string]int, n int) []ViolationNameCount {
	out := make([]ViolationNameCount, 0, len(m))
	for k, v := range m {
		out = append(out, ViolationNameCount{Name: k, Count: v})
	}
	// 简单选择排序取前 n(数据量小)
	for i := 0; i < len(out); i++ {
		max := i
		for j := i + 1; j < len(out); j++ {
			if out[j].Count > out[max].Count {
				max = j
			}
		}
		out[i], out[max] = out[max], out[i]
		if i+1 >= n {
			break
		}
	}
	if len(out) > n {
		out = out[:n]
	}
	return out
}
