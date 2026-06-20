package handler

import (
	"net/http"

	"github.com/basketikun/infinite-canvas/service"
)

func TopAISession(w http.ResponseWriter, r *http.Request) {
	session, err := service.LoginWithTopAI(r)
	if err != nil {
		FailError(w, err)
		return
	}
	OK(w, session)
}

func CurrentUser(w http.ResponseWriter, r *http.Request) {
	if user, ok := service.UserFromContext(r.Context()); ok {
		OK(w, user)
		return
	}
	OK(w, service.GuestUser())
}
