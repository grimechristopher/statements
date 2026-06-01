package main

import (
	"fmt"
	"net/http"
)

func main() {
	fmt.Println("pay-dashboard starting on :8080")
	http.ListenAndServe(":8080", nil)
}
