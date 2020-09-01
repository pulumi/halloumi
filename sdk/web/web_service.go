package web

import (
	"fmt"
	"net/http"
	"os"
)

// WebService runs an http handler in the cloud
type WebService struct {
	Name string
	Fn   WebServiceFunc
}

// URL returns the the URL of the webservice
func (w *WebService) URL() string {
	// when another process is deploying, use an environment variable
	// when self is deploying, use localhost:80
	return "https://" + os.Getenv(fmt.Sprintf("HALLOUMI_%s_URL", w.Name))
}

type WebServiceFunc = func() http.Handler

func NewWebService(name string, fn WebServiceFunc) WebService {
	return WebService{
		Name: name,
		Fn:   fn,
	}
}
