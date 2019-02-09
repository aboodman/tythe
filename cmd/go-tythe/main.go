package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/attic-labs/noms/go/d"
	gdax "github.com/preichenberger/go-gdax"
	"github.com/tythe-protocol/go-tythe/dep"
	"github.com/tythe-protocol/go-tythe/pconf"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

type command struct {
	cmd     *kingpin.CmdClause
	handler func()
}

func main() {
	app := kingpin.New("go-tythe", "A command-line tythe client.")

	commands := []command{
		list(app),
		pay(app),
		send(app),
	}

	selected := kingpin.MustParse(app.Parse(os.Args[1:]))
	for _, c := range commands {
		if selected == c.cmd.FullCommand() {
			c.handler()
			break
		}
	}
}

func list(app *kingpin.Application) (c command) {
	c.cmd = app.Command("list", "List transitive dependencies of a package")
	url := c.cmd.Arg("package-url", "URL of the package to list.").Required().URL()

	c.handler = func() {
		deps, err := dep.List(*url)
		d.CheckErrorNoUsage(err)

		for _, d := range deps {
			fmt.Println(d)
		}
	}

	return c
}

func pay(app *kingpin.Application) (c command) {
	c.cmd = app.Command("pay", "Pay a single package")
	url := c.cmd.Arg("package-url", "URL of the package to pay.").Required().URL()
	amount := c.cmd.Arg("amount", "Amount to send to the package (in USD).").Required().Float()

	c.handler = func() {
		config, err := pconf.Read(*url)
		d.CheckErrorNoUsage(err)
		if config == nil {
			fmt.Printf("no tythe.json for package: %s", (*url).String())
			return
		}

		fmt.Printf("Found tythe.json in %s:\n", (*url).String())
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		err = enc.Encode(config)
		d.CheckError(err)

		sendImpl(*amount, config.Destination.Address)
	}

	return
}

func send(app *kingpin.Application) (c command) {
	c.cmd = app.Command("send", "Sends USDC to the specified address (for testing/development)")
	address := c.cmd.Arg("address", "USDC address to send to.").Required().String()
	amount := c.cmd.Arg("amount", "Amount to send (in USD).").Required().Float()

	c.handler = func() {
		if !pconf.ValidUSDCAddress(*address) {
			fmt.Fprintln(os.Stderr, "Invalid USDC address")
			// TODO: refactor exit handling
			os.Exit(1)
			return
		}

		sendImpl(*amount, *address)
	}

	return
}

func sendImpl(amt float64, addr string) {
	key := getEnv("TYTHE_COINBASE_API_KEY")
	secret := getEnv("TYTHE_COINBASE_API_SECRET")
	passphrase := getEnv("TYTHE_COINBASE_API_PASSPHRASE")

	fmt.Printf("Really send $%f (y/n)?\n", amt)

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	d.CheckErrorNoUsage(err)

	if line != "y\n" {
		return
	}

	client := gdax.NewClient(secret, key, passphrase)
	params := map[string]interface{}{
		"amount":         amt,
		"currency":       pconf.USDC,
		"crypto_address": addr,
	}

	var res map[string]interface{}
	_, err = client.Request("POST", "/withdrawals/crypto", params, &res)
	d.PanicIfError(err)

	fmt.Printf("All done! Coinbase transaction ID: %s\n", res["id"])
}

func getEnv(s string) string {
	v := os.Getenv(s)
	if v == "" {
		fmt.Fprintf(os.Stderr, "Could not find required environment variable: %s\n", s)
		os.Exit(1)
	}
	return v
}
