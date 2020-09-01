package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	petname "github.com/dustinkirkland/golang-petname"
	"github.com/evanboyle/halloumi/sdk/app"
	"github.com/evanboyle/halloumi/sdk/web"
	"github.com/gorilla/mux"
)

func main() {
	app := app.NewApp("petStore")

	// a cloud web service that returns a number [0-100)
	randomNumberService := web.NewWebService("randomNumber", func() http.Handler {
		r := mux.NewRouter()
		handler := func(w http.ResponseWriter, r *http.Request) {
			rand.Seed(time.Now().UnixNano())
			fmt.Fprintf(w, "%d", rand.Intn(100))
		}
		r.HandleFunc("/", handler)
		return r
	})

	// a cloud web service that returns the name of a random animal
	randomAnimalService := web.NewWebService("randomAnimal", func() http.Handler {
		r := mux.NewRouter()
		handler := func(w http.ResponseWriter, r *http.Request) {
			animalName := petname.Generate(1 /*words*/, "" /*seperator*/)
			fmt.Fprintf(w, animalName)
		}
		r.HandleFunc("/", handler)
		return r
	})

	// a cloud web service that returns N of a random animal.
	nAnimalsService := web.NewWebService("nAnimals", func() http.Handler {
		r := mux.NewRouter()
		handler := func(w http.ResponseWriter, r *http.Request) {
			num, err := http.Get(randomNumberService.URL())
			if err != nil {
				fmt.Fprintf(w, err.Error())
			}

			animal, err := http.Get(randomAnimalService.URL())
			if err != nil {
				fmt.Fprintf(w, err.Error())
			}

			fmt.Fprintf(w, "Wow, you got %d %ss!", num, animal)
		}
		r.HandleFunc("/", handler)
		return r
	})

	app.Register(randomNumberService)
	app.Register(randomAnimalService)
	app.Register(nAnimalsService)
}
