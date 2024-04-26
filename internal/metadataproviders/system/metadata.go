// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package system // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/system"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/Showmax/go-fqdn"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/internal"
)

var (
	errNoPrivateIPv4 = errors.New("unable to find private IPv4")
)

// nameInfoProvider abstracts domain name resolution so it can be swapped for
// testing
type nameInfoProvider struct {
	osHostname  func() (string, error)
	lookupCNAME func(string) (string, error)
	lookupHost  func(string) ([]string, error)
	lookupAddr  func(string) ([]string, error)
}

// newNameInfoProvider creates a name info provider for production use, using
// DNS to resolve domain names
func newNameInfoProvider() nameInfoProvider {
	return nameInfoProvider{
		osHostname:  os.Hostname,
		lookupCNAME: net.LookupCNAME,
		lookupHost:  net.LookupHost,
		lookupAddr:  net.LookupAddr,
	}
}

type Provider interface {
	// Hostname returns the OS hostname
	Hostname() (string, error)

	// FQDN returns the fully qualified domain name
	FQDN() (string, error)

	// OSDescription returns a human readable description of the OS.
	OSDescription(ctx context.Context) (string, error)

	// OSType returns the host operating system
	OSType() (string, error)

	// LookupCNAME returns the canonical name for the current host
	LookupCNAME() (string, error)

	// ReverseLookupHost does a reverse DNS query on the current host's IP address
	ReverseLookupHost() (string, error)

	// HostID returns Host Unique Identifier
	HostID(ctx context.Context) (string, error)

	// HostArch returns the host architecture
	HostArch() (string, error)

	// HostIP returns the host IP
	HostIP() (string, error)
}

type systemMetadataProvider struct {
	nameInfoProvider
	newResource   func(context.Context, ...resource.Option) (*resource.Resource, error)
	netInterfaces func() ([]netInterface, error)
}

func NewProvider() Provider {
	return systemMetadataProvider{
		nameInfoProvider: newNameInfoProvider(),
		newResource:      resource.New,
		netInterfaces: func() ([]netInterface, error) {
			interfaces, err := net.Interfaces()
			if err != nil {
				return nil, err
			}
			return filterInterfaces(interfaces), nil
		},
	}
}

func (systemMetadataProvider) OSType() (string, error) {
	return internal.GOOSToOSType(runtime.GOOS), nil
}

func (systemMetadataProvider) FQDN() (string, error) {
	return fqdn.FqdnHostname()
}

func (p systemMetadataProvider) Hostname() (string, error) {
	return p.nameInfoProvider.osHostname()
}

func (p systemMetadataProvider) LookupCNAME() (string, error) {
	hostname, err := p.Hostname()
	if err != nil {
		return "", fmt.Errorf("LookupCNAME failed to get hostname: %w", err)
	}
	cname, err := p.nameInfoProvider.lookupCNAME(hostname)
	if err != nil {
		return "", fmt.Errorf("LookupCNAME failed to get CNAME: %w", err)
	}
	return strings.TrimRight(cname, "."), nil
}

func (p systemMetadataProvider) ReverseLookupHost() (string, error) {
	hostname, err := p.Hostname()
	if err != nil {
		return "", fmt.Errorf("ReverseLookupHost failed to get hostname: %w", err)
	}
	return p.hostnameToDomainName(hostname)
}

func (p systemMetadataProvider) hostnameToDomainName(hostname string) (string, error) {
	ipAddresses, err := p.nameInfoProvider.lookupHost(hostname)
	if err != nil {
		return "", fmt.Errorf("hostnameToDomainName failed to convert hostname to IP addresses: %w", err)
	}
	return p.reverseLookup(ipAddresses)
}

func (p systemMetadataProvider) reverseLookup(ipAddresses []string) (string, error) {
	var err error
	for _, ip := range ipAddresses {
		var names []string
		names, err = p.nameInfoProvider.lookupAddr(ip)
		if err != nil {
			continue
		}
		return strings.TrimRight(names[0], "."), nil
	}
	return "", fmt.Errorf("reverseLookup failed to convert IP addresses to name: %w", err)
}

func (p systemMetadataProvider) fromOption(ctx context.Context, opt resource.Option, semconv string) (string, error) {
	res, err := p.newResource(ctx, opt)
	if err != nil {
		return "", fmt.Errorf("failed to obtain %q: %w", semconv, err)
	}

	iter := res.Iter()
	for iter.Next() {
		if iter.Attribute().Key == attribute.Key(semconv) {
			v := iter.Attribute().Value.Emit()

			if v == "" {
				return "", fmt.Errorf("empty %q", semconv)
			}
			return v, nil
		}
	}

	return "", fmt.Errorf("failed to obtain %q", semconv)
}

func (p systemMetadataProvider) HostID(ctx context.Context) (string, error) {
	return p.fromOption(ctx, resource.WithHostID(), conventions.AttributeHostID)
}

func (p systemMetadataProvider) OSDescription(ctx context.Context) (string, error) {
	return p.fromOption(ctx, resource.WithOSDescription(), conventions.AttributeOSDescription)
}

func (systemMetadataProvider) HostArch() (string, error) {
	return internal.GOARCHtoHostArch(runtime.GOARCH), nil
}

func (p systemMetadataProvider) HostIP() (string, error) {
	interfaces, err := p.netInterfaces()
	if err != nil {
		return "", fmt.Errorf("failed to load network interfaces: %w", err)
	}
	return selectIP(toIPv4s(interfaces))
}

func filterInterfaces(interfaces []net.Interface) []netInterface {
	sort.Sort(byIndex(interfaces))
	var ifaces []netInterface
	for _, i := range interfaces {
		iface := i
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagPointToPoint != 0 {
			continue
		}
		ifaces = append(ifaces, &iface)
	}
	return ifaces
}

func toIPv4s(interfaces []netInterface) []net.IP {
	var ipv4s []net.IP
	for _, i := range interfaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPAddr:
				ipv4s = append(ipv4s, v.IP.To4())
			case *net.IPNet:
				ipv4s = append(ipv4s, v.IP.To4())
			}
		}
	}
	return ipv4s
}

func selectIP(ips []net.IP) (string, error) {
	var result net.IP
	for _, ip := range ips {
		if ip != nil && !ip.IsUnspecified() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && ip.IsPrivate() {
			result = ip
			break
		}
	}
	if result == nil {
		return "", errNoPrivateIPv4
	}
	return result.String(), nil
}

// netInterface is used to stub out the Addrs function to avoid the syscalls
type netInterface interface {
	Addrs() ([]net.Addr, error)
}

// byIndex implements sorting for net.Interface.
type byIndex []net.Interface

func (b byIndex) Len() int           { return len(b) }
func (b byIndex) Less(i, j int) bool { return b[i].Index < b[j].Index }
func (b byIndex) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
