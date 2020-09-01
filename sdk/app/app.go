package app

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/evanboyle/halloumi/sdk/web"
)

type App struct {
	Name string
}

func (a *App) Register(w web.WebService) {
	if os.Getenv("DRY_RUN") != "" {
		fmt.Printf("\nHALLOUMI::webservice____%s____%s\n", a.Name, w.Name)
		return
	}
	if os.Getenv(fmt.Sprintf("HALLOUMI_%s_%s", a.Name, w.Name)) != "" {
		handler := w.Fn()
		s := &http.Server{
			Addr:    ":80",
			Handler: handler,
		}
		log.Fatal(s.ListenAndServe())
	}
}

func NewApp(name string) App {
	if os.Getenv("DRY_RUN") != "" {
		fmt.Printf("\nHALLOUMI::app____%s\n", name)
	}
	return App{
		Name: name,
	}
}
