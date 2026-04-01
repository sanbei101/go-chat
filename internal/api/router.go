package api

import (
	"github.com/gin-gonic/gin"

	"github.com/sanbei101/im/internal/api/handler"
)

func SetupRouter(userHandler *handler.UserHandler) *gin.Engine {
	r := gin.Default()

	v1 := r.Group("/api/v1")
	{
		users := v1.Group("/users")
		{
			users.POST("/register", userHandler.Register)
			users.POST("/login", userHandler.Login)
			users.POST("/batch", userHandler.BatchGenerate)
		}
	}

	return r
}
