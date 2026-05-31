package api

import "net/http"

func (s Server) staticData(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	data, err := s.static.Get(r.Context(), kind, r.URL.Query().Get("patch"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, data)
}
