package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Database struct {
	*gorm.DB
}

type AccountModel struct {
	gorm.Model

	ID          uint      // Internal numeric account ID
	Token       uuid.UUID `gorm:"uniqueIndex"` // Token for account or admin actions
	UploadToken uuid.UUID `gorm:"uniqueIndex"` // Token that authorizes uploads for that account
	AccountType string    // Either "USER" or "ADMIN"
}

type ImageModel struct {
	gorm.Model

	ID         uint // Internal numeric image ID
	FileName   string
	ExpiryDate time.Time // Time when the image will be deleted

	UploaderID uint
	Uploader   AccountModel `gorm:"foreignKey:UploaderID"`
}

var ErrInvalidDatabaseType = errors.New("Invalid database type")

func prepareDB(l *Logger, c Config) (database Database) {
	l.logInfo.Println("Setting up database")

	var gormConnection gorm.Dialector
	if c.DatabaseType == "postgresql" {
		gormConnection = postgres.Open(c.DatabaseConnectionUrl)
	} else if c.DatabaseType == "sqlite" {
		gormConnection = sqlite.Open(c.DatabaseConnectionUrl)
	} else {
		l.logError.Fatal(ErrInvalidDatabaseType)
	}

	var err error
	database.DB, err = gorm.Open(gormConnection, &gorm.Config{})
	if err != nil {
		l.logError.Fatal(err)
	}

	if err := database.DB.AutoMigrate(&AccountModel{}, &ImageModel{}); err != nil {
		l.logError.Fatal(err)
	}

	// Create the first admin user if no user with ID 1 exists
	if _, err := database.getUserByID(1); errors.Is(err, gorm.ErrRecordNotFound) {
		var user AccountModel
		user, err = database.createNewAdmin()
		if err != nil {
			l.logError.Fatal(err)
		}

		var jsonData []byte
		if jsonData, err = json.MarshalIndent(user, "", "\t"); err != nil {
			l.logError.Fatal(err)
		} else {
			l.logInfo.Println("Created first account: ", string(jsonData))
		}
	}

	return
}

func (db *Database) getUserByID(userID uint) (account AccountModel, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = db.WithContext(ctx).
		Where(&AccountModel{ID: userID}).
		First(&account).Error

	return
}

// Deletes image entry from database
func (db *Database) deleteImage(fileName string, uploadToken uuid.UUID) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	return db.WithContext(ctx).
		Where(&ImageModel{FileName: fileName, Uploader: AccountModel{UploadToken: uploadToken}}).
		Delete(&ImageModel{}).Error
}

func (db *Database) insertNewImageUploadToken(fileName string, uploadToken uuid.UUID) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// First get the user ID by upload token
	var userID uint
	err = db.WithContext(ctx).Model(&AccountModel{}).
		Select("id").
		Where(&AccountModel{UploadToken: uploadToken}).
		First(&userID).Error
	if err != nil {
		return
	}

	// Insert new image
	return db.WithContext(ctx).Create(&ImageModel{
		FileName:   fileName,
		UploaderID: userID,
	}).Error
}

func (db *Database) deleteImagesFromAccount(userID uint) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	return db.WithContext(ctx).
		Where(&ImageModel{UploaderID: userID}).
		Delete(&ImageModel{}).Error
}

func (db *Database) deleteAccount(userID uint) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	return db.WithContext(ctx).Delete(&AccountModel{}, userID).Error
}

func (db *Database) getAllImagesFromAccount(userID uint) (images []ImageModel, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = db.WithContext(ctx).
		Select("file_name").
		Where(&ImageModel{UploaderID: userID}).
		Find(&images).Error

	return
}

// Looks if file exists in database
func (db *Database) fileExists(fileName string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var count int64
	err := db.WithContext(ctx).Model(&ImageModel{}).
		Where(&ImageModel{FileName: fileName}).
		Count(&count).Error

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// gets user id by token
func (db *Database) idByToken(token uuid.UUID) (id uint, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = db.WithContext(ctx).Model(&AccountModel{}).
		Select("id").
		Where(&AccountModel{Token: token}).
		Scan(&id).Error

	return
}

// gets user id by upload token
func (db *Database) idByUploadToken(uploadToken uuid.UUID) (id int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = db.WithContext(ctx).Model(&AccountModel{}).
		Select("id").
		Where(&AccountModel{UploadToken: uploadToken}).
		Scan(&id).Error

	return
}

func (db *Database) replaceUploadToken(token uuid.UUID) (uploadToken string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Update and return the new upload token
	var account AccountModel
	err = db.WithContext(ctx).Model(&account).
		Where(&AccountModel{Token: token}).
		Update("upload_token", uuid.New()).Error

	if err != nil {
		return "", err
	}

	// Get the updated upload token
	err = db.WithContext(ctx).Model(&AccountModel{}).
		Select("upload_token").
		Where(&AccountModel{Token: token}).
		Scan(&uploadToken).Error

	return
}

func (db *Database) createNewUser() (account AccountModel, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	account = AccountModel{
		AccountType: "USER",
		Token:       uuid.New(),
		UploadToken: uuid.New(),
	}

	err = db.WithContext(ctx).Create(&account).Error
	return
}

func (db *Database) createNewAdmin() (account AccountModel, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	account = AccountModel{
		AccountType: "ADMIN",
		Token:       uuid.New(),
		UploadToken: uuid.New(),
	}

	err = db.WithContext(ctx).Create(&account).Error
	return
}

func (db *Database) findAdminByToken(token uuid.UUID) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var count int64
	err = db.WithContext(ctx).Model(&AccountModel{}).
		Where(&AccountModel{Token: token, AccountType: "ADMIN"}).
		Count(&count).Error

	if err != nil {
		return err
	}

	if count == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (db *Database) findAllExpiredImages() (images []ImageModel, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// TODO: use expiry date field
	err = db.WithContext(ctx).
		Where("created_date < ?", time.Now().AddDate(0, 0, -7)).
		Find(&images).Error

	return
}

func (db *Database) deleteAllExpiredImages() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// TODO: use expiry date field
	err = db.WithContext(ctx).
		Where("created_date < ?", time.Now().AddDate(0, 0, -7)).
		Delete(&ImageModel{}).Error

	return
}
