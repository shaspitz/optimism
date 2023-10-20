package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

type etherscanContract struct {
	Name             string
	DeployedAddress  string
	PredeployAddress string
	Abi              string
	Bytecode         string
}

type etherscanApiResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  string `json:"result"`
}

type etherscanRpcApiResponse struct {
	JsonRpc string `json:"jsonrpc"`
	Id      int    `json:"id"`
	Result  string `json:"result"`
}

type etherscanContractMetadata struct {
	Name        string
	DeployedBin string
	Package     string
}

const (
	etherscanGetAbiURLFormat      = "https://api.etherscan.io/api?module=contract&action=getabi&address=%s&apikey=%s"
	etherscanGetBytecodeURLFormat = "https://api.etherscan.io/api?module=proxy&action=eth_getCode&address=%s&tag=latest&apikey=%s"
)

// readEtherscanContractsList reads a JSON file specified by the given file path and
// parses it into a slice of `etherscanContract`.
//
// Parameters:
// - filePath: The path to the JSON file containing the list of etherscan contracts.
//
// Returns:
// - A slice of etherscanContract parsed from the JSON file.
// - An error if reading the file or parsing the JSON failed.
func readEtherscanContractsList(filePath string) ([]etherscanContract, error) {
	var data contractsData
	err := readJSONFile(filePath, &data)
	return data.Etherscan, err
}

// fetchEtherscanData sends an HTTP GET request to the provided URL and
// retrieves the response body. The function returns the body as a byte slice.
//
// Parameters:
//   - url: The target URL for the HTTP GET request.
//
// Returns:
//   - A byte slice containing the response body.
//   - An error if there was an issue with the HTTP request or reading the response.
//
// Note:
//
//	The caller is responsible for interpreting the returned data, including
//	unmarshaling it if it represents structured data (e.g., JSON).
func fetchEtherscanData(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// fetchEtherscanAbi sends an HTTP GET request to the provided Etherscan API URL
// to retrieve the ABI of a contract.
// In the event of a rate limit error from the Etherscan API ("Max rate limit reached"),
// the function will retry the request based on the provided retry parameters.
//
// Parameters:
//   - url: The target Etherscan API URL for fetching the ABI.
//   - apiMaxRetries: The maximum number of times to retry the request in case of a rate limit error.
//   - apiRetryDelay: The delay (in seconds) between retries when a rate limit error is encountered.
//
// Returns:
//   - A string containing the ABI of the smart contract.
//   - An error if there was an issue with the HTTP request, unmarshaling the response,
//     or if the maximum number of retries is exceeded.
//
// Note:
//
//	The function is designed specifically for interacting with the Etherscan API,
//	and expects the response to be in a specific format (`etherscanApiResponse`).
func fetchEtherscanAbi(url string, apiMaxRetries, apiRetryDelay int) (string, error) {
	var maxRetries = apiMaxRetries
	var retryDelay = time.Duration(apiRetryDelay) * time.Second

	for retries := 0; retries < maxRetries; retries++ {
		body, err := fetchEtherscanData(url)
		if err != nil {
			return "", err
		}

		var apiResponse etherscanApiResponse
		err = json.Unmarshal(body, &apiResponse)
		if err != nil {
			log.Printf("Failed to unmarshal as etherscanApiResponse: %v", err)
			return "", err
		}

		if apiResponse.Message == "NOTOK" && apiResponse.Result == "Max rate limit reached" {
			log.Printf("Reached API rate limit, waiting %v and trying again", retryDelay)
			time.Sleep(retryDelay)
			continue
		}

		if apiResponse.Message != "OK" {
			return "", fmt.Errorf("There was an issue with the Etherscan request to %s, received response: %v", url, apiResponse)
		}

		return apiResponse.Result, nil
	}

	return "", fmt.Errorf("Failed to fetch ABI after %d retries", maxRetries)
}

// fetchEtherscanBytecode sends an HTTP GET request to the provided Etherscan API URL
// to retrieve the bytecode of a smart contract.
//
// Parameters:
//   - url: The target Etherscan API URL for fetching the bytecode.
//
// Returns:
//   - A string containing the bytecode of the smart contract.
//   - An error if there was an issue with the HTTP request, unmarshaling the response,
//     or if the response does not match the expected format.
//
// Note:
//
//	The function is specifically designed for interacting with the Etherscan API and
//	expects the response to be in a specific RPC format (`etherscanRpcApiResponse`).
func fetchEtherscanBytecode(url string) (string, error) {
	body, err := fetchEtherscanData(url)
	if err != nil {
		return "", err
	}

	var rpcResponse etherscanRpcApiResponse
	err = json.Unmarshal(body, &rpcResponse)
	if err != nil {
		log.Printf("Failed to unmarshal as etherscanRpcApiResponse: %v", err)
		return "", err
	}

	return rpcResponse.Result, nil
}

// writeEtherscanContractMetadata writes the provided `etherscanContractMetadata`
// to a file using the provided `fileTemplate`.
// The file is named after the contract (with the contract name transformed to lowercase),
// and has the "_more.go" suffix.
//
// Parameters:
// - contractMetaData: An instance of `etherscanContractMetadata` containing metadata details of the contract.
// - metadataOutputDir: The directory where the metadata file should be saved.
// - contractName: The name of the contract for which the metadata is being written.
// - fileTemplate: A pointer to a `template.Template` used to format and write the metadata to the file.
//
// Returns:
// - An error if there's an issue opening the metadata file, executing the template, or writing to the file.
func writeEtherscanContractMetadata(contractMetaData etherscanContractMetadata, metadataOutputDir, contractName string, fileTemplate *template.Template) error {
	metaDataFilePath := filepath.Join(metadataOutputDir, strings.ToLower(contractName)+"_more.go")
	metadataFile, err := os.OpenFile(
		metaDataFilePath,
		os.O_RDWR|os.O_CREATE|os.O_TRUNC,
		0o600,
	)
	defer metadataFile.Close()
	if err != nil {
		return fmt.Errorf("Error opening %s's metadata file at %s: %v", contractName, metaDataFilePath, err)
	}

	if err := fileTemplate.Execute(metadataFile, contractMetaData); err != nil {
		return fmt.Errorf("Error writing %s's contract metadata at %s: %v", contractName, metaDataFilePath, err)
	}

	log.Printf("Wrote %s's contract metadata to: %s", contractName, metaDataFilePath)
	return nil
}

// genEtherscanBindings generates Go bindings for Ethereum smart contracts based on the ABI and bytecode
// fetched from Etherscan.
// The function reads the list of contracts from the provided file path and fetches the ABI and
// bytecode for each contract from Etherscan using the provided API key. It then generates Go bindings
// for each contract and writes metadata for each contract to the specified output directory.
//
// Parameters:
// - contractListFilePath: Path to the file containing the list of contracts.
// - sourceMapsListStr: Comma-separated list of source maps.
// - etherscanApiKey: API key to fetch data from Etherscan.
// - goPackageName: Name of the Go package for the generated bindings.
// - metadataOutputDir: Directory to output the generated contract metadata.
//
// Returns:
//   - An error if there are issues reading the contract list, fetching data from Etherscan, generating
//     contract bindings, or writing contract metadata.
func genEtherscanBindings(contractListFilePath, sourceMapsListStr, etherscanApiKey, goPackageName, metadataOutputDir string, apiMaxRetries, apiRetryDelay int) error {
	contracts, err := readEtherscanContractsList(contractListFilePath)
	if err != nil {
		return fmt.Errorf("Error reading contract list %s: %v", contractListFilePath, err)
	}

	if len(contracts) == 0 {
		return fmt.Errorf("No contracts parsable from given contract list: %s", contractListFilePath)
	}

	tempArtifactsDir, err := mkTempArtifactsDir()
	defer func() {
		err := os.RemoveAll(tempArtifactsDir)
		if err != nil {
			log.Printf("Error removing temporary directory %s: %v", tempArtifactsDir, err)
		} else {
			log.Printf("Successfully removed temporary directory")
		}
	}()

	contractMetadataFileTemplate := template.Must(template.New("etherscanContractMetadata").Parse(etherscanContractMetadataTemplate))

	sourceMapsList := strings.Split(sourceMapsListStr, ",")
	sourceMapsSet := make(map[string]struct{})
	for _, k := range sourceMapsList {
		sourceMapsSet[k] = struct{}{}
	}

	for _, contract := range contracts {
		log.Printf("Generating bindings and metadata for Etherscan contract: %s", contract.Name)

		contract.Abi, err = fetchEtherscanAbi(fmt.Sprintf(etherscanGetAbiURLFormat, contract.DeployedAddress, etherscanApiKey), apiMaxRetries, apiRetryDelay)
		if err != nil {
			return err
		}
		contract.Bytecode, err = fetchEtherscanBytecode(fmt.Sprintf(etherscanGetBytecodeURLFormat, contract.DeployedAddress, etherscanApiKey))
		if err != nil {
			return err
		}

		abiFilePath, bytecodeFilePath, err := writeContractArtifacts(tempArtifactsDir, contract.Name, []byte(contract.Abi), []byte(contract.Bytecode))
		if err != nil {
			return err
		}

		err = genContractBindings(abiFilePath, bytecodeFilePath, goPackageName, contract.Name)
		if err != nil {
			return err
		}

		contractMetaData := etherscanContractMetadata{
			Name:        contract.Name,
			DeployedBin: contract.Bytecode,
			Package:     goPackageName,
		}

		if err := writeEtherscanContractMetadata(contractMetaData, metadataOutputDir, contract.Name, contractMetadataFileTemplate); err != nil {
			return err
		}
	}

	return nil
}

// etherscanContractMetadataTemplate is a Go text template for generating the metadata
// associated with a Etherscan Ethereum contract. This template is used to produce
// Go code containing necessary a constant and initialization logic for the contract's
// deployed bytecode.
//
// The template expects to be provided with:
// - .Package: the name of the Go package.
// - .Name: the name of the contract.
// - .DeployedBin: the binary (hex-encoded) of the deployed contract.
var etherscanContractMetadataTemplate = `// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package {{.Package}}

var {{.Name}}DeployedBin = "{{.DeployedBin}}"
func init() {
	deployedBytecodes["{{.Name}}"] = {{.Name}}DeployedBin
}
`
