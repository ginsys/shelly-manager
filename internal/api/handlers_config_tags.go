package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	apiresp "github.com/ginsys/shelly-manager/internal/api/response"
	"github.com/ginsys/shelly-manager/internal/configuration"
)

type AddTagRequest struct {
	Tag string `json:"tag"`
}

type TagsResponse struct {
	Tags []string `json:"tags"`
}

type AllTagsResponse struct {
	Tags   []string       `json:"tags"`
	Counts map[string]int `json:"counts,omitempty"`
}

type TagDevicesResponse struct {
	Devices []DeviceTagInfo `json:"devices"`
}

type DeviceTagInfo struct {
	ID            uint `json:"id"`
	ConfigApplied bool `json:"config_applied"`
	HasOverrides  bool `json:"has_overrides"`
	TemplateCount int  `json:"template_count"`
}

func (h *Handler) GetDeviceNewTags(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	tags, err := h.ConfigService.ConfigurationSvc.GetDeviceTags(uint(id))
	if err != nil {
		if errors.Is(err, configuration.ErrDeviceNotFound) {
			rw.WriteNotFoundError(w, r, "Device")
			return
		}
		rw.WriteInternalError(w, r, err)
		return
	}

	rw.WriteSuccess(w, r, TagsResponse{Tags: tags})
}

func (h *Handler) AddDeviceNewTag(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	var req AddTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rw.WriteValidationError(w, r, "Invalid JSON request body")
		return
	}

	if req.Tag == "" {
		rw.WriteValidationError(w, r, "tag is required")
		return
	}

	if err := h.ConfigService.ConfigurationSvc.AddDeviceTag(uint(id), req.Tag); err != nil {
		if errors.Is(err, configuration.ErrDeviceNotFound) {
			rw.WriteNotFoundError(w, r, "Device")
			return
		}
		rw.WriteInternalError(w, r, err)
		return
	}

	tags, _ := h.ConfigService.ConfigurationSvc.GetDeviceTags(uint(id))
	rw.WriteSuccess(w, r, TagsResponse{Tags: tags})
}

func (h *Handler) RemoveDeviceNewTag(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		rw.WriteError(w, r, http.StatusBadRequest, apiresp.ErrCodeBadRequest, "Invalid device ID", nil)
		return
	}

	tag := vars["tag"]
	if tag == "" {
		rw.WriteValidationError(w, r, "tag is required")
		return
	}

	if err := h.ConfigService.ConfigurationSvc.RemoveDeviceTag(uint(id), tag); err != nil {
		if errors.Is(err, configuration.ErrDeviceNotFound) {
			rw.WriteNotFoundError(w, r, "Device")
			return
		}
		rw.WriteInternalError(w, r, err)
		return
	}

	tags, _ := h.ConfigService.ConfigurationSvc.GetDeviceTags(uint(id))
	rw.WriteSuccess(w, r, TagsResponse{Tags: tags})
}

func (h *Handler) ListAllNewTags(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	tags, err := h.ConfigService.ConfigurationSvc.ListAllTags()
	if err != nil {
		rw.WriteInternalError(w, r, err)
		return
	}

	counts := make(map[string]int)
	for _, tag := range tags {
		devices, err := h.ConfigService.ConfigurationSvc.GetDevicesByTag(tag)
		if err == nil {
			counts[tag] = len(devices)
		}
	}

	rw.WriteSuccess(w, r, AllTagsResponse{
		Tags:   tags,
		Counts: counts,
	})
}

func (h *Handler) GetDevicesByNewTag(w http.ResponseWriter, r *http.Request) {
	rw := h.responseWriter()

	vars := mux.Vars(r)
	tag := vars["tag"]
	if tag == "" {
		rw.WriteValidationError(w, r, "tag is required")
		return
	}

	devices, err := h.ConfigService.ConfigurationSvc.GetDevicesByTag(tag)
	if err != nil {
		rw.WriteInternalError(w, r, err)
		return
	}

	deviceInfos := make([]DeviceTagInfo, len(devices))
	for i, d := range devices {
		hasOverrides := d.Overrides != "" && d.Overrides != "{}"
		templateCount := 0
		if d.TemplateIDs != "" && d.TemplateIDs != "[]" {
			var ids []uint
			if err := json.Unmarshal([]byte(d.TemplateIDs), &ids); err == nil {
				templateCount = len(ids)
			}
		}

		deviceInfos[i] = DeviceTagInfo{
			ID:            d.ID,
			ConfigApplied: d.ConfigApplied,
			HasOverrides:  hasOverrides,
			TemplateCount: templateCount,
		}
	}

	rw.WriteSuccess(w, r, TagDevicesResponse{Devices: deviceInfos})
}
