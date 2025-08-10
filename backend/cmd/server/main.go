package main

import (
	"backend/content-layer/internal/api"
	"net/http"
)

func main() {

	handler := api.NewHandler()
	http.ListenAndServe((":8080"), handler)

}
