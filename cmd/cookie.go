package cmd

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const AUTH_COOKIE = "auth"
const LINKING_COOKIE = "linking"

func (app *Application) setLinkingCookie(c *gin.Context) {
	c.SetCookie("linking", "true", 500, "/", app.config.publicUrl, gin.Mode() == gin.ReleaseMode, true)
}

func (app *Application) clearLinkingCookie(c *gin.Context) {
	c.SetCookie("linking", "", -1, "/", app.config.publicUrl, gin.Mode() == gin.ReleaseMode, true)
}

func (app *Application) setAuthCookie(sessionToken uuid.UUID, c *gin.Context) {
	// TODO: use actual max age
	c.SetCookie(AUTH_COOKIE, sessionToken.String(), 86400*7, "/", app.config.publicUrl, gin.Mode() == gin.ReleaseMode, true)
}

func (app *Application) clearAuthCookie(c *gin.Context) {
	c.SetCookie(AUTH_COOKIE, "", -1, "/", app.config.publicUrl, gin.Mode() == gin.ReleaseMode, true)
}

var ErrInvalidAuthCookie = errors.New("Invalid session token")

func (app *Application) validateCookie(c *gin.Context) (sessionToken uuid.UUID, account Accounts, loggedIn bool, err error) {
	rawSessionToken, err := c.Cookie(AUTH_COOKIE)
	if errors.Is(err, http.ErrNoCookie) {
		err = nil
		return
	} else if err != nil {
		return
	}

	sessionToken, err = parseToken(rawSessionToken)
	if err != nil {
		err = ErrInvalidAuthCookie
		return
	}

	if account, err = app.db.getUserBySessionToken(sessionToken); errors.Is(err, gorm.ErrRecordNotFound) {
		err = ErrInvalidAuthCookie
		return
	} else if err != nil {
		return
	}

	loggedIn = true

	return
}
