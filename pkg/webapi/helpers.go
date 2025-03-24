package webapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	giga "github.com/dogecoinfoundation/gigawallet/pkg"
)

var httpCodeForError = map[string]int{
	string(giga.BadRequest):    400,
	string(giga.NotAvailable):  503,
	string(giga.NotFound):      404,
	string(giga.AlreadyExists): 500,
	string(giga.Unauthorized):  401,
	string(giga.UnknownError):  500,
}

func HttpStatusForError(code giga.ErrorCode) int {
	status, found := httpCodeForError[string(code)]
	if !found {
		status = http.StatusInternalServerError
	}
	return status
}

func sendResponse(w http.ResponseWriter, payload any) {
	// note: w.Header after this, so we can call sendError
	b, err := json.Marshal(payload)
	if err != nil {
		sendErrorResponse(w, http.StatusInternalServerError, "marshal", fmt.Sprintf("in json.Marshal: %s", err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store") // do not cache (Browsers cache GET forever by default)
	w.Write(b)
}

func sendBadRequest(w http.ResponseWriter, message string) {
	sendErrorResponse(w, http.StatusBadRequest, giga.BadRequest, message)
}

func sendError(w http.ResponseWriter, where string, err error) {
	var info *giga.ErrorInfo
	if errors.As(err, &info) {
		status := HttpStatusForError(info.Code)
		message := fmt.Sprintf("%s: %s", where, info.Message)
		sendErrorResponse(w, status, info.Code, message)
	} else {
		message := fmt.Sprintf("%s: %s", where, err.Error())
		sendErrorResponse(w, http.StatusInternalServerError, giga.UnknownError, message)
	}
}

func sendErrorResponse(w http.ResponseWriter, statusCode int, code giga.ErrorCode, message string) {
	log.Printf("[!] %s: %s\n", code, message)
	// would prefer to use json.Marshal, but this avoids the need
	// to handle encoding errors arising from json.Marshal itself!
	payload := fmt.Sprintf("{\"error\":{\"code\":%q,\"message\":%q}}", code, message)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store") // do not cache (Browsers cache GET forever by default)
	w.WriteHeader(statusCode)
	w.Write([]byte(payload))
}
