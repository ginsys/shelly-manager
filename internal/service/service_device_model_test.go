package service

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ginsys/shelly-manager/internal/database"
)

func TestDeviceModel(t *testing.T) {
	cases := []struct {
		name   string
		device database.Device
		want   string
	}{
		{"model from settings wins over the display type", database.Device{Type: "Smart Plug", Settings: `{"model":"SHPLG-S","name":"x"}`}, "SHPLG-S"},
		{"no settings falls back to type", database.Device{Type: "SHSW-1"}, "SHSW-1"},
		{"settings without model falls back to type", database.Device{Type: "Smart Plug", Settings: `{"name":"x"}`}, "Smart Plug"},
		{"unparseable settings falls back to type", database.Device{Type: "Smart Plug", Settings: `{not json`}, "Smart Plug"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, deviceModel(&tc.device))
		})
	}
}
