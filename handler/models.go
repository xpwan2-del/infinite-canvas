package handler

import (
	"net/http"

	"github.com/basketikun/infinite-canvas/service"
)

func CanvasModels(w http.ResponseWriter, r *http.Request) {
	models, err := service.FetchTopAIModels(r.Context())
	if err != nil {
		FailError(w, err)
		return
	}
	OK(w, models)
}
