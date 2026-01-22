package main

import (
	"crypto/sha256"
	"errors"
	"encoding/hex"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// 模型定义
type ClipboardItem struct {
	gorm.Model
	Content    string `json:"content"`
	DeviceInfo string `json:"deviceInfo"`
	Type       string `json:"type"`                // text 或 image
	Hash       string `json:"hash" gorm:"index"`   // sha256 hex of normalized content/image bytes, used for dedup
	ImageData  []byte `json:"imageData,omitempty"` // 仅当 Type 为 image 时使用
}

// JWT Claims
type Claims struct {
	DeviceInfo string `json:"deviceInfo"`
	jwt.StandardClaims
}

var db *gorm.DB
var jwtKey = []byte("secret_key")

const PASSWORD = "yourpassword"

func main() {
	// 初始化数据库
	var err error
	db, err = gorm.Open(sqlite.Open("clipboard.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect database:", err)
	}

	// 自动迁移
	db.AutoMigrate(&ClipboardItem{})
	ensureClipboardItemHashes()

	// 初始化路由
	router := setupRouter()

	// 启动服务
	router.Run("0.0.0.0:5859")
}

func normalizeTextForHash(text string) string {
	// Normalize common cross-platform clipboard newline differences (CRLF/CR -> LF).
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}

func computeClipboardHash(itemType string, content string, imageData []byte) (string, error) {
	var data []byte
	switch itemType {
	case "image":
		if len(imageData) == 0 {
			return "", errors.New("missing imageData")
		}
		data = imageData
	case "text":
		data = []byte(normalizeTextForHash(content))
	default:
		return "", errors.New("invalid type")
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func clipboardItemToResponse(item ClipboardItem) map[string]interface{} {
	itemMap := map[string]interface{}{
		"id":         item.ID,
		"content":    item.Content,
		"deviceInfo": item.DeviceInfo,
		"type":       item.Type,
		"createdAt":  item.CreatedAt,
	}
	if item.Type == "image" {
		itemMap["imageData"] = item.ImageData
	}
	return itemMap
}

func ensureClipboardItemHashes() {
	// Best-effort backfill so /api/is_exist and insert-dedup can use fast indexed lookups.
	rows, err := db.Model(&ClipboardItem{}).
		Select("id", "type", "content", "image_data", "hash").
		Where("hash = '' OR hash IS NULL").
		Rows()
	if err != nil {
		log.Printf("Failed to iterate clipboard items for hash backfill: %v", err)
		return
	}
	defer rows.Close()

	var item ClipboardItem
	for rows.Next() {
		if err := db.ScanRows(rows, &item); err != nil {
			log.Printf("Failed to scan clipboard item for hash backfill: %v", err)
			continue
		}

		calculatedHash, err := computeClipboardHash(item.Type, item.Content, item.ImageData)
		if err != nil {
			log.Printf("Failed to compute hash for item %d: %v", item.ID, err)
			continue
		}

		if err := db.Model(&ClipboardItem{}).Where("id = ?", item.ID).Update("hash", calculatedHash).Error; err != nil {
			log.Printf("Failed to update hash for item %d: %v", item.ID, err)
		}
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error iterating clipboard items for hash backfill: %v", err)
	}
}

// 初始化路由
func setupRouter() *gin.Engine {
	router := gin.Default()

	// 配置CORS
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 身份验证
	router.POST("/auth", handleAuth)

	// 受保护的路由
	api := router.Group("/api")
	api.Use(authMiddleware())
	{
		api.GET("/clipboard", handleGetClipboard)
		api.GET("/clipboard/search", handleSearchClipboard)
		api.POST("/split-words", handleSplitWords)
		api.POST("/clipboard", handlePostClipboard)
		api.GET("/clipboard/latest", handleGetLatestClipboard)
		api.GET("/is_exist", handleIsExist)
	}

	return router
}

// 身份验证处理函数
func handleAuth(c *gin.Context) {
	var req struct {
		Password   string `json:"password"`
		DeviceInfo string `json:"deviceInfo"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// 检查密码
	if req.Password != PASSWORD {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
		return
	}

	// 创建一个有效期为1天的token
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		DeviceInfo: req.DeviceInfo,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

// 获取剪贴板项目处理函数
func handleGetClipboard(c *gin.Context) {
	var items []ClipboardItem
	oldParam := c.Query("old")

	var oldID uint64
	if oldParam == "" {
		// 如果 old 参数为空，获取数据库中最新的 id
		var latestItem ClipboardItem
		if err := db.Order("id DESC").First(&latestItem).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusOK, []map[string]interface{}{})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query clipboard items"})
			return
		}
		oldID = uint64(latestItem.ID) + 1
	} else {
		// 将 old 参数转换为 uint
		var err error
		oldID, err = strconv.ParseUint(oldParam, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid old parameter"})
			return
		}
	}

	// 查询小于 oldID 的前 10 条数据
	db.Where("id < ?", oldID).Order("id DESC").Limit(10).Find(&items)

	// 构造响应
	var response []map[string]interface{}
	for _, item := range items {
		response = append(response, clipboardItemToResponse(item))
	}

	c.JSON(http.StatusOK, response)
}

// 搜索剪贴板项目处理函数
func handleSearchClipboard(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing q parameter"})
		return
	}

	limit := 50
	if limitParam := strings.TrimSpace(c.Query("limit")); limitParam != "" {
		parsedLimit, err := strconv.Atoi(limitParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid limit parameter"})
			return
		}
		if parsedLimit < 1 {
			parsedLimit = 1
		}
		if parsedLimit > 200 {
			parsedLimit = 200
		}
		limit = parsedLimit
	}

	likeQuery := "%" + query + "%"
	var items []ClipboardItem
	if err := db.Where("content LIKE ? OR device_info LIKE ?", likeQuery, likeQuery).
		Order("id DESC").
		Limit(limit).
		Find(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search clipboard items"})
		return
	}

	var response []map[string]interface{}
	for _, item := range items {
		response = append(response, clipboardItemToResponse(item))
	}

	c.JSON(http.StatusOK, response)
}

// 分词处理函数
func handleSplitWords(c *gin.Context) {
	var req struct {
		Text string `json:"text"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	words := splitWords(req.Text)
	c.JSON(http.StatusOK, gin.H{"words": words})
}

// 添加剪贴板项目处理函数
func handlePostClipboard(c *gin.Context) {
	var req struct {
		Content    string `json:"content"`
		DeviceInfo string `json:"deviceInfo"`
		Type       string `json:"type"`
		ImageData  []byte `json:"imageData,omitempty"`
	}

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	calculatedHash, err := computeClipboardHash(req.Type, req.Content, req.ImageData)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid clipboard payload"})
		return
	}

	// Server-side dedup: prevents duplicates even under polling overlap / network retries.
	var existing ClipboardItem
	if err := db.Where("hash = ?", calculatedHash).Order("id DESC").First(&existing).Error; err == nil {
		c.JSON(http.StatusOK, clipboardItemToResponse(existing))
		return
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	item := ClipboardItem{
		Content:    req.Content,
		DeviceInfo: req.DeviceInfo,
		Type:       req.Type,
		Hash:       calculatedHash,
	}

	if req.Type == "image" {
		item.ImageData = req.ImageData
	}

	db.Create(&item)
	c.JSON(http.StatusCreated, clipboardItemToResponse(item))
}

// 获取最新剪贴板项目处理函数
func handleGetLatestClipboard(c *gin.Context) {
	var items []ClipboardItem
	newParam := c.Query("new")

	if newParam == "" {
		// 如果 new 参数为空，返回 400 错误码
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing new parameter"})
		return
	}

	// 将 new 参数转换为 uint
	newID, err := strconv.ParseUint(newParam, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid new parameter"})
		return
	}

	// 查询大于 newID 的所有数据
	db.Where("id > ?", newID).Order("id ASC").Find(&items)

	// 构造响应
	var response []map[string]interface{}
	for _, item := range items {
		response = append(response, clipboardItemToResponse(item))
	}

	c.JSON(http.StatusOK, response)
}

// 身份验证中间件
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "No authorization header provided"})
			return
		}

		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(authHeader, bearerPrefix) || len(authHeader) <= len(bearerPrefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header"})
			return
		}
		tokenString := authHeader[len(bearerPrefix):]

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		c.Set("deviceInfo", claims.DeviceInfo)
		c.Next()
	}
}

// 分词函数
func splitWords(text string) []string {
	var wordSplitRegex = regexp.MustCompile(`[a-zA-Z]+|[0-9]+|\p{Han}+|\p{Hiragana}+|\p{Katakana}+`)
	matches := wordSplitRegex.FindAllString(text, -1) // -1 表示查找所有匹配项
	if matches == nil {
		return []string{} // 如果没有匹配项，返回空切片
	}
	return matches
}

// 检查是否存在处理函数
func handleIsExist(c *gin.Context) {
	targetHash := c.Query("sha256")
	if targetHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 sha256 查询参数"})
		return
	}
	if len(targetHash) != 64 { // SHA256 哈希应为 64 个十六进制字符
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 sha256 哈希格式"})
		return
	}

	var count int64
	if err := db.Model(&ClipboardItem{}).Where("hash = ?", targetHash).Count(&count).Error; err != nil {
		log.Printf("查询数据库进行哈希检查时出错: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
		return
	}
	if count > 0 {
		c.JSON(http.StatusOK, gin.H{"exists": true})
		return
	}

	// Fallback: backfill and check rows with missing hash (older DBs).
	rows, err := db.Model(&ClipboardItem{}).
		Select("id", "type", "content", "image_data", "hash").
		Where("hash = '' OR hash IS NULL").
		Rows()
	if err != nil {
		log.Printf("获取数据库行进行哈希检查时出错: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
		return
	}
	defer rows.Close() // 确保在函数结束时关闭 rows

	var item ClipboardItem
	for rows.Next() {
		if err := db.ScanRows(rows, &item); err != nil {
			log.Printf("扫描数据库行进行哈希检查时出错: %v", err)
			continue // 跳过有问题的行
		}

		calculatedHashHex, err := computeClipboardHash(item.Type, item.Content, item.ImageData)
		if err != nil {
			continue
		}

		// Backfill for future fast checks.
		if err := db.Model(&ClipboardItem{}).Where("id = ?", item.ID).Update("hash", calculatedHashHex).Error; err != nil {
			log.Printf("更新剪贴板哈希时出错 (id=%d): %v", item.ID, err)
		}

		// 比较哈希
		if calculatedHashHex == targetHash {
			c.JSON(http.StatusOK, gin.H{"exists": true})
			return
		}
	}

	// 检查迭代过程中是否发生错误
	if err := rows.Err(); err != nil {
		log.Printf("迭代数据库行进行哈希检查时出错: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库迭代错误"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"exists": false})
}
