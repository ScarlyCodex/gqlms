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
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
)

// GraphQLType es un tipo genérico para la información de introspección.
type GraphQLType struct {
	Kind   string       `json:"kind"`
	Name   string       `json:"name"`
	OfType *GraphQLType `json:"ofType,omitempty"`
}

// Mutation representa una mutación GraphQL.
type Mutation struct {
	Name string `json:"name"`
	Args []struct {
		Name string      `json:"name"`
		Type GraphQLType `json:"type"`
	} `json:"args"`
}

// IntrospectionResponse representa la respuesta de la introspección para las mutaciones.
type IntrospectionResponse struct {
	Data struct {
		Schema struct {
			MutationType struct {
				Fields []Mutation `json:"fields"`
			} `json:"mutationType"`
		} `json:"__schema"`
	} `json:"data"`
}

// InputField representa un campo de un input object.
type InputField struct {
	Name string      `json:"name"`
	Type GraphQLType `json:"type"`
}

// IntrospectionInputResponse representa la respuesta de la introspección de un input type.
type IntrospectionInputResponse struct {
	Data struct {
		Type struct {
			Name        string       `json:"name"`
			InputFields []InputField `json:"inputFields"`
		} `json:"__type"`
	} `json:"data"`
}

// GraphQLResponse es la estructura para parsear respuestas de GraphQL.
type GraphQLResponse struct {
	Data   interface{} `json:"data"`
	Errors []struct {
		Message    string `json:"message"`
		Extensions struct {
			Code string `json:"code"`
		} `json:"extensions"`
	} `json:"errors"`
}

var (
	// Expresiones regulares para detectar mensajes de error de autorización.
	unauthorizedRegexes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)unauthorized`),
		regexp.MustCompile(`(?i)forbidden`),
		regexp.MustCompile(`(?i)access denied`),
		regexp.MustCompile(`(?i)restricted`),
	}

	// Mensajes predefinidos (por si no se parsea el JSON correctamente)
	deniedMessages = []string{
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

	// Flag verbose para registro detallado.
	verbose bool
)

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
	astraMagenta := color.RGB(148, 68, 180)
	astraMagenta.Println(`
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
	verboseFlag := flag.Bool("v", false, "Enable verbose logging of responses for analysis")

	var proxy ProxyFlag
	flag.Var(&proxy, "proxy", "Use proxy. Use -proxy= (default: http://127.0.0.1:8080) or -proxy=http://custom:port")
	flag.Parse()
	verbose = *verboseFlag

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
			fmt.Printf("\n⚠️ The following headers were not found in the request: %v\n", missing)
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

	// Step 1: obtener el schema autenticado
	mutations := getMutations(endpoint, headers, proxy.URL, proxy.Enabled)

	// Step 2: remover headers para pruebas sin auth
	if len(unauthList) > 0 {
		fmt.Println("🔓 Switching to unauthenticated mode. Removing headers:", unauthList)
		for _, h := range unauthList {
			delete(headers, h)
		}
	}

	// Step 3: probar mutaciones en modo no autenticado
	testMutations(mutations, endpoint, headers, *delay, baseRequestBody, proxy.URL, proxy.Enabled)
}

// getMutations obtiene las mutaciones vía introspección
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

func dummyValueForType(t GraphQLType) interface{} {
	if t.Kind == "NON_NULL" && t.OfType != nil {
		return dummyValueForType(*t.OfType)
	}

	if t.Kind == "LIST" && t.OfType != nil {
		element := dummyValueForType(*t.OfType)
		return []interface{}{element, element}
	}

	switch t.Kind {
	case "SCALAR":
		return dummyValueForScalar(t.Name)
	case "ENUM":
		return "ENUM_VALUE"
	case "INPUT_OBJECT":
		return map[string]interface{}{} // Rellenado dinámicamente
	default:
		if t.OfType != nil {
			return dummyValueForType(*t.OfType)
		}
		return "dummy"
	}
}



// testMutations prueba cada mutación y registra el resultado
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
			Saucer:        "🔹",
			SaucerHead:    "🔸",
			SaucerPadding: "▫️",
			BarStart:      "|",
			BarEnd:        " |",
		}),
		progressbar.OptionSetWidth(40),
		progressbar.OptionShowCount(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionSetElapsedTime(true),
	)

	for _, mutation := range mutations {
		bar.Describe(fmt.Sprintf("🔄 %s", mutation.Name))

		// Se construye el payload dinámicamente usando la información real de los argumentos
		payload := buildMutationPayload(mutation, endpoint, headers, baseRequestBody, proxy, useProxy)
		resp, err := sendRequest(endpoint, headers, payload, proxy, useProxy)

		// Si verbose está activo, se imprime la respuesta completa para análisis
		if verbose {
			bodyBytes, _ := io.ReadAll(resp.Body)
			fmt.Printf("\nResponse for mutation %s:\n%s\n", mutation.Name, string(bodyBytes))
			resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

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

// buildMutationPayload construye el payload de la mutación usando la definición real de argumentos
func buildMutationPayload(mutation Mutation, endpoint string, headers map[string]string, baseRequestBody string, proxy string, useProxy bool) []byte {
	variables := make(map[string]interface{})
	var varDefs []string
	var argsList []string

	for _, arg := range mutation.Args {
		typeName := resolveTypeName(arg.Type)

		if arg.Type.Kind == "INPUT_OBJECT" || (arg.Type.Kind == "NON_NULL" && arg.Type.OfType != nil && arg.Type.OfType.Kind == "INPUT_OBJECT") {
			inputTypeName := typeName
			inputFieldsData := getInputFields(inputTypeName, endpoint, headers, proxy, useProxy)
			dummyObj := make(map[string]interface{})

			for _, field := range inputFieldsData {
				dummyObj[field.Name] = dummyValueForType(field.Type)
			}
			variables[arg.Name] = dummyObj
		} else {
			variables[arg.Name] = dummyValueForType(arg.Type)
		}

		nonNull := ""
		if arg.Type.Kind == "NON_NULL" {
			nonNull = "!"
		}

		varDefs = append(varDefs, fmt.Sprintf("$%s: %s%s", arg.Name, typeName, nonNull))
		argsList = append(argsList, fmt.Sprintf("%s: $%s", arg.Name, arg.Name))
	}

	query := fmt.Sprintf("mutation %s(%s) { %s(%s) { __typename } }",
		mutation.Name,
		strings.Join(varDefs, ", "),
		mutation.Name,
		strings.Join(argsList, ", "),
	)

	payload := map[string]interface{}{
		"operationName": mutation.Name,
		"query":         query,
		"variables":     variables,
	}

	payloadBytes, _ := json.Marshal(payload)
	return payloadBytes
}


// resolveTypeName obtiene el nombre real de un tipo GraphQL
func resolveTypeName(t GraphQLType) string {
	if t.Name != "" {
		return t.Name
	}
	if t.OfType != nil {
		return t.OfType.Name
	}
	return ""
}

// dummyValueForScalar retorna un valor dummy según el tipo escalar
func dummyValueForScalar(typeName string) interface{} {
	switch typeName {
	case "String", "ID":
		return "gqlmsTestValue"
	case "Int":
		return 0
	case "Float":
		return 0.0
	case "Boolean":
		return false
	default:
		return "gqlmsTestValue"
	}
}

// dummyValueForInputField retorna un valor dummy para un campo de input basado en su tipo
func dummyValueForInputField(t GraphQLType) interface{} {
	var actualType string
	if t.Name != "" {
		actualType = t.Name
	} else if t.OfType != nil {
		actualType = t.OfType.Name
	}
	return dummyValueForScalar(actualType)
}

// getInputFields realiza una introspección para obtener los inputFields de un input type
func getInputFields(typeName, endpoint string, headers map[string]string, proxy string, useProxy bool) []InputField {
	queryPayload := map[string]interface{}{
		"query": `query IntrospectType($typeName: String!) {
  __type(name: $typeName) {
    name
    inputFields {
      name
      type {
        kind
        name
        ofType {
          kind
          name
        }
      }
    }
  }
}`,
		"variables": map[string]interface{}{
			"typeName": typeName,
		},
	}
	payloadBytes, _ := json.Marshal(queryPayload)
	resp, err := sendRequest(endpoint, headers, payloadBytes, proxy, useProxy)
	if err != nil {
		fmt.Printf("❌ Error introspecting type %s: %v\n", typeName, err)
		return []InputField{}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var introspectionResp IntrospectionInputResponse
	json.Unmarshal(body, &introspectionResp)
	return introspectionResp.Data.Type.InputFields
}

// containsDeniedMessage detecta si la respuesta indica acceso no autorizado utilizando múltiples heurísticas
func containsDeniedMessage(resp *http.Response) bool {
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return true
	}

	responseBody, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(responseBody))

	var gqlResp GraphQLResponse
	if json.Unmarshal(responseBody, &gqlResp) == nil {
		if len(gqlResp.Errors) > 0 {
			for _, errObj := range gqlResp.Errors {
				code := strings.ToUpper(errObj.Extensions.Code)
				if code == "UNAUTHENTICATED" || code == "FORBIDDEN" || code == "ACCESS_DENIED" {
					return true
				}
				for _, re := range unauthorizedRegexes {
					if re.MatchString(errObj.Message) {
						return true
					}
				}
			}

			if gqlResp.Data == nil {
				return true
			}
		}
	}

	lowerBody := strings.ToLower(string(responseBody))
	for _, keyword := range []string{"unauthorized", "forbidden", "access denied", "restricted"} {
		if strings.Contains(lowerBody, keyword) {
			return true
		}
	}

	if resp.StatusCode >= 500 {
		return false
	}

	if resp.StatusCode == 400 {
		return false
	}

	return false
}

// sendRequest envía una petición HTTP, opcionalmente usando proxy
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

// parseRequestFile parsea el archivo .http y extrae endpoint, headers y body
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