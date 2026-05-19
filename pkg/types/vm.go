package types

type Service struct {
	Mode      PortMode `json:"mode" yaml:"mode"`
	ExtPort   uint16   `json:"ext_port" yaml:"ext_port"`
	GuestPort uint16   `json:"guest_port" yaml:"guest_port"`
	Public    bool     `json:"public" yaml:"public"`
}

type PortMode string

const (
	PortModeHTTPS PortMode = "https"
	PortModeTCP   PortMode = "tcp"
)

type Volume struct {
	Name      string
	MountPath string
	SizeGB    uint
	ReadOnly  bool
}

type Exec struct {
	Env     map[string]string
	Command string
	Args    []string
}

type VM struct {
	Name     string
	CPUs     int
	MemoryGB int
	Image    string
	Ports    []Service
	Exec     Exec
	Volumes  []Volume
}
