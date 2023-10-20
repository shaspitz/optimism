package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "BindGen",
		Usage: "Generate contract bindings using Foundry artifacts and/or Etherscan API",
		Commands: []*cli.Command{
			{
				Name:  "generate",
				Usage: "Generate bindings for both local and etherscan contracts",
				Subcommands: []*cli.Command{
					{
						Name:   "all",
						Usage:  "Generate bindings for local contracts and from Etherscan",
						Action: generateAllBindings,
						Flags:  append(localFlags(), etherscanFlags()...),
					},
					{
						Name:   "local",
						Usage:  "Generate bindings for local contracts",
						Action: generateLocalBindings,
						Flags:  localFlags(),
					},
					{
						Name:   "etherscan",
						Usage:  "Generate bindings for contracts from Etherscan",
						Action: generateEtherscanBindings,
						Flags:  etherscanFlags(),
					},
				},
				Flags: generateFlags(),
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func generateAllBindings(c *cli.Context) error {
	if err := generateLocalBindings(c); err != nil {
		log.Fatal(err)
	}
	if err := generateEtherscanBindings(c); err != nil {
		log.Fatal(err)
	}
	return nil
}

func generateLocalBindings(c *cli.Context) error {
	if err := genLocalBindings(c.String("local-contracts"), c.String("source-maps-list"), c.String("forge-artifacts"), c.String("go-package"), c.String("monorepo-base"), c.String("metadata-out")); err != nil {
		log.Fatal(err)
	}
	return nil
}

func generateEtherscanBindings(c *cli.Context) error {
	if err := genEtherscanBindings(c.String("etherscan-contracts"), c.String("source-maps-list"), c.String("etherscan-apikey"), c.String("go-package"), c.String("metadata-out"), c.Int("api-max-retries"), c.Int("api-retry-delay")); err != nil {
		log.Fatal(err)
	}
	return nil
}

func generateFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "metadata-out",
			Usage:    "Output directory to put contract metadata files in",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "go-package",
			Usage:    "Go package name given to generated files",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "monorepo-base",
			Usage:    "Path to the base of the monorepo",
			Required: true,
		},
	}
}

func localFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "local-contracts",
			Usage:    "Path to file containing list of contracts to generate bindings for that have Forge artifacts available locally",
			Required: true,
		},
		&cli.StringFlag{
			Name:  "source-maps-list",
			Usage: "Comma-separated list of contracts to generate source-maps for",
		},
		&cli.StringFlag{
			Name:     "forge-artifacts",
			Usage:    "Path to forge-artifacts directory, containing compiled contract artifacts",
			Required: true,
		},
	}
}

func etherscanFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "etherscan-contracts",
			Usage:    "Path to file containing list of contracts to generate bindings for that will have ABI and bytecode sourced from Etherscan API",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "etherscan-apikey",
			Usage:    "Etherscan API key",
			Required: true,
		},
		&cli.IntFlag{
			Name:  "api-max-retries",
			Usage: "Max number of retries for getting a contract's ABI from Etherscan if rate limit is reached",
			Value: 3,
		},
		&cli.IntFlag{
			Name:  "api-retry-delay",
			Usage: "Number of seconds before trying to fetch a contract's ABI from Etherscan if rate limit is reached",
			Value: 2,
		},
	}
}
