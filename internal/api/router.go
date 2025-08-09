package api

import (
	"net/http"

	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/gorilla/mux"
)

// SetupRoutes configures all API routes
func SetupRoutes(handler *Handler) *mux.Router {
	return SetupRoutesWithLogger(handler, logging.GetDefault())
}

// SetupRoutesWithLogger configures all API routes with logging middleware
func SetupRoutesWithLogger(handler *Handler, logger *logging.Logger) *mux.Router {
	r := mux.NewRouter()
	
	// Add logging middleware
	r.Use(logging.HTTPMiddleware(logger))
	r.Use(logging.RecoveryMiddleware(logger))
	r.Use(logging.CORSMiddleware(logger))

	// API routes
	api := r.PathPrefix("/api/v1").Subrouter()
	
	// Device routes
	api.HandleFunc("/devices", handler.GetDevices).Methods("GET")
	api.HandleFunc("/devices", handler.AddDevice).Methods("POST")
	api.HandleFunc("/devices/{id}", handler.GetDevice).Methods("GET")
	api.HandleFunc("/devices/{id}", handler.UpdateDevice).Methods("PUT")
	api.HandleFunc("/devices/{id}", handler.DeleteDevice).Methods("DELETE")
	
	// Device control routes
	api.HandleFunc("/devices/{id}/control", handler.ControlDevice).Methods("POST")
	api.HandleFunc("/devices/{id}/status", handler.GetDeviceStatus).Methods("GET")
	api.HandleFunc("/devices/{id}/energy", handler.GetDeviceEnergy).Methods("GET")
	
	// Discovery route
	api.HandleFunc("/discover", handler.DiscoverHandler).Methods("POST")
	
	// Provisioning routes
	api.HandleFunc("/provisioning/status", handler.GetProvisioningStatus).Methods("GET")
	api.HandleFunc("/provisioning/provision", handler.ProvisionDevices).Methods("POST")
	
	// DHCP routes
	api.HandleFunc("/dhcp/reservations", handler.GetDHCPReservations).Methods("GET")

	// Static file serving
	r.PathPrefix("/web/").Handler(http.StripPrefix("/web/", http.FileServer(http.Dir("./web/static/"))))
	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("./web/static/"))))

	return r
}