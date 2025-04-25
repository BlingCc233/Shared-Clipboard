// main.go
package main

import (
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
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
var jwtKey = []byte("yoursecretkey")

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
	var result []string
	var currentWord strings.Builder
	var currentType rune // 0: 未知, 1: 中文, 2: 英文, 3: 数字, 4: 其他

	for _, char := range text {
		var charType rune

		switch {
		case unicode.Is(unicode.Han, char):
			charType = 1 // 中文
		case unicode.IsLetter(char):
			charType = 2 // 英文
		case unicode.IsDigit(char):
			charType = 3 // 数字
		default:
			charType = 4 // 其他
		}

		// 如果类型改变或者是中文字符，则当前单词结束
		if currentType != 0 && (currentType != charType || charType == 1) {
			// 处理中文词组：如果当前是中文词组且长度大于1
			if currentType == 1 && currentWord.Len() > 0 {
				chinesePhrase := currentWord.String()

				// 简单的二三字词检测
				if len(chinesePhrase) >= 4 {
					// 每2个字符尝试作为一个词组
					runes := []rune(chinesePhrase)
					for i := 0; i < len(runes); i += 2 {
						if i+1 < len(runes) {
							result = append(result, string(runes[i:i+2]))
						} else {
							result = append(result, string(runes[i:]))
						}
					}
				} else {
					result = append(result, chinesePhrase)
				}
			} else {
				// 非中文直接添加
				if currentWord.Len() > 0 {
					result = append(result, currentWord.String())
				}
			}

			currentWord.Reset()
		}

		currentWord.WriteRune(char)
		currentType = charType

		// 针对中文单字，立即添加到结果
		if charType == 1 && currentType == 1 && unicode.Is(unicode.Han, char) {
			// 不再立即添加单个汉字，而是等待积累
		}
	}

	// 处理最后一个单词
	if currentWord.Len() > 0 {
		if currentType == 1 {
			// 中文词组处理
			chinesePhrase := currentWord.String()
			if len(chinesePhrase) >= 4 {
				runes := []rune(chinesePhrase)
				for i := 0; i < len(runes); i += 2 {
					if i+1 < len(runes) {
						result = append(result, string(runes[i:i+2]))
					} else {
						result = append(result, string(runes[i:]))
					}
				}
			} else {
				result = append(result, chinesePhrase)
			}
		} else {
			result = append(result, currentWord.String())
		}
	}

	// 过滤空结果
	var filteredResult []string
	for _, word := range result {
		if strings.TrimSpace(word) != "" {
			filteredResult = append(filteredResult, word)
		}
	}

	return filteredResult
}
