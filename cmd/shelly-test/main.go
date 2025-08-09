package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ginsys/shelly-manager/internal/shelly/gen1"
)

func main() {
	var (
		ip       = flag.String("ip", "", "Device IP address")
		action   = flag.String("action", "status", "Action: status, on, off, toggle, info, power")
		username = flag.String("user", "", "Username for authentication")
		password = flag.String("pass", "", "Password for authentication")
	)
	flag.Parse()

	if *ip == "" {
		fmt.Println("Usage: shelly-test -ip <device-ip> -action <status|on|off|toggle|info|power> [-user <username> -pass <password>]")
		fmt.Println("\nExample:")
		fmt.Println("  shelly-test -ip 172.31.103.100 -action status")
		fmt.Println("  shelly-test -ip 172.31.103.100 -action on -user admin -pass secret")
		os.Exit(1)
	}

	// Create client with optional auth
	var opts []gen1.ClientOption
	if *username != "" && *password != "" {
		opts = append(opts, gen1.WithAuth(*username, *password))
		fmt.Printf("Using authentication as user: %s\n", *username)
	}
	
	client := gen1.NewClient(*ip, opts...)
	ctx := context.Background()

	switch *action {
	case "info":
		// Get device information
		info, err := client.GetInfo(ctx)
		if err != nil {
			log.Fatalf("Failed to get device info: %v", err)
		}
		
		fmt.Println("\n=== Device Information ===")
		fmt.Printf("Model: %s\n", info.Model)
		fmt.Printf("MAC: %s\n", info.MAC)
		fmt.Printf("Firmware: %s\n", info.FW)
		fmt.Printf("Auth Required: %v\n", info.Auth)
		fmt.Printf("Device ID: %s\n", info.ID)

	case "status":
		// Get device status
		status, err := client.GetStatus(ctx)
		if err != nil {
			log.Fatalf("Failed to get device status: %v", err)
		}
		
		fmt.Println("\n=== Device Status ===")
		fmt.Printf("Temperature: %.1f°C\n", status.Temperature)
		fmt.Printf("Uptime: %d seconds\n", status.Uptime)
		
		if status.WiFiStatus != nil {
			fmt.Printf("WiFi Connected: %v\n", status.WiFiStatus.Connected)
			fmt.Printf("WiFi SSID: %s\n", status.WiFiStatus.SSID)
			fmt.Printf("WiFi RSSI: %d dBm\n", status.WiFiStatus.RSSI)
		}
		
		// Show relay/switch status
		for i, sw := range status.Switches {
			fmt.Printf("\nSwitch %d:\n", i)
			fmt.Printf("  State: %v\n", sw.Output)
			fmt.Printf("  Power: %.2f W\n", sw.APower)
			fmt.Printf("  Source: %s\n", sw.Source)
		}
		
		// Show power meters
		for i, meter := range status.Meters {
			fmt.Printf("\nMeter %d:\n", i)
			fmt.Printf("  Power: %.2f W\n", meter.Power)
			fmt.Printf("  Total: %.3f kWh\n", meter.Total/1000)
			fmt.Printf("  Valid: %v\n", meter.IsValid)
		}

	case "on":
		// Turn on the device
		fmt.Println("Turning device ON...")
		err := client.SetSwitch(ctx, 0, true)
		if err != nil {
			log.Fatalf("Failed to turn on device: %v", err)
		}
		fmt.Println("✅ Device turned ON")

	case "off":
		// Turn off the device
		fmt.Println("Turning device OFF...")
		err := client.SetSwitch(ctx, 0, false)
		if err != nil {
			log.Fatalf("Failed to turn off device: %v", err)
		}
		fmt.Println("✅ Device turned OFF")

	case "toggle":
		// Toggle the device state
		fmt.Println("Getting current state...")
		status, err := client.GetStatus(ctx)
		if err != nil {
			log.Fatalf("Failed to get device status: %v", err)
		}
		
		if len(status.Switches) == 0 {
			log.Fatal("No switches found on device")
		}
		
		currentState := status.Switches[0].Output
		newState := !currentState
		
		fmt.Printf("Current state: %v, switching to: %v\n", currentState, newState)
		err = client.SetSwitch(ctx, 0, newState)
		if err != nil {
			log.Fatalf("Failed to toggle device: %v", err)
		}
		fmt.Printf("✅ Device toggled to %v\n", newState)

	case "power":
		// Get detailed power/energy data
		energy, err := client.GetEnergyData(ctx, 0)
		if err != nil {
			// Try getting from status if direct energy endpoint fails
			status, err2 := client.GetStatus(ctx)
			if err2 != nil {
				log.Fatalf("Failed to get power data: %v", err)
			}
			
			if len(status.Meters) > 0 {
				fmt.Println("\n=== Power Consumption ===")
				fmt.Printf("Current Power: %.2f W\n", status.Meters[0].Power)
				fmt.Printf("Total Energy: %.3f kWh\n", status.Meters[0].Total/1000)
			} else {
				fmt.Println("No power metering available on this device")
			}
		} else {
			fmt.Println("\n=== Energy Data ===")
			fmt.Printf("Current Power: %.2f W\n", energy.Power)
			fmt.Printf("Total Energy: %.3f kWh\n", energy.Total)
			fmt.Printf("Voltage: %.1f V\n", energy.Voltage)
			fmt.Printf("Current: %.3f A\n", energy.Current)
			if energy.PowerFactor > 0 {
				fmt.Printf("Power Factor: %.2f\n", energy.PowerFactor)
			}
		}

	case "config":
		// Get device configuration
		config, err := client.GetConfig(ctx)
		if err != nil {
			log.Fatalf("Failed to get device config: %v", err)
		}
		
		fmt.Println("\n=== Device Configuration ===")
		fmt.Printf("Name: %s\n", config.Name)
		fmt.Printf("Timezone: %s\n", config.Timezone)
		
		if config.Cloud != nil {
			fmt.Printf("Cloud Enabled: %v\n", config.Cloud.Enable)
		}
		
		// Pretty print full config
		configJSON, _ := json.MarshalIndent(config, "", "  ")
		fmt.Println("\nFull Configuration:")
		fmt.Println(string(configJSON))

	default:
		fmt.Printf("Unknown action: %s\n", *action)
		fmt.Println("Valid actions: status, on, off, toggle, info, power, config")
		os.Exit(1)
	}
}