package router

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sanbei101/go-chat/internal/user"
	"github.com/sanbei101/go-chat/internal/ws"
	"github.com/sanbei101/go-chat/util"
)

// GIN Router to create API endpoints

var r *gin.Engine

func Init(userHandler *user.Handler, wsHandler *ws.Handler) *gin.Engine {
	r = gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE"},
	}))

	r.POST("/signup", userHandler.CreateUser)
	r.POST("/login", userHandler.Login)
	r.GET("/logout", userHandler.Logout)

	// Protected routes
	protected := r.Group("/ws")
	protected.Use(util.JWTValidateToken())
	{
		protected.GET("/auth", userHandler.AuthUser)
		protected.POST("/createRoom", wsHandler.CreateRoom)
		protected.GET("/getRooms", wsHandler.GetRooms)
		protected.GET("/getClients/:roomId", wsHandler.GetClients)
	}

	// Moved joinRoom out of protected group
	// for WebSocket issues in deployment
	r.GET("/joinRoom/:roomId", wsHandler.JoinRoom)

	return r
}
