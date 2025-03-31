package defs

import "time"

// Protocol constants
const (
	MagicNumber uint16 = 0xCAFE

	// Message types
	MsgWorkerRegister  byte = 0x01
	MsgWorkerHeartbeat byte = 0x02
	MsgJobRequest      byte = 0x03
	MsgJobAssign       byte = 0x04
	MsgJobResult       byte = 0x05
	MsgJobAvailable    byte = 0x06
	MsgError           byte = 0x07

	// Configuration constants
	InitialRegistrationTimeout = 30 * time.Second
	ConnectionRetryDelay       = 1 * time.Second
)
