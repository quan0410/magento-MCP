package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/datumbridge/magento-mcp/internal/tools"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	toolName := "magento_list_invoices"
	argsJSON := "{}"

	if len(os.Args) > 1 {
		toolName = os.Args[1]
	}
	if len(os.Args) > 2 {
		argsJSON = os.Args[2]
	}

	_, handlers := tools.Register()
	h, ok := handlers[toolName]
	if !ok {
		fmt.Printf("Tool %s not found\n", toolName)
		os.Exit(1)
	}

	res := h([]byte(argsJSON))
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal result: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(b))
}
