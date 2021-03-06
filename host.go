package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/fatih/color"
	text "github.com/tonnerre/golang-text"
	"log"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

// A HostInfo is a YAML file with information about a group of hosts
type HostInfo struct {
	RawType    string   `yaml:"type"`
	RawSummary string   `yaml:"summary"`
	Types      HostType `yaml:"types"`
	id         string
	path       string
	repo       *Repo
}

// HostType is a collection of categories
type HostType map[string]Category

// Category defines a set categories of machines
type Category struct {
	Summary string `yaml:"summary"`
	Primary bool   `yaml:"primary"`
	Hosts   []Host `yaml:"hosts"`
}

// Host is a representation of one host
type Host struct {
	FQDN    string `yaml:"fqdn"`
	Summary string `yaml:"summary"`
	Kind    string `yaml:"kind"`
	Primary bool   `yaml:"primary"`
}

func (h HostInfo) String() string {
	return fmt.Sprintf("H: %s (%d)", h.ID(), len(h.Types))
}

// Execute opens a ssh connection to the specified host
func (h HostInfo) Execute(c *cli.Context) {
	args := c.Args()
	arglen := len(args)

	switch arglen {
	case 0:
		// No further arguments - we have selected a host entry but no type.
		// Print the list of Types.
		h.Types.PrintType()

	case 1, 2:
		t := args[0]
		if cat, ok := h.Types[t]; ok {
			if arglen == 1 {
				// One argument, go to the primary of that category
				cat.PrimaryHost().Execute("")
			} else {
				// Two arguments, go to specified host
				x, err := strconv.Atoi(args[1])
				if err != nil {
					log.Fatal("Non-integer argument:", args[1])
				}

				host := cat.Hosts[x]
				host.Execute("")
			}

		} else {
			fmt.Println("No such type:", t)
			fmt.Println(
				fmt.Sprintf("Choices are: %s", strings.Join(h.Types.List(), ", ")),
			)
			os.Exit(1)
		}
	}
}

// ID returns the ID of the item
func (h HostInfo) ID() string {
	return h.id
}

// Type returns the Type of the item
func (h HostInfo) Type() string {
	return h.RawType
}

// Path returns the path of the item
func (h HostInfo) Path() string {
	return h.path
}

// Summary returns the summary of the item
func (h HostInfo) Summary() string {
	return h.RawSummary
}

// MakeCLI creates the CLI tree for a Host info
func (h HostInfo) MakeCLI() []cli.Command {
	sc := make([]cli.Command, 0, len(h.Types))
	for _, key := range h.Types.List() {
		cat := h.Types[key]
		cc := cli.Command{ // cc = category command
			Name:        key,
			Usage:       cat.Summary,
			HideHelp:    true,
			Subcommands: make([]cli.Command, 0, len(cat.Hosts)),
			Action: func(c *cli.Context) {
				cat.PrimaryHost().Execute("")
			},
		}

		for _, host := range cat.Hosts {
			hc := cli.Command{ // hc = host command
				Name:     host.FQDN,
				Usage:    host.Summary,
				HideHelp: true,
				Action: func(c *cli.Context) {
					var host *Host
					args := c.Args()

					if len(args) == 0 {
						// No extra arguments - go to the primary host
						host = cat.PrimaryHost()
					} else {
						// Arguments were defined - go to the fqdn specified
						// TODO(thiderman): Error handling, integer index handling
						host = cat.GetHost(args[0])
					}

					host.Execute("")
				},
			}
			cc.Subcommands = append(cc.Subcommands, hc)
		}

		sc = append(sc, cc)
	}
	return sc
}

// getHosts gets a string representation of all of the Types in the item
func (h HostInfo) getHosts() (Types []string) {
	for _, host := range h.Types.Hosts() {
		Types = append(Types, host.FQDN)
	}

	return
}

// PrimaryHost returns the primary host inside of the HostInfo
func (c *Category) PrimaryHost() (h *Host) {
	for _, host := range c.Hosts {
		if host.Primary {
			h = &host
			return
		}
	}

	// No primary was found, just pick the first one
	return &c.Hosts[0]
}

// GetHost returns a specific host, based on FQDN
func (c *Category) GetHost(fqdn string) (h *Host) {
	for _, host := range c.Hosts {
		if host.FQDN == fqdn {
			return &host
		}
	}
	return
}

// List returns a list of the types in the category map
func (h HostType) List() (keys []string) {
	for key := range h {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	return keys
}

// Hosts returns an array of all the Types in the category map
func (h HostType) Hosts() (hosts []Host) {
	for _, cat := range h {
		for _, host := range cat.Hosts {
			hosts = append(hosts, host)
		}
	}

	return hosts
}

// PrimaryHost returns the primary host of the category
func (h HostType) PrimaryHost() *Host {
	for _, cat := range h {
		if cat.Primary {
			return cat.PrimaryHost()
		}
	}
	return nil
}

// PrintType prints a pretty list of the different types and their hosts
func (h HostType) PrintType() {
	blue := color.New(color.FgBlue, color.Bold).SprintfFunc()
	green := color.New(color.FgGreen, color.Bold).SprintfFunc()
	cyan := color.New(color.FgCyan, color.Bold).SprintfFunc()
	yellow := color.New(color.FgYellow).SprintfFunc()
	hiyellow := color.New(color.FgHiYellow, color.Bold).SprintfFunc()
	grey := color.New(color.FgWhite).SprintfFunc()

	for _, t := range h.List() {
		fmt.Println(fmt.Sprintf("%s:", cyan(t)))
		cat := h[t]
		fmt.Printf("  %s\n", text.Wrap(cat.Summary, 80))
		for x, host := range cat.Hosts {
			// Print the main host item
			fmt.Printf(
				"  %s%s%s %s",
				yellow("["),
				hiyellow(strconv.Itoa(x)),
				yellow("]"),
				blue(host.FQDN),
			)

			// If the host is primary, mark that clearly
			if host.Primary {
				fmt.Printf(" (%s)", green("primary"))
			}

			// If the host has a summary, add that as well
			if host.Summary != "" {
				fmt.Printf(" (%s)", grey(host.Summary))
			}

			fmt.Println()
		}
		fmt.Println()
	}
}

// hasHost returns true if there is a Host definition and false if not.
func (h *Host) hasHost() bool {
	return h.FQDN != ""
}

// Execute runs a command on the server
// The default is to open a shell. If arguments are given, those arguments
// will be executed verbatim on the host.
func (h *Host) Execute(extra ...string) {
	ssh, _ := exec.LookPath("ssh")

	args := append([]string{ssh, h.FQDN, "-A", "-t"}, extra...)

	cmd := exec.Cmd{
		Path:   ssh,
		Args:   args,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  os.Stdin,
	}

	err := cmd.Run()
	if err != nil {
		log.Fatal("ssh command failed: ", err)
	}
}
