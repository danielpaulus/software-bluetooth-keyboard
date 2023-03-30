package gobt

import (
	"fmt"

	"golang.org/x/sys/unix"

	"github.com/danielpaulus/software-bluetooth-keyboard/bluetooth"
	"github.com/danielpaulus/software-bluetooth-keyboard/hid"
	"github.com/godbus/dbus"
	log "github.com/sirupsen/logrus"
)

//https://git.kernel.org/pub/scm/bluetooth/bluez.git/tree/doc/profile-api.txt
type HidProfile struct {
	path dbus.ObjectPath

	gb map[dbus.ObjectPath]*GoBt

	connIntr *bluetooth.Bluetooth

	sintr           *bluetooth.Bluetooth
	sctrl           *bluetooth.Bluetooth
	keyboardAdapter *hid.BluetoothKeyboardAdapter
}

func NewHidProfile(path string, connIntr *bluetooth.Bluetooth, keyboardAdapter *hid.BluetoothKeyboardAdapter) *HidProfile {
	return &HidProfile{
		path:            (dbus.ObjectPath)(path),
		gb:              make(map[dbus.ObjectPath]*GoBt),
		connIntr:        connIntr,
		keyboardAdapter: keyboardAdapter,
	}
}

func (p *HidProfile) Path() dbus.ObjectPath {
	return p.path
}

func (p *HidProfile) Release() *dbus.Error {
	log.Debug("Release")
	return nil
}

func (p *HidProfile) NewConnection(dev dbus.ObjectPath, fd dbus.UnixFD, fdProps map[string]dbus.Variant) *dbus.Error {
	log.Debug("NewConnection", dev, fd, fdProps)

	var err error
	p.sintr, err = p.connIntr.Accept()
	if err != nil {
		p.connIntr.Close()
		log.Debug("Accept failed", err, bluetooth.PSMINTR)
		return dbus.NewError(fmt.Sprintf("Accept failed: %v", bluetooth.PSMINTR), []interface{}{err})
	}
	log.Debug("Connection Accepted", bluetooth.PSMINTR)

	p.sctrl, err = bluetooth.NewBluetoothSocket(int(fd))
	if err != nil {
		_err := unix.Close(int(fd))

		if _err != nil {
			log.Debug("NewBluetoothSocket closing fd failed", _err)
			return dbus.NewError("NewBluetoothSocket closing fd failed", []interface{}{_err})
		}
		log.Debug("NewBluetoothSocket failed", err, fd, fdProps)
		return dbus.NewError(fmt.Sprintf("NewBluetoothSocket failed: %v, %v, %v", err, fd, fdProps), []interface{}{err})
	}
	log.Debug("Created New Ctrl Socket")

	p.gb[dev] = NewGoBt(p.sintr, p.sctrl, p.keyboardAdapter)
	return nil
}

func (p *HidProfile) RequestDisconnection(dev dbus.ObjectPath) *dbus.Error {
	log.Debug("RequestDisconnection", dev)
	p.gb[dev].Close()
	return nil
}

func (p *HidProfile) Close() {
	log.Debug("Hid Profile will close")
	for k := range p.gb {
		p.gb[k].Close()
		p.gb[k] = nil
	}

	if p.sintr != nil {
		p.sintr.Close()
	}

	if p.sctrl != nil {
		p.sctrl.Close()
	}
}
