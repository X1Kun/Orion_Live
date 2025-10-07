package middleware

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// 中间件工厂，改成AuthMiddleware(role string)，就能创建一个只允许特定角色的用户通过的中间件
// 流程：1、从http请求中取出"Authorization"字段 2、验证"Bearer [token]" 3、通过secretKey验证token有效性 4、若成功，从token中取出后续用到的用户信息，放入context
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 拿到http协议请求头中的Authorization字段
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// 立刻调用c.Abort()，阻止后续的任何处理器（包括其他中间件和最终的handler）被执行
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "请求未包含授权令牌"})
			return
		}

		// 通常Token的格式是 "Bearer [token]"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "授权令牌格式不正确"})
			return
		}

		tokenString := parts[1]
		secretKey := []byte(os.Getenv("JWT_SECRET_KEY"))

		// 解析Token，返回加密前的token（Header.Payload.Signature），还附带valid判断是否有效
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			// 确保签名方法是对称加密族
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("非预期的签名方法")
			}
			return secretKey, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "无效的授权令牌"})
			return
		}

		// Token验证成功！将用户信息存入Context，以便后续使用
		claims, ok := token.Claims.(jwt.MapClaims)
		if ok {
			c.Set("userID", claims["user_id"])
			c.Set("username", claims["username"])
		}

		// 放行，继续处理请求
		c.Next()
	}
}
