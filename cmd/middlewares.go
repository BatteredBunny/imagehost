package cmd

import (
	"net/http"

	"github.com/didip/tollbooth/v8"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
)

func (app *Application) ratelimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		httpError := tollbooth.LimitByRequest(app.RateLimiter, c.Writer, c.Request)
		if httpError != nil {
			c.Data(httpError.StatusCode, app.RateLimiter.GetMessageContentType(), []byte(httpError.Message))
			c.Abort()
		} else {
			c.Next()
		}
	}
}

// limits request body size
func (app *Application) bodySizeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, app.config.MaxUploadSize)
		c.Next()
	}
}

// parses form
func (app *Application) apiMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := c.Request.ParseMultipartForm(app.config.MaxUploadSize); err != nil {
			c.String(http.StatusRequestEntityTooLarge, "Too big file")
			c.Abort()
			return
		}

		c.Next()
	}
}

type TokenVerification struct {
	Token string `form:"token"`
}

// Makes sure request has token and a valid one
func hasTokenMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var token TokenVerification
		var err error

		if err = c.MustBindWith(&token, binding.FormPost); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		if _, err = uuid.Parse(token.Token); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"invalid token": err.Error()})
			c.Abort()
			return
		}

		c.Set("token", token.Token)
		c.Next()
	}
}

func (app *Application) adminTokenVerificationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Verify the field exists
		rawToken, exists := c.Get("token")
		if !exists {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		// Verify its valid format
		token, err := uuid.Parse(rawToken.(string))
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		isAdmin, err := app.isAdmin(token)
		if err != nil { // Could be a database error
			app.logError.Println(err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		} else if !isAdmin { // Invalid token or not an admin account
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next()
	}
}
func (app *Application) userTokenVerificationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Verify the field exists
		rawToken, exists := c.Get("token")
		if !exists {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		// Verify its valid format
		token, err := uuid.Parse(rawToken.(string))
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		valid, err := app.isValidUserToken(token)
		if err != nil { // Could be a database error
			app.logError.Println(err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		} else if !valid { // Wrong or expired token given
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next()
	}
}

type UploadTokenVerification struct {
	UploadToken string `form:"upload_token"`
}

// Makes sure request has upload token and a valid one
func hasUploadTokenMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var uploadToken UploadTokenVerification
		var err error

		if err = c.MustBindWith(&uploadToken, binding.FormPost); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		if _, err = uuid.Parse(uploadToken.UploadToken); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"invalid upload token": err.Error()})
			c.Abort()
			return
		}

		c.Set("uploadToken", uploadToken.UploadToken)
		c.Next()
	}
}

func (app *Application) uploadTokenVerificationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Verify the field exists
		rawUploadToken, exists := c.Get("uploadToken")
		if !exists {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		// Verify its valid format
		token, err := uuid.Parse(rawUploadToken.(string))
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		valid, err := app.isValidUploadToken(token)
		if err != nil { // Could be a database error
			app.logError.Println(err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		} else if !valid { // Wrong or expired token given
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next()
	}
}
