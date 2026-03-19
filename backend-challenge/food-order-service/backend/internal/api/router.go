package api

import "net/http"

func Router() *http.Server {

	base := "/api"

	mux := http.NewServeMux()

	mux.HandleFunc("POST "+base+"/upload", UploadHandler)
	mux.HandleFunc("POST "+base+"/process", ProcessHandler)
	mux.HandleFunc("GET "+base+"/progress", ProgressHandler)
	mux.HandleFunc("GET "+base+"/processing-status", ProcessingStatusHandler)

	mux.HandleFunc("GET "+base+"/product", ListProducts)
	mux.HandleFunc("GET "+base+"/product/", GetProduct)
	mux.HandleFunc("POST "+base+"/order", requireAPIKey(PlaceOrder))

	mux.HandleFunc("GET "+base+"/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./public/openapi.yaml")
	})

	srv := &http.Server{Addr: ":8080", Handler: mux}
	return srv
}
