package api

import (
	"encoding/json"
	"net/http"
)

func ProgressHandler(w http.ResponseWriter, r *http.Request) {

	resp := map[string]uint64{
		"processed": couponProcessor.Stats(),
	}

	json.NewEncoder(w).Encode(resp)

}

func ProcessingStatusHandler(w http.ResponseWriter, r *http.Request) {

	status := couponProcessor.Status()

	json.NewEncoder(w).Encode(status)

}
