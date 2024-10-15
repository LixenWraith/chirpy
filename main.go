package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf(`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, cfg.fileserverHits.Load())))
}

func (cfg *apiConfig) handleMetricsReset(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits.Store(0)
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintf("Metrics reset: %d", cfg.fileserverHits.Load())))
}

func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(http.StatusText(http.StatusOK)))
}

func handlerValidate(w http.ResponseWriter, r *http.Request) {
	type input struct {
		Body string `json:"body"`
	}

	type error_response struct {
		Error string `json:"error"`
	}

	type valid_reponse struct {
		Valid bool `json:"valid"`
	}

	decoder := json.NewDecoder(r.Body)

	params := input{}

	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't decode", err)
	}

	const maxChirpyLength = 140
	if len(params.Body) > maxChirpyLength {
		respondWithError(w, http.StatusBadRequest, "chirpy looong", nil)
	}

	respondWithJSON(w, http.StatusOK, valid_reponse{
		Valid: true,
	})

}

func respondWithError(w http.ResponseWriter, code int, msg string, err error) {
	if err != nil {
		log.Println(err)
	}
	if code > 499 {
		log.Printf("Responding with 5XX error: %s", msg)
	}
	type errorResponse struct {
		Error string `json:"error"`
	}
	respondWithJSON(w, code, errorResponse{
		Error: msg,
	})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(code)
	w.Write(dat)
}

func main() {
	const filepath = "./site"
	const port = "8080"
	const readinesspath = "GET /api/healthz"
	const metricspath = "GET /admin/metrics"
	const metricsreset = "POST /admin/reset"
	const validatepath = "POST /api/validate_chirp"

	mux := http.NewServeMux()

	apiCfg := &apiConfig{}

	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(filepath)))))

	mux.HandleFunc(readinesspath, handlerReadiness)
	mux.HandleFunc(validatepath, handlerValidate)
	mux.HandleFunc(metricspath, apiCfg.handleMetrics)
	mux.HandleFunc(metricsreset, apiCfg.handleMetricsReset)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	log.Printf("Serving files from %s on port: %s\n", filepath, port)
	log.Fatal(srv.ListenAndServe())
}
