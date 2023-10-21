package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/ethereum-optimism/optimism/op-bindings/ast"
	"github.com/ethereum-optimism/optimism/op-bindings/foundry"
)

type localContractMetadata struct {
	Name              string
	StorageLayout     string
	DeployedBin       string
	Package           string
	DeployedSourceMap string
}

// readLocalContractList reads a JSON file specified by the given file path and
// parses it into a slice of contract names.
//
// Parameters:
// - filePath: The path to the JSON file containing the list of contract names.
//
// Returns:
// - A slice of contract names parsed from the JSON file.
// - An error if reading the file or parsing the JSON failed.
func readLocalContractList(filePath string) ([]string, error) {
	var data contractsData
	err := readJSONFile(filePath, &data)
	return data.Local, err
}

// getContractArtifactPaths scans the provided directory for JSON contract artifacts
// and constructs a map where the keys are the contract names and the values are their
// corresponding file paths. In cases where multiple contracts share the same name, the
// path of the later instance (based on the deterministic walk order) is used.
//
// It also sanitizes the contract name by removing the compiler version from it.
//
// Parameters:
// - forgeArtifactsPath: The directory path where contract artifacts (JSON files) are located.
//
// Returns:
// - A map where keys are contract names and values are the paths to their respective JSON artifact files.
// - An error if the directory walking or processing fails.
func getContractArtifactPaths(forgeArtifactsPath string) (map[string]string, error) {
	// If some contracts have the same name then the path to their
	// artifact depends on their full import path. Scan over all artifacts
	// and hold a mapping from the contract name to the contract path.
	// Walk walks the directory deterministically, so the later instance
	// of the contract with the same name will be used
	artifactPaths := make(map[string]string)
	if err := filepath.Walk(forgeArtifactsPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if strings.HasSuffix(path, ".json") {
				base := filepath.Base(path)
				name := strings.TrimSuffix(base, ".json")

				// remove the compiler version from the name
				re := regexp.MustCompile(`\.\d+\.\d+\.\d+`)
				sanitized := re.ReplaceAllString(name, "")
				if _, ok := artifactPaths[sanitized]; !ok {
					artifactPaths[sanitized] = path
				}
			}
			return nil
		}); err != nil {
		return artifactPaths, err
	}

	return artifactPaths, nil
}

// readForgeArtifact attempts to read a contract's forge artifact located at the
// given path. If the artifact is not found at the standard location, the function
// will try to look it up in the provided map of known contract paths.
//
// Parameters:
// - forgeArtifactsPath: The base directory path where forge artifacts are expected to be located.
// - contractName: The name of the contract whose forge artifact is to be read.
// - contractArtifactPaths: A map where keys are contract names and values are the paths to their respective artifact files.
//
// Returns:
// - A `foundry.Artifact` structure containing the parsed data of the forge artifact.
// - An error if the forge artifact is not found or there's an issue reading/parsing it.
func readForgeArtifact(forgeArtifactsPath, contractName string, contractArtifactPaths map[string]string) (foundry.Artifact, error) {
	var forgeArtifact foundry.Artifact

	contractArtifactPath := path.Join(forgeArtifactsPath, contractName+".sol", contractName+".json")
	forgeArtifactRaw, err := os.ReadFile(contractArtifactPath)
	if errors.Is(err, os.ErrNotExist) {
		log.Printf("Cannot find forge-artifact for %s at standard path %s, trying %s\n", contractName, contractArtifactPath, contractArtifactPaths[contractName])
		contractArtifactPath = contractArtifactPaths[contractName]
		forgeArtifactRaw, err = os.ReadFile(contractArtifactPath)
		if errors.Is(err, os.ErrNotExist) {
			return forgeArtifact, fmt.Errorf("Cannot find forge-artifact of %q\n", contractName)
		}
	}

	log.Printf("Using forge-artifact %s\n", contractArtifactPath)
	if err := json.Unmarshal(forgeArtifactRaw, &forgeArtifact); err != nil {
		return forgeArtifact, fmt.Errorf("Failed to parse forge artifact of %q: %v\n", contractName, err)
	}

	return forgeArtifact, nil
}

// canonicalizeStorageLayout processes a given `forgeArtifact`'s storage layout and returns its canonical representation.
// This function also checks if a source map for the contract exists in the provided `sourceMapsSet`, and if it does,
// the source map for the deployed bytecode is returned as well.
//
// Parameters:
// - forgeArtifact: The `foundry.Artifact` object that contains the contract's information including its storage layout.
// - monorepoBasePath: The base path to the monorepo where contract sources are located.
// - sourceMapsSet: A set (represented as a map) of contract names that have source maps.
// - contractName: The name of the contract being processed.
//
// Returns:
// - The source map string for the deployed bytecode (if it exists in the `sourceMapsSet`, otherwise an empty string).
// - The canonical string representation of the contract's storage layout.
// - An error if any occurred during processing.
func canonicalizeStorageLayout(forgeArtifact foundry.Artifact, monorepoBasePath string, sourceMapsSet map[string]struct{}, contractName string) (string, string, error) {
	artifactStorageStruct := forgeArtifact.StorageLayout
	canonicalStorageStruct := ast.CanonicalizeASTIDs(&artifactStorageStruct, monorepoBasePath)
	canonicalStorageJson, err := json.Marshal(canonicalStorageStruct)
	if err != nil {
		return "", "", fmt.Errorf("Error marshaling canonical storage: %v", err)
	}
	canonicalStorageStr := strings.Replace(string(canonicalStorageJson), "\"", "\\\"", -1)

	deployedSourceMap := ""
	if _, ok := sourceMapsSet[contractName]; ok {
		deployedSourceMap = forgeArtifact.DeployedBytecode.SourceMap
	}

	return deployedSourceMap, canonicalStorageStr, nil
}

// writeLocalContractMetadata writes the metadata of a local contract to a file.
// The metadata file is created (or overwritten if it already exists) in the specified directory.
//
// Parameters:
// - contractMetaData: The metadata of the local contract to be written.
// - metadataOutputDir: The directory where the metadata file will be created.
// - contractName: The name of the contract. This is used to determine the name of the metadata file.
// - fileTemplate: A Go `text/template.Template` that defines the format of the metadata file.
//
// Returns:
// - An error if there's an issue creating, opening, or writing to the metadata file.
func writeLocalContractMetadata(contractMetaData localContractMetadata, metadataOutputDir, contractName string, fileTemplate *template.Template) error {
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

// genLocalBindings generates Go bindings and metadata for local contracts.
// The function reads a list of contracts from a specified file path, and for each contract,
// it fetches its Forge artifact, generates Go bindings for the contract,
// canonicalizes the storage layout, and writes the contract metadata to a file in a specified directory.
//
// Parameters:
// - contractListFilePath: The path to the file containing the list of local contracts.
// - sourceMapsListStr: A comma-separated string of source maps.
// - forgeArtifactsPath: The directory path where the Forge artifacts are stored.
// - goPackageName: The name of the Go package for the generated bindings.
// - monorepoBasePath: The base path of the monorepo.
// - metadataOutputDir: The directory where the metadata files will be written.
//
// Returns:
// - An error if there's an issue reading the contract list, generating bindings, or writing metadata.
func genLocalBindings(contractListFilePath, sourceMapsListStr, forgeArtifactsPath, goPackageName, monorepoBasePath, metadataOutputDir string) error {
	contracts, err := readLocalContractList(contractListFilePath)
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

	contractArtifactPaths, err := getContractArtifactPaths(forgeArtifactsPath)
	if err != nil {
		return err
	}

	contractMetadataFileTemplate := template.Must(template.New("localContractMetadata").Parse(localContractMetadataTemplate))

	sourceMapsList := strings.Split(sourceMapsListStr, ",")
	sourceMapsSet := make(map[string]struct{})
	for _, k := range sourceMapsList {
		sourceMapsSet[k] = struct{}{}
	}

	for _, contractName := range contracts {
		log.Printf("Generating bindings and metadata for local contract: %s", contractName)

		forgeArtifact, err := readForgeArtifact(forgeArtifactsPath, contractName, contractArtifactPaths)
		if err != nil {
			return err
		}

		abiFilePath, bytecodeFilePath, err := writeContractArtifacts(tempArtifactsDir, contractName, forgeArtifact.Abi, []byte(forgeArtifact.Bytecode.Object.String()))
		if err != nil {
			return err
		}

		err = genContractBindings(abiFilePath, bytecodeFilePath, goPackageName, contractName)
		if err != nil {
			return err
		}

		deployedSourceMap, canonicalStorageStr, err := canonicalizeStorageLayout(forgeArtifact, monorepoBasePath, sourceMapsSet, contractName)

		contractMetaData := localContractMetadata{
			Name:              contractName,
			StorageLayout:     canonicalStorageStr,
			DeployedBin:       forgeArtifact.DeployedBytecode.Object.String(),
			Package:           goPackageName,
			DeployedSourceMap: deployedSourceMap,
		}

		if err := writeLocalContractMetadata(contractMetaData, metadataOutputDir, contractName, contractMetadataFileTemplate); err != nil {
			return err
		}
	}

	return nil
}

// localContractMetadataTemplate is a Go text template for generating the metadata
// associated with a local Ethereum contract. This template is used to produce
// Go code containing necessary constants and initialization logic for the contract's
// storage layout, deployed bytecode, and optionally its deployed source map.
//
// The template expects the following fields to be provided:
// - Package: The name of the Go package for the generated bindings.
// - Name: The name of the contract.
// - StorageLayout: Canonicalized storage layout of the contract as a JSON string.
// - DeployedBin: The deployed bytecode of the contract.
// - DeployedSourceMap (optional): The source map of the deployed contract.
var localContractMetadataTemplate = `// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package {{.Package}}

import (
	"encoding/json"

	"github.com/ethereum-optimism/optimism/op-bindings/solc"
)

const {{.Name}}StorageLayoutJSON = "{{.StorageLayout}}"

var {{.Name}}StorageLayout = new(solc.StorageLayout)

var {{.Name}}DeployedBin = "{{.DeployedBin}}"
{{if .DeployedSourceMap}}
var {{.Name}}DeployedSourceMap = "{{.DeployedSourceMap}}"
{{end}}
func init() {
	if err := json.Unmarshal([]byte({{.Name}}StorageLayoutJSON), {{.Name}}StorageLayout); err != nil {
		panic(err)
	}

	layouts["{{.Name}}"] = {{.Name}}StorageLayout
	deployedBytecodes["{{.Name}}"] = {{.Name}}DeployedBin
}
`
