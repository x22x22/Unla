package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"bytes"
	"errors"
	"io"
	"regexp"

	"github.com/amoylab/unla/internal/template"
)

//go:embed playground.html
var playgroundHTML embed.FS

var (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
)

var (
	responseBodyFile = flag.String("response-body", "", "Path to response body JSON file (e.g., response.body.json)")
	templateFile     = flag.String("template", "", "Path to template file (e.g., response.template.txt)")
	argsFile         = flag.String("args", "", "Path to arguments JSON file (optional)")
	configFile       = flag.String("config", "", "Path to config JSON file (optional)")
	verbose          = flag.Bool("v", false, "Verbose output with detailed debugging info")
	noColor          = flag.Bool("no-color", false, "Disable colored output")
	port             = flag.Int("port", 8080, "Port to run the playground server on (serve command only)")
)

type TestInput struct {
	ResponseBody map[string]any    `json:"responseBody,omitempty"`
	Args         map[string]any    `json:"args,omitempty"`
	Config       map[string]string `json:"config,omitempty"`
	Template     string            `json:"template,omitempty"`
}

type RenderRequest struct {
	Template     string            `json:"template"`
	ResponseBody map[string]any    `json:"responseBody,omitempty"`
	Args         map[string]any    `json:"args,omitempty"`
	Config       map[string]string `json:"config,omitempty"`
}

type RenderResponse struct {
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type FixRequest struct {
	RenderRequest
	Error string `json:"error"`
}

type FixResponse struct {
	FixedTemplate string `json:"fixedTemplate"`
	Result        string `json:"result,omitempty"`
	Error         string `json:"error,omitempty"`
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Template Tester - Test Go template parsing for MCP Gateway\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s [command] [options]\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "Commands:\n")
		fmt.Fprintf(os.Stderr, "  serve    Start web-based playground server\n")
		fmt.Fprintf(os.Stderr, "  test     Test templates from command line (default)\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Start playground server\n")
		fmt.Fprintf(os.Stderr, "  %s serve -port 8080\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "  # Test with separate files\n")
		fmt.Fprintf(os.Stderr, "  %s -response-body response.body.json -template response.template.txt\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "  # Test with verbose output\n")
		fmt.Fprintf(os.Stderr, "  %s -response-body response.body.json -template response.template.txt -v\n\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "  # Include args and config\n")
		fmt.Fprintf(os.Stderr, "  %s -response-body response.body.json -template response.template.txt -args args.json -config config.json\n\n", filepath.Base(os.Args[0]))
	}

	// Check for serve command
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		os.Args = append(os.Args[:1], os.Args[2:]...)
		flag.Parse()
		runServer()
		return
	}

	flag.Parse()

	if *noColor {
		disableColors()
	}

	if *responseBodyFile == "" && *templateFile == "" {
		printError("Error: At least -response-body or -template must be specified\n")
		flag.Usage()
		os.Exit(1)
	}

	if err := runTest(); err != nil {
		printError(fmt.Sprintf("Test failed: %v\n", err))
		os.Exit(1)
	}
}

func runTest() error {
	printHeader("Template Tester")
	fmt.Println()

	// Load response body
	var responseData map[string]any
	if *responseBodyFile != "" {
		printSection("Loading Response Body")
		data, err := loadJSONFile(*responseBodyFile)
		if err != nil {
			return fmt.Errorf("failed to load response body: %w", err)
		}
		responseData = data
		printSuccess(fmt.Sprintf("Loaded response body from: %s\n", *responseBodyFile))
		if *verbose {
			printJSON("Response Data:", responseData)
		}
	}

	// Load template
	var tmplContent string
	if *templateFile != "" {
		printSection("Loading Template")
		content, err := os.ReadFile(*templateFile)
		if err != nil {
			return fmt.Errorf("failed to load template: %w", err)
		}
		tmplContent = string(content)
		printSuccess(fmt.Sprintf("Loaded template from: %s\n", *templateFile))
		if *verbose {
			printTemplate("Template Content:", tmplContent)
		}
	}

	// Load optional args
	var args map[string]any
	if *argsFile != "" {
		printSection("Loading Arguments")
		data, err := loadJSONFile(*argsFile)
		if err != nil {
			return fmt.Errorf("failed to load args: %w", err)
		}
		args = data
		printSuccess(fmt.Sprintf("Loaded args from: %s\n", *argsFile))
		if *verbose {
			printJSON("Args Data:", args)
		}
	}

	// Load optional config
	var config map[string]string
	if *configFile != "" {
		printSection("Loading Config")
		data, err := loadJSONFile(*configFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		config = make(map[string]string)
		for k, v := range data {
			config[k] = fmt.Sprintf("%v", v)
		}
		printSuccess(fmt.Sprintf("Loaded config from: %s\n", *configFile))
		if *verbose {
			printJSON("Config Data:", config)
		}
	}

	// Prepare template context
	printSection("Preparing Template Context")
	ctx := template.NewContext()

	if responseData != nil {
		// Preprocess response data (same as internal/core/handler.go)
		responseData = preprocessResponseData(responseData)
		ctx.Response.Data = responseData

		// Also store raw JSON as string in Response.Body
		bodyBytes, _ := json.Marshal(responseData)
		ctx.Response.Body = string(bodyBytes)
	}

	if args != nil {
		ctx.Args = args
	}

	if config != nil {
		ctx.Config = config
	}

	if *verbose {
		printInfo("Context prepared with:\n")
		fmt.Printf("  - Response.Data: %v\n", ctx.Response.Data != nil)
		fmt.Printf("  - Response.Body: %v\n", ctx.Response.Body != nil && ctx.Response.Body != "")
		fmt.Printf("  - Args: %d items\n", len(ctx.Args))
		fmt.Printf("  - Config: %d items\n", len(ctx.Config))
		fmt.Println()
	}

	// Render template
	printSection("Rendering Template")

	renderer := template.NewRenderer()
	result, err := renderer.Render(tmplContent, ctx)
	if err != nil {
		printError(fmt.Sprintf("Template rendering failed:\n"))
		printError(fmt.Sprintf("  Error: %v\n\n", err))

		// Provide debugging hints
		printInfo("Debugging hints:\n")
		fmt.Println("  1. Check template syntax - Go templates use {{.Field}} notation")
		fmt.Println("  2. Verify field paths - use .Response.Data.fieldName")
		fmt.Println("  3. Available template functions:")
		fmt.Println("     - env(name)           - Get environment variable")
		fmt.Println("     - add(a, b)           - Add two integers")
		fmt.Println("     - fromJSON(s)         - Parse JSON string")
		fmt.Println("     - toJSON(v)           - Convert to JSON string")
		fmt.Println("     - safeGet(path, data) - Safe nested field access")
		fmt.Println("     - safeGetOr(path, data, default) - Safe access with default")
		fmt.Println()

		return err
	}

	printSuccess("Template rendered successfully!\n\n")

	printSection("Rendered Output")
	printOutput(result)

	// Try to pretty-print if it's valid JSON
	if *verbose {
		var jsonData any
		if err := json.Unmarshal([]byte(result), &jsonData); err == nil {
			fmt.Println()
			printSection("Formatted JSON Output")
			printJSON("", jsonData)
		}
	}

	return nil
}

func loadJSONFile(path string) (map[string]any, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var data map[string]any
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, err
	}

	return data, nil
}

// preprocessResponseData converts []any to []map[string]any for better template access
// This is the same logic as in internal/core/handler.go
func preprocessResponseData(data map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range data {
		result[k] = preprocessValue(v)
	}
	return result
}

func preprocessValue(v any) any {
	switch val := v.(type) {
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = preprocessValue(item)
		}
		return result
	case map[string]any:
		return preprocessResponseData(val)
	default:
		return v
	}
}

// Print helper functions
func disableColors() {
	colorReset = ""
	colorRed = ""
	colorGreen = ""
	colorYellow = ""
	colorBlue = ""
	colorPurple = ""
	colorCyan = ""
}

func printHeader(text string) {
	fmt.Printf("%s=== %s ===%s\n", colorCyan, text, colorReset)
}

func printSection(text string) {
	fmt.Printf("%s--- %s ---%s\n", colorBlue, text, colorReset)
}

func printSuccess(text string) {
	fmt.Printf("%s✓ %s%s", colorGreen, text, colorReset)
}

func printError(text string) {
	fmt.Fprintf(os.Stderr, "%s✗ %s%s", colorRed, text, colorReset)
}

func printInfo(text string) {
	fmt.Printf("%s%s%s", colorYellow, text, colorReset)
}

func printTemplate(label string, content string) {
	if label != "" {
		fmt.Printf("%s%s%s\n", colorPurple, label, colorReset)
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		fmt.Printf("%s%4d | %s%s\n", colorPurple, i+1, colorReset, line)
	}
	fmt.Println()
}

func printOutput(content string) {
	fmt.Printf("%s", content)
	if !strings.HasSuffix(content, "\n") {
		fmt.Println()
	}
}

func printJSON(label string, data any) {
	if label != "" {
		fmt.Printf("%s%s%s\n", colorPurple, label, colorReset)
	}
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("(failed to marshal: %v)\n", err)
		return
	}
	fmt.Println(string(bytes))
	fmt.Println()
}

// Server functions
func runServer() {
	http.HandleFunc("/", handlePlayground)
	http.HandleFunc("/api/render", handleRender)
	http.HandleFunc("/api/fix", handleFix)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Starting template playground server on http://localhost%s\n", addr)
	log.Printf("Open your browser and visit: http://localhost%s\n", addr)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start server: %v\n", err)
	}
}

func handlePlayground(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	content, err := playgroundHTML.ReadFile("playground.html")
	if err != nil {
		http.Error(w, "Failed to load playground", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(content)
}

func handleRender(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RenderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.Template == "" {
		sendError(w, "Template is required", http.StatusBadRequest)
		return
	}

	ctx := template.NewContext()

	if req.ResponseBody != nil {
		responseData := preprocessResponseData(req.ResponseBody)
		ctx.Response.Data = responseData

		bodyBytes, _ := json.Marshal(responseData)
		ctx.Response.Body = string(bodyBytes)
	}

	if req.Args != nil {
		ctx.Args = req.Args
	}

	if req.Config != nil {
		ctx.Config = req.Config
	}

	renderer := template.NewRenderer()
	result, err := renderer.Render(req.Template, ctx)
	if err != nil {
		sendError(w, fmt.Sprintf("Template rendering failed: %v", err), http.StatusBadRequest)
		return
	}

	response := RenderResponse{
		Result: result,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleFix(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	apiKey := os.Getenv("ARK_API_KEY")
	if apiKey == "" {
		sendError(w, "ARK_API_KEY environment variable is not set", http.StatusInternalServerError)
		return
	}

	var req FixRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Loop to try to fix the template (max 3 retries)
	fixedTemplate, result, err := tryFix(req, apiKey)
	if err != nil {
		sendError(w, fmt.Sprintf("Failed to fix template: %v", err), http.StatusInternalServerError)
		return
	}

	response := FixResponse{
		FixedTemplate: fixedTemplate,
		Result:        result,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func tryFix(req FixRequest, apiKey string) (string, string, error) {
	currentTemplate := req.Template
	currentError := req.Error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		log.Printf("Attempt %d/3 to fix template via AI...\n", i+1)

		newTemplate, err := fixWithAI(currentTemplate, currentError, req.ResponseBody, apiKey)
		if err != nil {
			return "", "", fmt.Errorf("AI request failed: %w", err)
		}

		// Verify the new template
		ctx := template.NewContext()
		if req.ResponseBody != nil {
			responseData := preprocessResponseData(req.ResponseBody)
			ctx.Response.Data = responseData
			bodyBytes, _ := json.Marshal(responseData)
			ctx.Response.Body = string(bodyBytes)
		}
		if req.Args != nil {
			ctx.Args = req.Args
		}
		if req.Config != nil {
			ctx.Config = req.Config
		}

		renderer := template.NewRenderer()
		result, renderErr := renderer.Render(newTemplate, ctx)

		if renderErr == nil {
			// Success!
			return newTemplate, result, nil
		}

		// Failed, update for next iteration
		currentTemplate = newTemplate
		currentError = renderErr.Error()
		log.Printf("Generated template failed validation: %v", renderErr)
	}

	return "", "", fmt.Errorf("failed to fix template after %d attempts. Last error: %s", maxRetries, currentError)
}

func fixWithAI(tmpl string, errStr string, data any, apiKey string) (string, error) {
	url := "https://ark.cn-beijing.volces.com/api/v3/chat/completions"

	dataBytes, _ := json.Marshal(data)
	dataStr := string(dataBytes)
	if len(dataStr) > 2000 {
		dataStr = dataStr[:2000] + "...(truncated)"
	}

	systemPrompt := `You are a Go template (text/template) expert. 
Your goal is to fix a Go template that failed to render.
The environment supports standard Go template syntax and Sprig functions (e.g., fromJSON, toJSON, safeGet, kindIs).
The data context usually has structure like .Response.Data.
IMPORTANT: Return ONLY the fixed template code. Do not include markdown formatting or explanations.`

	userPrompt := fmt.Sprintf("Here is the data (JSON):\n%s\n\nHere is the broken template:\n%s\n\nHere is the error message:\n%s\n\nPlease provide the corrected template code.", dataStr, tmpl, errStr)

	reqBody := map[string]any{
		"model": "doubao-seed-1-6-251015",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var respData struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return "", err
	}

	if len(respData.Choices) == 0 {
		return "", errors.New("no choices returned from AI")
	}

	return extractCodeBlock(respData.Choices[0].Message.Content), nil
}

func extractCodeBlock(content string) string {
	// If the content is wrapped in ``` ... ```, extract it
	re := regexp.MustCompile("(?s)```(?:go|text|template)?\\s*(.*?)\\s*```")
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	// Otherwise return the whole content stripped of whitespace
	return strings.TrimSpace(content)
}

func sendError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(RenderResponse{
		Error: message,
	})
}
