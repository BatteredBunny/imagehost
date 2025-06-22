package cmd

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/rs/zerolog/log"
)

// Admin api for deleting user
type adminDeleteUserInput struct {
	ID uint `form:"id"`
}

func (app *Application) adminDeleteUser(c *gin.Context) {
	// TODO: disallow deleting your own account with this
	var input adminDeleteUserInput
	var err error

	if err = c.MustBindWith(&input, binding.FormPost); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if err = app.deleteAccount(input.ID); err != nil {
		log.Err(err).Msg("Failed to delete account")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.String(http.StatusOK, fmt.Sprintf("User %d deleted", input.ID))
}
