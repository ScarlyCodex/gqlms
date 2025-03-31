package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
)

// Tipos
type Mutation struct {
	Name string `json:"name"`
	Args []struct {
		Name string `json:"name"`
		Type struct {
			Name   string `json:"name"`
			Kind   string `json:"kind"`
			OfType *struct {
				Name string `json:"name"`
			} `json:"ofType"`
		} `json:"type"`
	} `json:"args"`
}

type IntrospectionResponse struct {
	Data struct {
		Schema struct {
			MutationType struct {
				Fields []Mutation `json:"fields"`
			} `json:"mutationType"`
		} `json:"__schema"`
	} `json:"data"`
}

var deniedMessages = []string{
	"Access denied for this resource",
	"Unauthorized Access",
	"UNAUTHORIZED ACCESS",
	"You do not have permission to access this mutation",
	"Forbidden resource",
	"User is not authorized to perform this action",
	"Permission denied",
	"Unauthorized",
	"UNAUTHORIZED",
}

// ProxyFlag permite usar --proxy [opcional: url]
type ProxyFlag struct {
	Enabled bool
	URL     string
}

func (p *ProxyFlag) String() string {
	if p.Enabled {
		return p.URL
	}
	return ""
}

func (p *ProxyFlag) Set(value string) error {
	p.Enabled = true
	if value == "" || strings.HasPrefix(value, "-") {
		p.URL = "http://127.0.0.1:8080"
	} else {
		p.URL = value
	}
	return nil
}

// Banner
func printBanner() {
	color.Magenta(`
 #####   #####  #                     
#     # #     # #       #    #  ####  
#       #     # #       ##  ## #      
#  #### #     # #       # ## #  ####  
#     # #   # # #       #    #      # 
#     # #    #  #       #    # #    # 
 #####   #### # ####### #    #  ####  
                                      
 GraphQL Mutation Authorization Tester
`)
}

// Main
func main() {
	printBanner()

	requestFile := flag.String("r", "", "Path to the HTTP request file (e.g., request.txt)")
	delay := flag.Int("t", 1, "Time (in seconds) between each request")
	useSSL := flag.Bool("ssl", true, "Use HTTPS (default: true). Use -ssl=false to disable SSL resolution")
	unauthHeaders := flag.String("unauth", "", "Comma-separated list of authentication-related headers to remove after introspection")

	var proxy ProxyFlag
	flag.Var(&proxy, "proxy", "Use proxy. Use -proxy= (default: http://127.0.0.1:8080) or -proxy=http://custom:port")

	flag.Parse()

	if *requestFile == "" {
		fmt.Println("Error: You must provide a path to the request file using -r")
		os.Exit(1)
	}

	endpoint, headers, baseRequestBody, err := parseRequestFile(*requestFile, *useSSL)
	if err != nil {
		fmt.Printf("Error reading request file: %v\n", err)
		os.Exit(1)
	}

	if _, exists := headers["Authorization"]; !exists {
		fmt.Println("\n → Unauthenticated mode")
	} else {
		fmt.Println(" → Authenticated mode")
	}

	if proxy.Enabled {
		fmt.Printf(" → Using proxy\n")
	} else {
		fmt.Println(" → No proxy in use")
	}

	// Parse unauth headers if set
	var unauthList []string
	if *unauthHeaders != "" {
		unauthList = strings.Split(*unauthHeaders, ",")
		for i := range unauthList {
			unauthList[i] = strings.TrimSpace(unauthList[i])
		}

		// Warn if headers not found
		missing := []string{}
		for _, h := range unauthList {
			if _, ok := headers[h]; !ok {
				missing = append(missing, h)
			}
		}

		if len(missing) > 0 {
			fmt.Printf("⚠️  The following headers were not found in the request: %v\n", missing)
			fmt.Print("Do you want to continue anyway? [Y/n]: ")
			var response string
			fmt.Scanln(&response)

			switch strings.ToLower(response) {
			case "y":
				// continue
			case "n":
				fmt.Println("🛑 Aborting.")
				os.Exit(0)
			default:
				fmt.Println("❌ Invalid input. Please use Y or n.")
				os.Exit(1)
			}
		}
	}

	// Step 1: get schema with auth
	mutations := getMutations(endpoint, headers, proxy.URL, proxy.Enabled)

	// Step 2: remove auth headers for unauth testing
	if len(unauthList) > 0 {
		fmt.Println("🔓 Switching to unauthenticated mode. Removing headers:", unauthList)
		for _, h := range unauthList {
			delete(headers, h)
		}
	}

	// Step 3: test mutations unauthenticated
	testMutations(mutations, endpoint, headers, *delay, baseRequestBody, proxy.URL, proxy.Enabled)
}

// Get mutations from introspection
func getMutations(endpoint string, headers map[string]string, proxy string, useProxy bool) []Mutation {
	query := map[string]string{
		"query": `{ __schema { mutationType { fields { name args { name type { name kind ofType { name } } } } } } }`,
	}
	queryBytes, _ := json.Marshal(query)

	resp, err := sendRequest(endpoint, headers, queryBytes, proxy, useProxy)
	if err != nil {
		fmt.Printf("❌ Error fetching mutations: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)

	var introspectionResp IntrospectionResponse
	json.Unmarshal(responseBody, &introspectionResp)

	file, _ := os.Create("allMutations.txt")
	defer file.Close()
	writer := bufio.NewWriter(file)

	mutations := introspectionResp.Data.Schema.MutationType.Fields
	for _, field := range mutations {
		writer.WriteString(field.Name + "\n")
	}
	writer.Flush()

	fmt.Println("\n✅ Successfully fetched mutations. Now testing authorization...\n")
	return mutations
}

// Test mutations
func testMutations(mutations []Mutation, endpoint string, headers map[string]string, delay int, baseRequestBody string, proxy string, useProxy bool) {
	allowedCount := 0
	unallowedCount := 0
	allowedFile, _ := os.Create("allowedMutations.txt")
	defer allowedFile.Close()
	allowedWriter := bufio.NewWriter(allowedFile)

	unallowedFile, _ := os.Create("unallowedMutations.txt")
	defer unallowedFile.Close()
	unallowedWriter := bufio.NewWriter(unallowedFile)

	bar := progressbar.NewOptions(len(mutations),
	progressbar.OptionSetDescription("🔄 Testing mutations"),
	progressbar.OptionSetTheme(progressbar.Theme{
		Saucer:        "🟩", // Filled part
		SaucerHead:    "🟢",
		SaucerPadding: "⬜", // Empty part
		BarStart:      "|",
		BarEnd:        "|",
	}),
	progressbar.OptionSetWidth(40),
	progressbar.OptionShowCount(),
	progressbar.OptionSetPredictTime(true),
	progressbar.OptionSetElapsedTime(true),
)

	for _, mutation := range mutations {
		//fmt.Printf("\n🧪 Mutation: %s\n", mutation.Name)
		bar.Describe(fmt.Sprintf("🔄 %s", mutation.Name))

		payload := buildMutationPayload(mutation, endpoint, headers, baseRequestBody)
		resp, err := sendRequest(endpoint, headers, payload, proxy, useProxy)

		if err != nil || containsDeniedMessage(resp) {
			unallowedWriter.WriteString(mutation.Name + "\n")
			unallowedWriter.Flush()
			unallowedCount++
		} else {
			allowedWriter.WriteString(mutation.Name + "\n")
			allowedWriter.Flush()
			allowedCount++
		}

		bar.Add(1)
		time.Sleep(time.Duration(delay) * time.Second)
	}

	fmt.Println("\n✅ Authorization testing completed!")
	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("  ✅ Allowed:     %d\n", allowedCount)
	fmt.Printf("  ❌ Unauthorized: %d\n", unallowedCount)
	fmt.Printf("  📦 Total tested: %d\n\n", allowedCount+unallowedCount)
}

// Build GraphQL mutation payload
func buildMutationPayload(mutation Mutation, endpoint string, headers map[string]string, baseRequestBody string) []byte {
	inputType := mutation.Name + "Input"
	inputFields := map[string]interface{}{"testField": "test_value"} // Placeholder input

	query := fmt.Sprintf(
		"mutation %s($input: %s!) { %s(input: $input) { __typename } }",
		mutation.Name,
		inputType,
		mutation.Name,
	)

	payload := map[string]interface{}{
		"operationName": mutation.Name,
		"query":         query,
		"variables":     map[string]interface{}{"input": inputFields},
	}

	payloadBytes, _ := json.Marshal(payload)
	return payloadBytes
}

// Detect if response is unauthorized
func containsDeniedMessage(resp *http.Response) bool {
	if resp.StatusCode == 403 || resp.StatusCode == 401 {
		return true
	}

	responseBody, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))
	return strings.Contains(strings.ToLower(string(responseBody)), "unauthorized")
}

// Send HTTP request with optional proxy
func sendRequest(endpoint string, headers map[string]string, payload []byte, proxy string, useProxy bool) (*http.Response, error) {
	var transport *http.Transport

	if useProxy {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("❌ Invalid proxy URL: %v", err)
		}
		transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	} else {
		transport = &http.Transport{}
	}

	client := &http.Client{
		Transport: transport,
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	responseBody, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))
	return resp, nil
}

// Parse request.http file
func parseRequestFile(filePath string, useSSL bool) (string, map[string]string, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", nil, "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	headers := make(map[string]string)
	var endpointPath string
	var body strings.Builder
	isBody := false

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			isBody = true
			continue
		}

		if isBody {
			body.WriteString(line)
			continue
		}

		if strings.HasPrefix(line, "POST") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				endpointPath = parts[1]
			}
		} else if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				headers[http.CanonicalHeaderKey(key)] = val
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", nil, "", err
	}

	if endpointPath == "" {
		return "", nil, "", fmt.Errorf("❌ No endpoint path found in request file")
	}

	// Construir endpoint completo
	if !strings.HasPrefix(endpointPath, "http://") && !strings.HasPrefix(endpointPath, "https://") {
		scheme := "https"
		if !useSSL {
			scheme = "http"
		}
		host, exists := headers["Host"]
		if !exists {
			return "", nil, "", fmt.Errorf("❌ No Host header found, can't determine endpoint")
		}
		endpointPath = fmt.Sprintf("%s://%s%s", scheme, host, endpointPath)
	}

	return endpointPath, headers, body.String(), nil
}


