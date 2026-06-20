package handler

import (
	"net/http"

	"github.com/basketikun/infinite-canvas/service"
)

func Settings(w http.ResponseWriter, r *http.Request) {
	settings, err := service.PublicSettings()
	if err != nil {
		FailError(w, err)
		return
	}
	OK(w, settings)
}
