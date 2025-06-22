package cmd

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/rs/zerolog/log"
)

// Admin api for creating new user
func (app *Application) adminCreateUser(c *gin.Context) {
	user, err := app.db.createAccount("ADMIN")
	if err != nil {
		log.Err(err).Msg("Failed to create admin account")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, user)
}

// Admin api for deleting user
type adminDeleteUserInput struct {
	ID uint `form:"id"`
}

func (app *Application) adminDeleteUser(c *gin.Context) {
	var input adminDeleteUserInput
	var err error

	if err = c.MustBindWith(&input, binding.FormPost); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		c.Abort()
		return
	}

	if err = app.deleteAccountWithImages(input.ID); err != nil {
		log.Err(err).Msg("Failed to delete account")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.String(http.StatusOK, fmt.Sprintf("User %d deleted", input.ID))
}
