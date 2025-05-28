package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Database struct {
	*gorm.DB
}

type AccountModel struct {
	gorm.Model

	ID          uint
	Token       string `gorm:"type:uuid;default:uuid_generate_v4();uniqueIndex"`
	UploadToken string `gorm:"type:uuid;default:uuid_generate_v4();uniqueIndex;column:upload_token"`
	AccountType string
}

type ImageModel struct {
	gorm.Model

	ID         uint
	FileName   string
	ExpiryDate time.Time

	UploaderID uint
	Uploader   AccountModel `gorm:"foreignKey:UploaderID"`
}

var ErrInvalidDatabaseType = errors.New("Invalid database type")

func prepareDB(l *Logger, c Config) (database Database) {
	l.logInfo.Println("Setting up database")

	var err error
	if c.DatabaseType == "postgresql" {
		database.DB, err = gorm.Open(postgres.Open(c.DatabaseConnectionUrl), &gorm.Config{})
		if err != nil {
			l.logError.Fatal(err)
		}
	} else {
		l.logError.Fatal(ErrInvalidDatabaseType)
	}

	// Enable UUID extension
	if err := database.DB.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error; err != nil {
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

func (db *Database) getUserByID(userID int) (account AccountModel, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = db.WithContext(ctx).First(&account, userID).Error

	return
}

// Deletes image entry from database
func (db *Database) deleteImage(fileName string, uploadToken string) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	return db.WithContext(ctx).
		Where(&ImageModel{FileName: fileName, Uploader: AccountModel{UploadToken: uploadToken}}).
		Delete(&ImageModel{}).Error
}

func (db *Database) insertNewImageUploadToken(fileName string, uploadToken string) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// First get the user ID by upload token
	var userID uint
	if err = db.WithContext(ctx).Model(&AccountModel{}).
		Select("id").
		Where(&AccountModel{UploadToken: uploadToken}).
		Scan(&userID).Error; err != nil {
		return err
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
		Where(&ImageModel{Uploader: AccountModel{ID: userID}}).
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
		Where(&ImageModel{Uploader: AccountModel{ID: userID}}).
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
func (db *Database) idByToken(token string) (id uint, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = db.WithContext(ctx).Model(&AccountModel{}).
		Select("id").
		Where(&AccountModel{Token: token}).
		Scan(&id).Error

	return
}

// gets user id by upload token
func (db *Database) idByUploadToken(uploadToken string) (id int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = db.WithContext(ctx).Model(&AccountModel{}).
		Select("id").
		Where(&AccountModel{UploadToken: uploadToken}).
		Scan(&id).Error

	return
}

func (db *Database) replaceUploadToken(token string) (uploadToken string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Update and return the new upload token
	var account AccountModel
	err = db.WithContext(ctx).Model(&account).
		Where(&AccountModel{Token: token}).
		Update("upload_token", gorm.Expr("uuid_generate_v4()")).Error

	if err != nil {
		return "", err
	}

	// Get the updated upload token
	err = db.WithContext(ctx).Model(&AccountModel{}).
		Select("upload_token").
		Where("token = ?", token).
		Scan(&uploadToken).Error

	return
}

func (db *Database) createNewUser() (account AccountModel, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	account = AccountModel{
		AccountType: "USER",
	}

	err = db.WithContext(ctx).Create(&account).Error
	return
}

func (db *Database) createNewAdmin() (account AccountModel, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	account = AccountModel{
		AccountType: "ADMIN",
	}

	err = db.WithContext(ctx).Create(&account).Error
	return
}

func (db *Database) findAdminByToken(token string) (err error) {
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

	err = db.WithContext(ctx).
		Where("created_date < ?", time.Now().AddDate(0, 0, -7)).
		Find(&images).Error

	return
}

func (db *Database) deleteAllExpiredImages() (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = db.WithContext(ctx).
		Where("created_date < ?", time.Now().AddDate(0, 0, -7)).
		Delete(&ImageModel{}).Error

	return
}
