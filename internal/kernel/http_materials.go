package kernel

import (
	"net/http"
)

func handleIntakeMaterial(w http.ResponseWriter, r *http.Request, k *Kernel) {
	var req MaterialIntakeRequest
	if !decodeRequest(w, r, &req) {
		return
	}
	projection, err := k.IntakeMaterial(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, projection)
		return
	}
	writeJSON(w, http.StatusOK, projection)
}

func handleUploadMaterial(w http.ResponseWriter, r *http.Request, k *Kernel) {
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxMaterialUploadBytes)+1024*1024)
	if err := r.ParseMultipartForm(int64(maxMaterialUploadBytes)); err != nil {
		writeJSON(w, http.StatusBadRequest, refusedMaterialIntake("invalid_upload", err.Error()))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, refusedMaterialIntake("invalid_upload", "multipart file field is required"))
		return
	}
	defer file.Close()
	filename := ""
	if header != nil {
		filename = header.Filename
	}
	projection, err := k.IntakeUploadedMaterial(r.FormValue("session_id"), r.FormValue("purpose"), filename, file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, projection)
		return
	}
	writeJSON(w, http.StatusOK, projection)
}
