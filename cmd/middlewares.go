package cmd

import (
	"errors"
	"net/http"

	"github.com/didip/tollbooth/v8"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"gorm.io/gorm"
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
		if c.Request.ContentLength > 0 {
			if err := c.Request.ParseMultipartForm(app.config.MaxUploadSize); err != nil {
				c.String(http.StatusRequestEntityTooLarge, "Too big file")
				c.Abort()
				return
			}
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
		var (
			form TokenVerification
			err  error
		)
		if err := c.MustBindWith(&form, binding.FormPost); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		var sessionToken uuid.UUID
		if sessionToken, err = uuid.Parse(form.Token); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"invalid session token": err.Error()})
			c.Abort()
			return
		}

		c.Set("token", sessionToken)
		c.Next()
	}
}

// Makes sure the admin token provided is valid
func (app *Application) adminTokenVerificationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Verify the field exists
		token, exists := c.Get("token")
		if !exists {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		isAdmin, err := app.isAdmin(token.(uuid.UUID))
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

// Makes sure the user token provided is valid
func (app *Application) userTokenVerificationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Verify the field exists
		token, exists := c.Get("token")
		if !exists {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		if _, err := app.db.getUserBySessionToken(token.(uuid.UUID)); errors.Is(err, gorm.ErrRecordNotFound) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		} else if err != nil {
			app.logError.Println(err)
			c.AbortWithStatus(http.StatusInternalServerError)
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
		var (
			form UploadTokenVerification
			err  error
		)

		if err = c.MustBindWith(&form, binding.FormPost); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		var uploadToken uuid.UUID
		if uploadToken, err = uuid.Parse(form.UploadToken); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"invalid upload token": err.Error()})
			c.Abort()
			return
		}

		c.Set("uploadToken", uploadToken)
		c.Next()
	}
}

func (app *Application) uploadTokenVerificationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Verify the field exists
		uploadToken, exists := c.Get("uploadToken")
		if !exists {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		valid, err := app.isValidUploadToken(uploadToken.(uuid.UUID))
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
