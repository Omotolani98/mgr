// Package sshconfig imports OpenSSH config entries into mgr inventory servers.
package sshconfig

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Omotolani98/mgr/internals/inventory"
)

type hostBlock struct {
	aliases      []string
	hostName     string
	user         string
	port         int
	identityFile string
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ssh", "config")
}

func Import(path string) ([]inventory.Server, error) {
	if path == "" {
		path = DefaultPath()
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var servers []inventory.Server
	var current *hostBlock
	flush := func() {
		if current == nil {
			return
		}
		for _, alias := range current.aliases {
			if shouldSkipAlias(alias) {
				continue
			}
			host := current.hostName
			if host == "" {
				host = alias
			}
			port := current.port
			if port == 0 {
				port = 22
			}
			servers = append(servers, inventory.Server{
				Name:         alias,
				Host:         expandHome(host),
				Port:         port,
				User:         current.user,
				IdentityFile: expandHome(current.identityFile),
				Tags:         []string{"ssh-config"},
			})
		}
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := stripComment(strings.TrimSpace(scanner.Text()))
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.ToLower(fields[0])
		values := fields[1:]
		switch key {
		case "host":
			flush()
			current = &hostBlock{aliases: values}
		case "match":
			flush()
			current = nil
		case "hostname":
			if current != nil {
				current.hostName = values[0]
			}
		case "user":
			if current != nil {
				current.user = values[0]
			}
		case "port":
			if current != nil {
				if port, err := strconv.Atoi(values[0]); err == nil {
					current.port = port
				}
			}
		case "identityfile":
			if current != nil {
				current.identityFile = strings.Join(values, " ")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	flush()
	return servers, nil
}

func shouldSkipAlias(alias string) bool {
	return alias == "" || strings.ContainsAny(alias, "*?!")
}

func stripComment(line string) string {
	if i := strings.IndexByte(line, '#'); i >= 0 {
		return strings.TrimSpace(line[:i])
	}
	return line
}

func expandHome(path string) string {
	if path == "" || !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return path
}
