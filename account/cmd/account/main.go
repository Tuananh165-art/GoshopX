package main

import (
	"log"
	"time"

	"github.com/Tuananh165art/GoshopX/account/config"
	"github.com/Tuananh165art/GoshopX/account/internal"
	"github.com/tinrab/retry"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	var repository internal.Repository

	retry.ForeverSleep(2*time.Second, func(_ int) (err error) {
		db, err := gorm.Open(postgres.Open(config.DatabaseURL), &gorm.Config{})
		if err != nil {
			log.Fatal(err)
		}
		repository, err = internal.NewPostgresRepository(db)
		if err != nil {
			log.Println(err)
		}
		return
	})
	defer repository.Close()
	log.Println("Listening on port 8080...")
	service := internal.NewService(repository)
	log.Fatal(internal.ListenGRPC(service, 8080))
}

