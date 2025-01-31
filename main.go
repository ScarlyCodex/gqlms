package main

import (
        "bufio"
        "bytes"
        "encoding/json"
        "flag"
        "fmt"
        "io"
        "net/http"
        "os"
        "strings"
        "time"

        "github.com/fatih/color"
)

// Tipo explícito para mutaciones y argumentos
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

// Respuesta de introspección
type IntrospectionResponse struct {
        Data struct {
                Schema struct {
                        MutationType struct {
                                Fields []Mutation `json:"fields"`
                        } `json:"mutationType"`
                } `json:"__schema"`
        } `json:"data"`
}

// Mensajes de error de acceso denegado
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

// ✅ Banner
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

// ✅ Función principal
func main() {
        printBanner()

        // Flags
        requestFile := flag.String("r", "", "Path to the HTTP request file (e.g., request.txt)")
        delay := flag.Int("t", 1, "Time (in seconds) between each request")
        flag.Parse()

        if *requestFile == "" {
                fmt.Println("Error: You must provide a path to the request file using -r")
                os.Exit(1)
        }

        endpoint, headers, baseRequestBody, err := parseRequestFile(*requestFile)
        if err != nil {
                fmt.Printf("Error reading request file: %v\n", err)
                os.Exit(1)
        }

        if _, exists := headers["Authorization"]; !exists {
                fmt.Println("[!] Warning: No Authorization header found. Mutations may fail.")
        }

        mutations := getMutations(endpoint, headers)
        testMutations(mutations, endpoint, headers, *delay, baseRequestBody)
}

// ✅ Obtener mutaciones junto con sus argumentos requeridos
func getMutations(endpoint string, headers map[string]string) []Mutation {
        query := map[string]string{
                "query": `{ __schema { mutationType { fields { name args { name type { name kind ofType { name } } } } } } }`,
        }
        queryBytes, _ := json.Marshal(query)

        resp, err := sendRequest(endpoint, headers, queryBytes)
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

// 🔥 Testear cada mutación con la sintaxis correcta
func testMutations(mutations []Mutation, endpoint string, headers map[string]string, delay int, baseRequestBody string) {
        allowedFile, _ := os.Create("allowedMutations.txt")
        defer allowedFile.Close()
        allowedWriter := bufio.NewWriter(allowedFile)

        unallowedFile, _ := os.Create("unallowedMutations.txt")
        defer unallowedFile.Close()
        unallowedWriter := bufio.NewWriter(unallowedFile)

        for i, mutation := range mutations {
                fmt.Printf("\r🔄 Testing mutation: %s (%d/%d)", mutation.Name, i+1, len(mutations))

                payload := buildMutationPayload(mutation, endpoint, headers, baseRequestBody)
                resp, err := sendRequest(endpoint, headers, payload)
                if err != nil || containsDeniedMessage(resp) {
                        unallowedWriter.WriteString(mutation.Name + "\n")
                        unallowedWriter.Flush()
                        continue
                }

                allowedWriter.WriteString(mutation.Name + "\n")
                allowedWriter.Flush()
                time.Sleep(time.Duration(delay) * time.Second)
        }

        fmt.Println("\n✅ Authorization testing completed!")
}

// ✅ Construir payload dinámicamente asegurando la correcta sintaxis
func buildMutationPayload(mutation Mutation, endpoint string, headers map[string]string, baseRequestBody string) []byte {
        inputType := mutation.Name + "Input"
        inputFields := map[string]interface{}{"testField": "test_value"} // Simulación de valores por defecto

        // Construcción de la query
        query := fmt.Sprintf(
                "mutation %s($input: %s!) { %s(input: $input) { __typename } }",
                mutation.Name,
                inputType,
                mutation.Name,
        )

        // Construcción del JSON
        payload := map[string]interface{}{
                "operationName": mutation.Name,
                "query":         query,
                "variables":     map[string]interface{}{"input": inputFields},
        }

        payloadBytes, _ := json.Marshal(payload)
        return payloadBytes
}

// ✅ Detectar si la respuesta contiene mensajes de acceso denegado
func containsDeniedMessage(resp *http.Response) bool {
        if resp.StatusCode == 403 || resp.StatusCode == 401 {
                return true
        }

        responseBody, _ := io.ReadAll(resp.Body)
        resp.Body = io.NopCloser(bytes.NewBuffer(responseBody)) // Restaurar el body
        return strings.Contains(strings.ToLower(string(responseBody)), "unauthorized")
}

// ✅ Enviar request HTTP
func sendRequest(endpoint string, headers map[string]string, payload []byte) (*http.Response, error) {
        client := &http.Client{}
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
        resp.Body = io.NopCloser(bytes.NewBuffer(responseBody)) // Restaurar el body
        return resp, nil
}

// ✅ Leer y procesar el archivo `request.txt`
func parseRequestFile(filePath string) (string, map[string]string, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", nil, "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	headers := make(map[string]string)
	var endpoint string
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

		// Extraer el endpoint de la línea "POST /api/graphql HTTP/2"
		if strings.HasPrefix(line, "POST") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				endpoint = parts[1]
			}
		} else if strings.Contains(line, ": ") { // 🔥 Extraer headers
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) == 2 {
				headers[parts[0]] = parts[1]
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", nil, "", err
	}

	// ✅ Si el endpoint no tiene `http://` o `https://`, detectar el protocolo
        if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
        if host, exists := headers["Host"]; exists {
            // Detectar si el puerto sugiere HTTP o HTTPS
            if strings.Contains(host, ":443") || strings.HasSuffix(host, "https") {
                endpoint = "https://" + host + endpoint
            } else {
                endpoint = "http://" + host + endpoint
            }
        } else {
            return "", nil, "", fmt.Errorf("invalid endpoint: %s, no Host header found", endpoint)
        }
    }
    

	// **Si el endpoint sigue vacío, lanzar error claro**
	if endpoint == "" {
		return "", nil, "", fmt.Errorf("❌ No valid endpoint extracted from request.txt")
	}

	return endpoint, headers, body.String(), nil
}

