package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/arnavsurve/gateway-registry/pkg/db"
	appHandlers "github.com/arnavsurve/gateway-registry/pkg/handlers"
	"github.com/arnavsurve/gateway-registry/pkg/types"
)

func main() {
	db, err := db.InitDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	h := appHandlers.Handler{DB: db}
	r := mux.NewRouter()
	services := r.PathPrefix("/services").Subrouter()
	services.HandleFunc("", h.ListServicesHandler).Methods(http.MethodGet)
	services.HandleFunc("/{id}", h.GetServiceHandler).Methods(http.MethodGet)
	services.HandleFunc("/{id}", h.UpdateServiceHandler).Methods(http.MethodPut)
	services.HandleFunc("/{id}", h.DeleteServiceHandler).Methods(http.MethodDelete)
	services.HandleFunc("/{id}/heartbeat", h.SearchServicesHandler).Methods(http.MethodGet)
	services.HandleFunc("/search", h.SearchServicesHandler).Methods(http.MethodGet)

	corsMiddleware := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}),
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
	)

	// Add middleware for logging
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			log.Printf("Started %s %s", r.Method, r.URL.Path)
			next.ServeHTTP(w, r)
			log.Printf("Completed %s %s in %v", r.Method, r.URL.Path, time.Since(start))
		})
	})

	// Prune inactive services
	go func() {
		for {
			// Prune every 5 min
			// TODO: change this to 1 hour in prod, change heartbeat for mock registered services as well
			time.Sleep(5 * time.Minute)

			// Remove services that haven't sent a heartbeat in the last prune cycle
			cutoff := time.Now().Add(-5 * time.Minute)
			var inactiveServices []types.MCPService
			db.Where("last_seen < ?", cutoff).Find(&inactiveServices)

			for _, service := range inactiveServices {
				// Delete related records
				db.Where("service_id = ?", service.ID).Delete(&types.Capability{})
				db.Where("service_id = ?", service.ID).Delete(&types.Category{})
				db.Where("service_id = ?", service.ID).Delete(&types.MetadataItem{})

				// Delete the service
				db.Delete(&service)
				log.Printf("Pruned inactive service: %s (%s)", service.Name, service.ID)
			}
		}
	}()

	log.Println("MCP Registry Service running at :42069")
	http.ListenAndServe(":8080", corsMiddleware(r))
}
