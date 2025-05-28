package cmd

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
)

// Checks if the user is an admin with token
func (app *Application) isAdmin(sessionToken uuid.UUID) (isAdmin bool, err error) {
	account, err := app.db.getUserBySessionToken(sessionToken)
	if err != nil {
		return
	}

	isAdmin = account.AccountType == "ADMIN"

	return
}

// Admin api for creating new user
func (app *Application) adminCreateUser(c *gin.Context) {
	user, err := app.db.createAccount("ADMIN")
	if err != nil {
		app.logError.Println(err)
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
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.String(http.StatusOK, fmt.Sprintf("User %d deleted", input.ID))
}
