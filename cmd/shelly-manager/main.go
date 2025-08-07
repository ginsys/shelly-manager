package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ginsys/shelly-manager/internal/api"
	"github.com/ginsys/shelly-manager/internal/config"
	"github.com/ginsys/shelly-manager/internal/database"
	"github.com/ginsys/shelly-manager/internal/logging"
	"github.com/ginsys/shelly-manager/internal/service"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

// Global variables
var (
	shellyService *service.ShellyService
	dbManager     *database.Manager
	cfg           *config.Config
	logger        *logging.Logger
	configFile    string
)

// Root command
var rootCmd = &cobra.Command{
	Use:   "shelly-manager",
	Short: "Manage Shelly IoT devices",
	Long: `A comprehensive tool for discovering, configuring, and managing 
Shelly smart home devices on your network.`,
}

// CLI Commands
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all devices",
	Run: func(cmd *cobra.Command, args []string) {
		devices, err := dbManager.GetDevices()
		if err != nil {
			log.Fatal("Error fetching devices:", err)
		}
		
		fmt.Printf("%-5s %-15s %-18s %-12s %-20s %-10s\n", 
			"ID", "IP", "MAC", "Type", "Name", "Status")
		fmt.Println(strings.Repeat("-", 80))
		
		for _, device := range devices {
			fmt.Printf("%-5d %-15s %-18s %-12s %-20s %-10s\n",
				device.ID, device.IP, device.MAC, device.Type, device.Name, device.Status)
		}
	},
}

var discoverCmd = &cobra.Command{
	Use:   "discover [network]",
	Short: "Discover devices on network",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var network string
		if len(args) > 0 {
			network = args[0]
		} else {
			// Use "auto" to trigger config-based discovery
			network = "auto"
		}
		
		if network == "auto" && len(cfg.Discovery.Networks) > 0 {
			fmt.Printf("Discovering devices on configured networks: %v\n", cfg.Discovery.Networks)
		} else if network != "auto" {
			fmt.Printf("Discovering devices on network %s...\n", network)
		}
		
		devices, err := shellyService.DiscoverDevices(network)
		if err != nil {
			log.Fatal("Discovery failed:", err)
		}
		
		fmt.Printf("\nFound %d devices:\n", len(devices))
		fmt.Println(strings.Repeat("-", 80))
		
		for _, device := range devices {
			fmt.Printf("IP: %-15s  MAC: %s\n", device.IP, device.MAC)
			fmt.Printf("Type: %-20s  Firmware: %s\n", device.Type, device.Firmware)
			fmt.Printf("Name: %s\n", device.Name)
			
			// Parse settings to show more info
			var settings map[string]interface{}
			if err := json.Unmarshal([]byte(device.Settings), &settings); err == nil {
				if model, ok := settings["model"].(string); ok {
					fmt.Printf("Model: %s", model)
					if gen, ok := settings["gen"].(float64); ok {
						fmt.Printf(" (Gen %d)", int(gen))
					}
					if auth, ok := settings["auth_enabled"].(bool); ok && auth {
						fmt.Printf(" [Auth Required]")
					}
					fmt.Println()
				}
			}
			
			// Check if device already exists by MAC
			_, err := dbManager.GetDeviceByMAC(device.MAC)
			if err != nil && err == gorm.ErrRecordNotFound {
				if err := dbManager.AddDevice(&device); err != nil {
					log.Printf("Failed to add device %s: %v", device.IP, err)
				} else {
					fmt.Printf("✓ Added to database\n")
				}
			} else if err == nil {
				// Update existing device with new IP if changed
				existingDevice, _ := dbManager.GetDeviceByMAC(device.MAC)
				if existingDevice.IP != device.IP {
					existingDevice.IP = device.IP
					existingDevice.LastSeen = time.Now()
					dbManager.UpdateDevice(existingDevice)
					fmt.Printf("✓ Updated IP address in database\n")
				} else {
					fmt.Printf("• Already in database\n")
				}
			}
			fmt.Println(strings.Repeat("-", 80))
		}
	},
}

var addCmd = &cobra.Command{
	Use:   "add <ip> [name]",
	Short: "Add a device by IP address",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		ip := args[0]
		name := "Unknown Device"
		if len(args) > 1 {
			name = args[1]
		}
		
		// Try to discover the specific device first
		devices, err := shellyService.DiscoverDevices(ip + "/32")
		if err != nil || len(devices) == 0 {
			log.Fatal("Could not discover device at", ip)
		}
		
		device := devices[0]
		if name != "Unknown Device" {
			device.Name = name
		}
		
		if err := dbManager.AddDevice(&device); err != nil {
			log.Fatal("Failed to add device:", err)
		}
		
		fmt.Printf("Added device: %s (%s) at %s\n", device.Name, device.Type, device.IP)
	},
}

var provisionCmd = &cobra.Command{
	Use:   "provision",
	Short: "Provision unconfigured devices",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Device provisioning is not yet implemented")
		fmt.Println("This feature will configure unconfigured Shelly devices found in AP mode")
	},
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the HTTP API server",
	Run: func(cmd *cobra.Command, args []string) {
		startServer()
	},
}

// startServer starts the HTTP API server
func startServer() {
	// Create API handler with logger
	apiHandler := api.NewHandlerWithLogger(dbManager, logger)
	
	// Setup routes with middleware
	router := api.SetupRoutesWithLogger(apiHandler, logger)
	
	// Start server
	address := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.LogAppStart("1.0.0", address)
	
	fmt.Printf("Starting server on %s\n", address)
	fmt.Printf("Web interface: http://%s\n", address)
	fmt.Printf("API base URL: http://%s/api/v1\n", address)
	
	if err := http.ListenAndServe(address, router); err != nil {
		logger.WithFields(map[string]any{
			"address": address,
			"error": err.Error(),
			"component": "server",
		}).Error("Server failed to start")
		log.Fatal("Server failed to start:", err)
	}
}

// Initialize configuration, database, and services
func initApp() {
	var err error
	
	// Load configuration
	cfg, err = config.Load(configFile)
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}
	
	// Initialize logger from config
	logger, err = logging.New(logging.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})
	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}
	
	// Set as default logger
	logging.SetDefault(logger)
	
	// Log configuration load
	logger.LogConfigLoad(configFile, nil)
	
	// Initialize database with logger
	dbManager, err = database.NewManagerWithLogger(cfg.Database.Path, logger)
	if err != nil {
		logger.WithFields(map[string]any{
			"db_path": cfg.Database.Path,
			"error": err.Error(),
			"component": "database",
		}).Error("Failed to initialize database")
		log.Fatal("Failed to initialize database:", err)
	}
	
	// Initialize service with logger
	shellyService = service.NewServiceWithLogger(dbManager, cfg, logger)
	
	logger.WithFields(map[string]any{
		"db_path": cfg.Database.Path,
		"networks": cfg.Discovery.Networks,
		"component": "app",
	}).Info("Shelly Manager initialized")
	
	fmt.Printf("Shelly Manager initialized\n")
	fmt.Printf("Database: %s\n", cfg.Database.Path)
	if len(cfg.Discovery.Networks) > 0 {
		fmt.Printf("Discovery networks: %v\n", cfg.Discovery.Networks)
	}
}

func init() {
	// Add persistent flags
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", 
		"config file (default is shelly-manager.yaml)")
	
	// Add subcommands
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(discoverCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(provisionCmd)
	rootCmd.AddCommand(serverCmd)
}

func main() {
	// Initialize before running commands
	cobra.OnInitialize(initApp)
	
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}