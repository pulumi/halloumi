package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	petname "github.com/dustinkirkland/golang-petname"
	"github.com/gorilla/mux"
	"github.com/prometheus/common/log"
	"github.com/pulumi/halloumi/sdk/app"
	"github.com/pulumi/halloumi/sdk/web"
)

func main() {
	app := app.NewApp("HalloumiPetStore")

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
	// this service takes a dependency on the URLs of the previously defined services
	nAnimalsService := web.NewWebService("nAnimals", func() http.Handler {
		r := mux.NewRouter()
		var num string
		var animal string
		handler := func(w http.ResponseWriter, r *http.Request) {
			// Notice how we have the URL of randomNumberService
			// available to consume here!
			numResp, err := http.Get(randomNumberService.URL())
			if err != nil {
				fmt.Fprintf(w, err.Error())
			}
			defer numResp.Body.Close()

			if numResp.StatusCode == http.StatusOK {
				bodyBytes, err := ioutil.ReadAll(numResp.Body)
				if err != nil {
					log.Fatal(err)
				}
				num = string(bodyBytes)
			}

			// Notice how we have the URL of randomAnimalService
			// available to consume here!
			animalResp, err := http.Get(randomAnimalService.URL())
			if err != nil {
				fmt.Fprintf(w, err.Error())
			}
			defer numResp.Body.Close()

			if animalResp.StatusCode == http.StatusOK {
				bodyBytes, err := ioutil.ReadAll(animalResp.Body)
				if err != nil {
					log.Fatal(err)
				}
				animal = string(bodyBytes)
			}

			fmt.Fprintf(w, "Wow, you got %s %ss!", num, animal)
		}
		r.HandleFunc("/", handler)
		return r
	})

	app.Register(randomNumberService)
	app.Register(randomAnimalService)
	app.Register(nAnimalsService)
}
