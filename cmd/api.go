package cmd

import (
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Api for deleting your own account
func (app *Application) accountDeleteAPI(c *gin.Context) {
	rawToken := c.GetString("token")
	token, err := uuid.Parse(rawToken)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	userID, err := app.db.idByToken(token)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	} else if err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	if err = app.deleteAccountWithImages(userID); err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}

func (app *Application) deleteAccountWithImages(userID uint) (err error) {
	images, err := app.db.getAllImagesFromAccount(userID)
	if err != nil {
		return
	}

	for _, image := range images {
		if err = app.deleteFile(image.FileName); err != nil {
			app.logError.Println(err)
		}
	}

	if err = app.db.deleteImagesFromAccount(userID); err != nil {
		return
	}

	if err = app.db.deleteAccount(userID); err != nil {
		return
	}

	return
}

// Api for deleting an image from your account
type deleteImageAPIInput struct {
	FileName string `form:"file_name"`
}

func (app *Application) deleteImageAPI(c *gin.Context) {
	var input deleteImageAPIInput
	var err error

	if err = c.MustBindWith(&input, binding.FormPost); err != nil {
		c.String(http.StatusBadRequest, err.Error())
		c.Abort()
		return
	}

	// Makes sure the image exists
	var exists bool
	if exists, err = app.db.fileExists(input.FileName); err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	} else if !exists {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Deletes file
	if err = app.deleteFile(input.FileName); err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Should have been verified with middleware already
	rawUploadToken := c.GetString("uploadToken")
	uploadToken, err := uuid.Parse(rawUploadToken)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if err = app.db.deleteImage(input.FileName, uploadToken); err != nil { // Deletes file entry from database
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.String(http.StatusOK, "Successfully deleted the image")
}

// Api for uploading image
func (app *Application) uploadImageAPI(c *gin.Context) {
	fileRaw, _, err := c.Request.FormFile("file")
	defer fileRaw.Close()
	if err != nil {
		c.String(http.StatusBadRequest, "No file provided")
		c.Abort()
		return
	}

	file, err := io.ReadAll(fileRaw)
	if err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	fullFileName, err := app.generateFullFileName(file)
	if err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	switch app.config.fileStorageMethod {
	case fileStorageS3:
		err = app.uploadFileS3(file, fullFileName)
	case fileStorageLocal:
		err = os.WriteFile(app.config.DataFolder+fullFileName, file, 0600)
	default:
		err = ErrUnknownStorageMethod
	}

	if err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Should have been verified with middleware already
	rawUploadToken := c.GetString("uploadToken")
	uploadToken, err := uuid.Parse(rawUploadToken)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	err = app.db.insertNewImageUploadToken(fullFileName, uploadToken)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	} else if err != nil {
		app.logError.Println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, "/"+fullFileName)
}

// Api for changing your upload token
func (app *Application) newUploadTokenAPI(c *gin.Context) {
	rawToken := c.GetString("token")
	token, err := uuid.Parse(rawToken)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	uploadToken, err := app.db.replaceUploadToken(token)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			app.logError.Println(err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	c.String(http.StatusOK, uploadToken)
}
