package charger

import (
	"fmt"
	"net"
	"time"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/modbus"
)

const (
	phEMCPRegStatus     = 100 // Input
	phEMCPRegChargeTime = 102 // Input
	phEMCPRegMaxCurrent = 300 // Holding
	phEMCPRegEnable     = 400 // Coil
)

// PhoenixEMCP is an api.ChargeController implementation for Phoenix EM-CP-PP-ETH wallboxes.
// It uses Modbus TCP to communicate with the wallbox at modbus client id 180.
type PhoenixEMCP struct {
	log  *util.Logger
	conn *modbus.Connection
}

func init() {
	registry.Add("phoenix-emcp", NewPhoenixEMCPFromConfig)
}

// NewPhoenixEMCPFromConfig creates a Phoenix charger from generic config
func NewPhoenixEMCPFromConfig(other map[string]interface{}) (api.Charger, error) {
	cc := struct {
		URI string
		ID  uint8
	}{
		URI: "192.168.0.8:502", // default
		ID:  180,               // default
	}

	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}

	if _, _, err := net.SplitHostPort(cc.URI); err != nil {
		return nil, fmt.Errorf("missing or invalid phoenix uri: %s", cc.URI)
	}

	return NewPhoenixEMCP(cc.URI, cc.ID)
}

// NewPhoenixEMCP creates a Phoenix charger
func NewPhoenixEMCP(uri string, id uint8) (api.Charger, error) {
	log := util.NewLogger("emcp")

	conn, err := modbus.NewConnection(uri, "", "", 0, false, id)
	if err != nil {
		return nil, err
	}

	wb := &PhoenixEMCP{
		log:  log,
		conn: conn,
	}

	return wb, nil
}

// Status implements the Charger.Status interface
func (wb *PhoenixEMCP) Status() (api.ChargeStatus, error) {
	b, err := wb.conn.ReadInputRegisters(phEMCPRegStatus, 1)
	wb.log.TRACE.Printf("read status (%d): %0 X", phEMCPRegStatus, b)
	if err != nil {
		return api.StatusNone, err
	}

	return api.ChargeStatus(string(b[1])), nil
}

// Enabled implements the Charger.Enabled interface
func (wb *PhoenixEMCP) Enabled() (bool, error) {
	b, err := wb.conn.ReadCoils(phEMCPRegEnable, 1)
	wb.log.TRACE.Printf("read charge enable (%d): %0 X", phEMCPRegEnable, b)
	if err != nil {
		return false, err
	}

	return b[0] == 1, nil
}

// Enable implements the Charger.Enable interface
func (wb *PhoenixEMCP) Enable(enable bool) error {
	var u uint16
	if enable {
		u = 0xFF00
	}

	b, err := wb.conn.WriteSingleCoil(phEMCPRegEnable, u)
	wb.log.TRACE.Printf("write charge enable (%d) %0X: %0 X", phEMCPRegEnable, u, b)

	return err
}

// MaxCurrent implements the Charger.MaxCurrent interface
func (wb *PhoenixEMCP) MaxCurrent(current int64) error {
	if current < 6 {
		return fmt.Errorf("invalid current %d", current)
	}

	b, err := wb.conn.WriteSingleRegister(phEMCPRegMaxCurrent, uint16(current))
	wb.log.TRACE.Printf("write max current (%d) %0X: %0 X", phEMCPRegMaxCurrent, current, b)

	return err
}

// ChargingTime yields current charge run duration
func (wb *PhoenixEMCP) ChargingTime() (time.Duration, error) {
	b, err := wb.conn.ReadInputRegisters(phEMCPRegChargeTime, 2)
	wb.log.TRACE.Printf("read charge time (%d): %0 X", phEMCPRegChargeTime, b)
	if err != nil {
		return 0, err
	}

	// 2 words, least significant word first
	secs := uint64(b[3])<<16 | uint64(b[2])<<24 | uint64(b[1]) | uint64(b[0])<<8
	return time.Duration(time.Duration(secs) * time.Second), nil
}
