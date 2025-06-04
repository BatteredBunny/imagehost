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

func setLinkingCookie(c *gin.Context) {
	// TODO: fix
	c.SetCookie("linking", "true", 500, "/", c.Request.URL.Hostname(), false, true)
}

func clearLinkingCookie(c *gin.Context) {
	// TODO: fix
	c.SetCookie("linking", "", -1, "/", c.Request.URL.Hostname(), false, true)
}

func setAuthCookie(sessionToken uuid.UUID, c *gin.Context) {
	// TODO: set secure when appropriate, use actual max age
	c.SetCookie(AUTH_COOKIE, sessionToken.String(), 86400*7, "/", c.Request.URL.Hostname(), false, true)
}

func clearAuthCookie(c *gin.Context) {
	// TODO: fix
	c.SetCookie(AUTH_COOKIE, "", -1, "/", c.Request.URL.Hostname(), false, true)
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
