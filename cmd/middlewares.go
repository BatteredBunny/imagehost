package cmd

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/didip/tollbooth/v8"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
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

func (app *Application) parseTokenFromForm(c *gin.Context) (sessionToken uuid.UUID, err error) {
	var form TokenVerification
	if err = c.ShouldBindWith(&form, binding.FormPost); err != nil {
		return
	}

	sessionToken, err = uuid.Parse(form.Token)

	return
}

// Makes sure request has token and a valid one
func (app *Application) hasSessionTokenMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionToken, err := app.parseTokenFromForm(c)
		if err != nil {
			// Fallback to checking cookie
			log.Info().Msg("Validating cookie")
			var loggedIn bool
			sessionToken, _, loggedIn, _ = app.validateCookie(c)
			if loggedIn {
			} else {
				c.AbortWithError(http.StatusUnauthorized, err)
				return
			}
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
			log.Err(err).Msg("Failed to check if user is admin")
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
func (app *Application) sessionTokenVerificationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Verify the field exists
		token, exists := c.Get("token")
		if !exists {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if _, err := app.db.getUserBySessionToken(token.(uuid.UUID)); errors.Is(err, gorm.ErrRecordNotFound) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		} else if err != nil {
			log.Err(err).Msg("Failed to find user by session token")
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
		var (
			form UploadTokenVerification
			err  error
		)

		if err = c.MustBindWith(&form, binding.FormPost); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		if form.UploadToken == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "upload token is required"})
			c.Abort()
			return
		}

		var uploadToken uuid.UUID
		if uploadToken, err = uuid.Parse(form.UploadToken); err != nil {
			var errStr = fmt.Sprintf("invalid upload token: %s", err.Error())
			c.JSON(http.StatusUnauthorized, gin.H{"error": errStr})
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
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		valid, err := app.isValidUploadToken(uploadToken.(uuid.UUID))
		if err != nil { // Could be a database error
			log.Err(err).Msg("Failed to check if upload token is valid")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		} else if !valid { // Wrong or expired token given
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next()
	}
}
