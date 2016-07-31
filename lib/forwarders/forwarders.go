package forwarders

// Forwarder defines a log forwarder
type Forwarder struct {
	// Addr defines the address the forwarder is listening on
	Addr string `json:"address"`
	// Protocol defines the protocol to configure for this forwarder (TCP/UDP)
	Protocol string `json:"protocol"`
}
