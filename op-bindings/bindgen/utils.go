package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
)

type contractsData struct {
	Local     []string   `json:"local"`
	Etherscan []etherscanContract `json:"etherscan"`
}

// readJSONFile reads a JSON file from the given `filePath` and unmarshals
// its content into the provided result interface. It logs the path of the file
// being read.
//
// Parameters:
// - filePath: The path to the JSON file to be read.
// - result: A pointer to the structure where the JSON data will be unmarshaled.
//
// Returns:
// - An error if reading the file or unmarshaling fails, nil otherwise.
func readJSONFile(filePath string, result interface{}) error {
	log.Printf("Reading contract list from %s\n", filePath)
	contractData, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(contractData, result)
}

// mkTempArtifactsDir creates a temporary directory with a "op-bindings" prefix
// for holding contract artifacts. The path to the created directory is logged.
//
// Returns:
// - The path to the created temporary directory.
// - An error if the directory creation fails, nil otherwise.
func mkTempArtifactsDir() (string, error) {
	dir, err := os.MkdirTemp("", "op-bindings")
	if err != nil {
		return "", err
	}

	log.Printf("Created temporary directory to hold contract artifacts: %s", dir)
	return dir, nil
}

// writeContractArtifacts writes the provided ABI and bytecode data to respective
// files in the specified temporary directory. The naming convention for these
// files is based on the provided contract name. The ABI data is written to a file
// with a ".abi" extension, and the bytecode data is written to a file with a ".bin"
// extension.
//
// Parameters:
// - tempDirPath: The directory path where the ABI and bytecode files will be written.
// - contractName: The name of the contract, used to create the filenames.
// - abi: The ABI data of the contract.
// - bytecode: The bytecode of the contract.
//
// Returns:
// - The full path to the written ABI file.
// - The full path to the written bytecode file.
// - An error if writing either file fails, nil otherwise.
func writeContractArtifacts(tempDirPath, contractName string, abi, bytecode []byte) (string, string, error) {
	log.Printf("Writing %s's ABI and bytecode to temporary dir", contractName)

	abiFilePath := path.Join(tempDirPath, contractName+".abi")
	if err := os.WriteFile(abiFilePath, abi, 0o600); err != nil {
		return "", "", fmt.Errorf("Error writing %s's ABI file: %v", contractName, err)
	}

	bytecodeFilePath := path.Join(tempDirPath, contractName+".bin")
	if err := os.WriteFile(bytecodeFilePath, bytecode, 0o600); err != nil {
		return "", "", fmt.Errorf("Error writing %s's bytecode file: %v", contractName, err)
	}

	return abiFilePath, bytecodeFilePath, nil
}

// genContractBindings generates Go bindings for an Ethereum contract using
// the provided ABI and bytecode files. The bindings are generated using the
// `abigen` tool and are written to the specified Go package directory. The
// generated file's name is based on the provided contract name and will have
// a ".go" extension. The generated bindings will be part of the provided Go
// package.
//
// Parameters:
// - abiFilePath: The path to the ABI file for the contract.
// - bytecodeFilePath: The path to the bytecode file for the contract.
// - goPackageName: The name of the Go package where the bindings will be written.
// - contractName: The name of the contract, used for naming the output file and
//                 defining the type in the generated bindings.
//
// Returns:
// - An error if there's an issue during any step of the binding generation process,
//   nil otherwise.
//
// Note: This function relies on the external `abigen` tool, which should be
//       installed and available in the system's PATH.
func genContractBindings(abiFilePath, bytecodeFilePath, goPackageName, contractName string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Error getting cwd: %v", err)
	}

	outFilePath := path.Join(cwd, goPackageName, strings.ToLower(contractName)+".go")
	log.Printf("Generating contract bindings for %s, writing to: %s", contractName, outFilePath)

	cmd := exec.Command("abigen", "--abi", abiFilePath, "--bin", bytecodeFilePath, "--pkg", goPackageName, "--type", contractName, "--out", outFilePath)
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Error running abigen for %s: %v", contractName, err)
	}

	return nil
}
