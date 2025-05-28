package cmd

import (
	"errors"
	"os"

	"crypto/rand"

	"github.com/google/uuid"
	"github.com/h2non/filetype"
	"github.com/h2non/filetype/types"
	"gorm.io/gorm"
)

func (app *Application) deleteFile(fileName string) (err error) {
	switch app.config.fileStorageMethod {
	case fileStorageLocal:
		err = os.Remove(app.config.DataFolder + fileName)
	case fileStorageS3:
		err = app.deleteFileS3(fileName)
	default:
		err = ErrUnknownStorageMethod
	}

	return
}

func randomString() string {
	return rand.Text()
}

func (app *Application) generateFullFileName(file []byte) (name string, err error) {
	var extension types.Type
	extension, err = filetype.Get(file)
	if err != nil {
		return
	}

	if extension.Extension == "unknown" { // Unknown file type defaults to txt
		name = randomString() + "." + "txt"
		return
	}

	name = randomString() + "." + extension.Extension
	return
}

func (app *Application) isValidUserToken(token uuid.UUID) (bool, error) {
	if _, err := app.db.idByToken(token); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
func (app *Application) isValidUploadToken(uploadToken uuid.UUID) (bool, error) {
	if _, err := app.db.idByUploadToken(uploadToken); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}
