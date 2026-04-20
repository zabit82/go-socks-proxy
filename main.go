package main

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

var (
	allowedIPs map[string]bool
	allowAll   bool
)

// init parses settings from environment variables before the server starts
func init() {
	ipsEnv := os.Getenv("ALLOWED_IPS")
	
	if ipsEnv == "" {
		allowAll = true
		log.Println("ALLOWED_IPS environment variable is not set. Allowing connections from all IPs.")
		return
	}

	allowedIPs = make(map[string]bool)
	ips := strings.Split(ipsEnv, ",")
	for _, ip := range ips {
		cleanIP := strings.TrimSpace(ip)
		if cleanIP != "" {
			allowedIPs[cleanIP] = true
		}
	}
	
	log.Printf("IP filter enabled. Allowed addresses: %s", ipsEnv)
}

// isAllowedIP checks if the client's IP address is in the whitelist
func isAllowedIP(addr net.Addr) bool {
	// If the filter is disabled, allow everyone
	if allowAll {
		return true
	}

	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		return false
	}
	
	return allowedIPs[tcpAddr.IP.String()]
}

func main() {
	// Get the port from the environment variable. If empty, use 1080.
	port := os.Getenv("SOCKS_PROXY_PORT")
	if port == "" {
		port = "1080"
	}

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()

	log.Printf("SOCKS5 proxy started on port %s", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}

		// Immediately drop clients not in the whitelist
		if !isAllowedIP(conn.RemoteAddr()) {
			log.Printf("Blocked connection from %s", conn.RemoteAddr())
			conn.Close()
			continue
		}

		log.Printf("Accepted connection from %s", conn.RemoteAddr())
		go handleConnection(conn)
	}
}

func handleConnection(client net.Conn) {
	defer client.Close()
	if err := socks5Handshake(client); err != nil {
		log.Printf("Error with %s: %v", client.RemoteAddr(), err)
		return
	}
}

// socks5Handshake handles the SOCKS5 protocol and establishes a tunnel
func socks5Handshake(client net.Conn) error {
	buf := make([]byte, 256)

	// 1. Read client greeting
	if _, err := io.ReadFull(client, buf[:2]); err != nil {
		return err
	}
	if buf[0] != 0x05 {
		return errors.New("only SOCKS5 protocol is supported")
	}
	numMethods := int(buf[1])
	if _, err := io.ReadFull(client, buf[:numMethods]); err != nil {
		return err
	}

	// 2. Respond to client: version 5, method 0x00 (No authentication)
	if _, err := client.Write([]byte{0x05, 0x00}); err != nil {
		return err
	}

	// 3. Read connection request details
	if _, err := io.ReadFull(client, buf[:4]); err != nil {
		return err
	}
	if buf[0] != 0x05 {
		return errors.New("invalid SOCKS version in request")
	}
	if buf[1] != 0x01 { // Only CONNECT command (0x01) is supported
		return errors.New("only CONNECT command is supported")
	}

	addrType := buf[3]
	var destAddr string

	// Determine destination address type
	switch addrType {
	case 0x01: // IPv4
		if _, err := io.ReadFull(client, buf[:4]); err != nil {
			return err
		}
		destAddr = net.IP(buf[:4]).String()
	case 0x03: // Domain name
		if _, err := io.ReadFull(client, buf[:1]); err != nil {
			return err
		}
		domainLen := int(buf[0])
		if _, err := io.ReadFull(client, buf[:domainLen]); err != nil {
			return err
		}
		destAddr = string(buf[:domainLen])
	case 0x04: // IPv6
		if _, err := io.ReadFull(client, buf[:16]); err != nil {
			return err
		}
		destAddr = net.IP(buf[:16]).String()
	default:
		return errors.New("unknown address type")
	}

	// Read destination port
	if _, err := io.ReadFull(client, buf[:2]); err != nil {
		return err
	}
	destPort := binary.BigEndian.Uint16(buf[:2])
	destTarget := net.JoinHostPort(destAddr, strconv.Itoa(int(destPort)))

	// 4. Establish connection to the target server
	target, err := net.Dial("tcp", destTarget)
	if err != nil {
		// Tell the client that the host is unreachable (0x04)
		client.Write([]byte{0x05, 0x04, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return err
	}
	defer target.Close()

	// 5. Tell the client the connection is established (0x00)
	if _, err := client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return err
	}

	log.Printf("Tunnel opened: %s <-> %s", client.RemoteAddr(), destTarget)

	// 6. Proxy data (create a bidirectional tunnel)
	go func() {
		io.Copy(target, client)
		target.Close() // Close the target when the client disconnects
	}()
	io.Copy(client, target)

	return nil
}
