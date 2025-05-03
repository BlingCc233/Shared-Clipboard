// main.go
package main

import (
	"crypto/sha256"
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
	"time"
)

// 模型定义
type ClipboardItem struct {
	gorm.Model
	Content    string `json:"content"`
	DeviceInfo string `json:"deviceInfo"`
	Type       string `json:"type"`                // text 或 image
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

var wordSplitRegex = regexp.MustCompile(`[a-zA-Z]+|[0-9]+|\p{Han}+|\p{Hiragana}+|\p{Katakana}+`)

func main() {
	// 初始化数据库
	var err error
	db, err = gorm.Open(sqlite.Open("clipboard.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect database:", err)
	}

	// 自动迁移
	db.AutoMigrate(&ClipboardItem{})

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
	router.POST("/auth", func(c *gin.Context) {
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
	})

	// 受保护的路由
	api := router.Group("/api")
	api.Use(authMiddleware())
	{
		// 获取所有剪贴板项目
		api.GET("/clipboard", func(c *gin.Context) {
			var items []ClipboardItem
			oldParam := c.Query("old")

			var oldID uint64
			if oldParam == "" {
				// 如果 old 参数为空，获取数据库中最新的 id
				var latestItem ClipboardItem
				if err := db.Order("id DESC").First(&latestItem).Error; err != nil {
					c.JSON(http.StatusNotFound, gin.H{"error": "No clipboard items found"})
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
				response = append(response, itemMap)
			}

			c.JSON(http.StatusOK, response)
		})

		api.POST("/split-words", func(c *gin.Context) {
			var req struct {
				Text string `json:"text"`
			}

			if err := c.BindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
				return
			}

			words := splitWords(req.Text)
			c.JSON(http.StatusBadRequest, gin.H{"words": words})
		})

		// 添加新的剪贴板项目
		api.POST("/clipboard", func(c *gin.Context) {
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

			item := ClipboardItem{
				Content:    req.Content,
				DeviceInfo: req.DeviceInfo,
				Type:       req.Type,
			}

			if req.Type == "image" {
				item.ImageData = req.ImageData
			}

			db.Create(&item)
			c.JSON(http.StatusCreated, item)
		})

		api.GET("/clipboard/latest", func(c *gin.Context) {
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
				response = append(response, itemMap)
			}

			c.JSON(http.StatusOK, response)
		})

		api.GET("/is_exist", handleIsExist)
	}

	router.Run("0.0.0.0:5859")
}

// 身份验证中间件
func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "No authorization header provided"})
			return
		}

		tokenString := authHeader[len("Bearer "):]

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

func splitWords(text string) []string {
	matches := wordSplitRegex.FindAllString(text, -1) // -1 表示查找所有匹配项
	if matches == nil {
		return []string{} // 如果没有匹配项，返回空切片
	}
	return matches
}

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

	var item ClipboardItem
	found := false

	// 使用 Rows 迭代以优化内存，避免一次性加载所有数据
	rows, err := db.Model(&ClipboardItem{}).Rows()
	if err != nil {
		log.Printf("获取数据库行进行哈希检查时出错: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库错误"})
		return
	}
	defer rows.Close() // 确保在函数结束时关闭 rows

	for rows.Next() {
		// 逐行扫描数据
		if err := db.ScanRows(rows, &item); err != nil {
			log.Printf("扫描数据库行进行哈希检查时出错: %v", err)
			continue // 跳过有问题的行
		}

		var dataToHash []byte
		if item.Type == "image" {
			dataToHash = item.ImageData
		} else {
			dataToHash = []byte(item.Content)
		}

		// 计算哈希
		hasher := sha256.New()
		hasher.Write(dataToHash)
		calculatedHashBytes := hasher.Sum(nil)
		calculatedHashHex := hex.EncodeToString(calculatedHashBytes)

		// 比较哈希
		if calculatedHashHex == targetHash {
			found = true
			break // 找到即退出循环
		}
	}

	// 检查迭代过程中是否发生错误
	if err := rows.Err(); err != nil {
		log.Printf("迭代数据库行进行哈希检查时出错: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "数据库迭代错误"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"exists": found})
}
