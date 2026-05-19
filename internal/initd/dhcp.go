//go:build linux

package initd

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
	"github.com/vishvananda/netlink"
)

// ConfigureDHCP requests a lease for each usable non-loopback interface.
func ConfigureDHCP(ctx context.Context) error {
	ifaces, err := net.Interfaces()
	if err != nil {
		return fmt.Errorf("list interfaces: %w", err)
	}

	var (
		configured int
		errs       []error
	)
	for _, iface := range ifaces {
		if !usableInterface(iface) {
			continue
		}

		log.Printf("dhcp: iface=%s", iface.Name)
		if err := configureInterface(ctx, iface); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", iface.Name, err))
			continue
		}
		configured++
	}
	if configured > 0 {
		return nil
	}
	if len(errs) == 0 {
		return errors.New("no usable network interfaces")
	}
	return errors.Join(errs...)
}

func usableInterface(iface net.Interface) bool {
	return iface.Flags&net.FlagLoopback == 0 && len(iface.HardwareAddr) > 0 && iface.Name != "dummy0"
}

func configureInterface(ctx context.Context, iface net.Interface) error {
	link, err := netlink.LinkByName(iface.Name)
	if err != nil {
		return fmt.Errorf("lookup link: %w", err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("bring link up: %w", err)
	}

	client, err := nclient4.New(iface.Name)
	if err != nil {
		return fmt.Errorf("create dhcp client: %w", err)
	}
	defer client.Close()

	lease, err := client.Request(ctx, dhcpv4.WithRequestedOptions(
		dhcpv4.OptionSubnetMask,
		dhcpv4.OptionRouter,
		dhcpv4.OptionDomainNameServer,
	))
	if err != nil {
		return fmt.Errorf("request lease: %w", err)
	}
	return applyLease(link, lease)
}

func applyLease(link netlink.Link, lease *nclient4.Lease) error {
	if lease == nil || lease.ACK == nil {
		return errors.New("empty dhcp lease")
	}

	ack := lease.ACK
	if ip := ack.YourIPAddr.To4(); ip == nil {
		return errors.New("lease missing ipv4 address")
	}
	if len(ack.SubnetMask()) != net.IPv4len {
		return errors.New("lease missing subnet mask")
	}

	addr := &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   ack.YourIPAddr.To4(),
			Mask: ack.SubnetMask(),
		},
	}
	if err := netlink.AddrReplace(link, addr); err != nil {
		return fmt.Errorf("set address: %w", err)
	}

	if routers := ack.Router(); len(routers) > 0 {
		if err := netlink.RouteReplace(&netlink.Route{
			LinkIndex: link.Attrs().Index,
			Gw:        routers[0],
		}); err != nil {
			return fmt.Errorf("set default route: %w", err)
		}
	}

	if dns := ack.DNS(); len(dns) > 0 {
		if err := writeResolvConf(dns); err != nil {
			return fmt.Errorf("write resolv.conf: %w", err)
		}
	}
	return nil
}

func writeResolvConf(servers []net.IP) error {
	var b strings.Builder
	for _, server := range servers {
		if ip := server.To4(); ip != nil {
			fmt.Fprintf(&b, "nameserver %s\n", ip)
		}
	}
	if b.Len() == 0 {
		return nil
	}
	return os.WriteFile("/etc/resolv.conf", []byte(b.String()), 0644)
}
