package cmd

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Admin api for deleting user
type adminDeleteUserInput struct {
	ID uint `form:"id"`
}

var ErrCantDeleteSelf = fmt.Errorf("you can't delete yourself")

func (app *Application) adminDeleteUser(c *gin.Context) {
	var (
		input adminDeleteUserInput
		err   error
	)

	if err = c.MustBindWith(&input, binding.FormPost); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	sessionToken, exists := c.Get("sessionToken")
	if !exists {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// You can't delete yourself
	if account, err := app.db.getAccountBySessionToken(sessionToken.(uuid.UUID)); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	} else if account.ID == input.ID {
		c.AbortWithError(http.StatusBadRequest, ErrCantDeleteSelf)
		return
	}

	if err = app.deleteAccount(input.ID); err != nil {
		log.Err(err).Msg("Failed to delete account")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.String(http.StatusOK, fmt.Sprintf("User %d deleted", input.ID))
}

type adminDeleteUserFilesInput struct {
	ID uint `form:"id"`
}

func (app *Application) adminDeleteFiles(c *gin.Context) {
	var (
		input adminDeleteUserFilesInput
		err   error
	)

	if err = c.MustBindWith(&input, binding.FormPost); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if err = app.deleteFilesFromAccount(input.ID); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.String(http.StatusOK, "Files deleted")
}
