package forwarders

// Forwarder defines a log forwarder
type Forwarder struct {
	// HostPort defines the address the forwarder is listening on
	HostPort string `json:"host_port"`
	// Protocol defines the protocol to configure for this forwarder (TCP/UDP)
	Protocol string `json:"protocol"`
}
