package main

import (
	dhcp "github.com/krolaw/dhcp4"
	"math/rand"
	"net"
	"fmt"
	"time"
	"os"
	"syscall"
)


func NewUDP4BoundListener(interfaceName, laddr string) (pc net.PacketConn, e error) {
	addr, err := net.ResolveUDPAddr("udp4", laddr)
	if err != nil {
		return nil, err
	}

	s, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	if err != nil {
		return nil, err
	}
	defer func() { // clean up if something goes wrong
		if e != nil {
			syscall.Close(s)
		}
	}()

	if err := syscall.SetsockoptInt(s, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		return nil, err
	}
	if err := syscall.SetsockoptInt(s, syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1); err != nil {
		return nil, err
	}
	if err := syscall.SetsockoptString(s, syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, interfaceName); err != nil {
		return nil, err
	}

	lsa := syscall.SockaddrInet4{Port: addr.Port}
	copy(lsa.Addr[:], addr.IP.To4())

	if err := syscall.Bind(s, &lsa); err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(s), "")
	defer f.Close()
	return net.FilePacketConn(f)
}

type DHCPServer struct {
	ip            net.IP        // Server IP to use
	options       dhcp.Options  // Options to send to DHCP Clients
	start         net.IP        // Start of IP range to distribute
	leaseRange    int           // Number of IPs to distribute (starting from start)
	leaseDuration time.Duration // Lease period
	leases        map[int]lease // Map to keep track of leases
}

type lease struct {
	nic    string    // Client's CHAddr
	expiry time.Time // When the lease expires
}

func (s *DHCPServer) ServeDHCP(p dhcp.Packet, msgType dhcp.MessageType, options dhcp.Options) (d dhcp.Packet) {
	fmt.Printf("ServeDHCP\n")
	switch msgType {
	case dhcp.Discover:
		free, nic := -1, p.CHAddr().String()
		for i, v := range s.leases { // Find previous lease
			if v.nic == nic {
				free = i
				goto reply
			}
		}
		if free = s.freeLease(); free == -1 {
			return
		}
	reply:
		return dhcp.ReplyPacket(p, dhcp.Offer, s.ip, dhcp.IPAdd(s.start, free), s.leaseDuration,
			s.options.SelectOrderOrAll(options[dhcp.OptionParameterRequestList]))
	case dhcp.Request:
		if server, ok := options[dhcp.OptionServerIdentifier]; ok && !net.IP(server).Equal(s.ip) {
			return nil // Message not for this dhcp server
		}
		if reqIP := net.IP(options[dhcp.OptionRequestedIPAddress]); len(reqIP) == 4 {
			if leaseNum := dhcp.IPRange(s.start, reqIP) - 1; leaseNum >= 0 && leaseNum < s.leaseRange {
				if l, exists := s.leases[leaseNum]; !exists || l.nic == p.CHAddr().String() {
					s.leases[leaseNum] = lease{nic: p.CHAddr().String(), expiry: time.Now().Add(s.leaseDuration)}
					return dhcp.ReplyPacket(p, dhcp.ACK, s.ip, net.IP(options[dhcp.OptionRequestedIPAddress]), s.leaseDuration,
						s.options.SelectOrderOrAll(options[dhcp.OptionParameterRequestList]))
				}
			}
		}
		return dhcp.ReplyPacket(p, dhcp.NAK, s.ip, nil, 0, nil)
	case dhcp.Release, dhcp.Decline:
		nic := p.CHAddr().String()
		for i, v := range s.leases {
			if v.nic == nic {
				delete(s.leases, i)
				break
			}
		}
	}
	return nil
}

func (s *DHCPServer) freeLease() int {
	now := time.Now()
	b := rand.Intn(s.leaseRange) // Try random first
	for _, v := range [][]int{[]int{b, s.leaseRange}, []int{0, b}} {
		for i := v[0]; i < v[1]; i++ {
			if l, ok := s.leases[i]; !ok || l.expiry.Before(now) {
				return i
			}
		}
	}
	return -1
}
//192.16.200
func StartDhcpServer() {

	if os.Getenv("DHCPSERVER_ENABLED") == "true" {

		interfaceIP := net.ParseIP(os.Getenv("DHCPSERVER_IP")).To4()
		dhcpserverRangeIP := net.ParseIP(os.Getenv("DHCPSERVER_RANGESTART")).To4()
		dhcpserverGatewayIP := net.ParseIP(os.Getenv("DHCPSERVER_GATEWAY")).To4()
		dhcpserverDnsIP := net.ParseIP(os.Getenv("DHCPSERVER_DNS")).To4()
		interfaceNic := os.Getenv("DHCPSERVER_INTERFACE")
		if interfaceIP == nil {
	                fmt.Fprintf(os.Stderr, "DHCPSERVER_IP missing or wrong\n")
	                return 
		}else{
			fmt.Printf("DHCPSERVER_IP:%s\n", interfaceIP)
			fmt.Printf("DHCPSERVER_RANGESTART:%s\n", dhcpserverRangeIP)
			fmt.Printf("DHCPSERVER_INTERFACE:%s\n", interfaceNic)
			fmt.Printf("DHCPSERVER_GATEWAY:%s\n", dhcpserverGatewayIP)
			fmt.Printf("DHCPSERVER_DNS:%s\n", dhcpserverDnsIP)
		}

	server := &DHCPServer{
//		ip:            net.IP{192, 16, 200, 2},
		ip:            interfaceIP,
		leaseDuration: 2 * time.Hour,
		start:         dhcpserverRangeIP,
		leaseRange:    50,
		leases:        make(map[int]lease, 10),
	}
	server.options = dhcp.Options{
		dhcp.OptionSubnetMask:       []byte{255, 255, 255, 0},
		dhcp.OptionRouter:           []byte(dhcpserverGatewayIP), // Presuming Server is also your router
		dhcp.OptionDomainNameServer: []byte(dhcpserverDnsIP), // Presuming Server is also your DNS server
		dhcp.OptionBootFileName: []byte("lpxelinux.0"), // Presuming Server is also your DNS server
	}
	fmt.Printf("StartDhcpServer\n")
	interfaceS, err := NewUDP4BoundListener(interfaceNic,":67")
	        if err != nil {
                fmt.Fprintf(os.Stderr, "%v\n", err)
                return 
        }

	panic(dhcp.Serve(interfaceS, server))
	//log.Fatal(dhcp.Serve(interface2, server)) // Select interface on multi interface device - just linux for now
	//panic(dhcp.ListenAndServe(server).Error())
	//panic((&dhcp.Server{Handler: server, ServerIP: server.ip}).ListenAndServe().Error())
	}
}

