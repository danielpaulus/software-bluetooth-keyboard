package hid

import (
	"fmt"
	"strings"
	"sync"

	"github.com/danielpaulus/software-bluetooth-keyboard/bluetooth"

	log "github.com/sirupsen/logrus"
)

type DeviceEventCtrl byte

const (
	DEVEVCBQUIT DeviceEventCtrl = iota
)

type DeviceError struct {
	msg    string
	method string
}

func (de *DeviceError) Error() string {
	return fmt.Sprintf("DeviceError: '%s' by method: %s", de.msg, de.method)
}

/*
BTKeyboard HID Report structure
[
	0xA1, # This is an input report at first byte field
	0x02, # Usage report(exclusive for bt) = BTKeyboard
	# Bit array for Modifier keys (D7 being the first element, D0 being last)
	[
		0,   # Right GUI - (usually the Windows key)
		0,   # Right ALT
		0,   # Right Shift
		0,   # Right Control
		0,   # Left GUI - (again, usually the Windows key)
		0,   # Left ALT
		0,   # Left Shift
		0    # Left Control
	],
	0x00, # Vendor reserved
	0x00, # Rest is space for 6 keys
	0x00,
	0x00,
	0x00,
	0x00,
	0x00
]
*/

type KeyboardInput struct {
	Message  string
	Modifier string
}

type KeyboardStatus struct {
	IsReady bool
}

type Keyboard interface {
	TypeText(keyInput string) error
	TypeKey(keyInput string) error
	Status() KeyboardStatus
}

type BluetoothKeyboardAdapter struct {
	mux          sync.Mutex
	btConnection *bluetooth.Bluetooth
	status       KeyboardStatus
}

func NewBluetoothKeyboardAdapter() *BluetoothKeyboardAdapter {
	return &BluetoothKeyboardAdapter{mux: sync.Mutex{}, status: KeyboardStatus{false}}
}
func (ba *BluetoothKeyboardAdapter) TypeText(keyinput string) error {
	log.Infof("Start sending text '%s'", keyinput)
	for _, c := range keyinput {
		characterKey := strings.ToUpper(fmt.Sprintf("KEY_%c", c))
		log.Infof("Sending key %s", characterKey)
		SendKey(ba.btConnection, characterKey)
	}
	return nil
}

func (ba *BluetoothKeyboardAdapter) TypeKey(keyinput string) error {
	if !IsSupported(keyinput) {
		return fmt.Errorf("Unsupported key: %s", keyinput)
	}
	log.Infof("Sending key %s", keyinput)
	SendKey(ba.btConnection, keyinput)

	return nil
}

func (ba *BluetoothKeyboardAdapter) Status() KeyboardStatus {
	return ba.status
}
func (ba *BluetoothKeyboardAdapter) SetBtConnection(bt *bluetooth.Bluetooth) {
	ba.mux.Lock()
	defer ba.mux.Unlock()
	ba.btConnection = bt
	ba.status.IsReady = true

}

func SendKey(btConnection *bluetooth.Bluetooth, characterKey string) {
	state := make([]byte, 10)
	state[0] = 0xA1
	state[1] = 0x02
	changeState(characterKey, state, true, false)
	log.Debugf("%x", state)
	if _, err := btConnection.Write(state); err != nil {
		log.Debug("Failure on Sending Keyboard State")
		return
	}
	state = make([]byte, 10)
	state[0] = 0xA1
	state[1] = 0x02

	log.Debugf("%x", state)
	if _, err := btConnection.Write(state); err != nil {
		log.Debug("Failure on Sending Keyboard State")
		return
	}
	log.Printf("Sending Keyboard State Done")
}

func changeState(characterKeyName string, state []byte, keyDown bool, keyUp bool) error {

	key, mkey := Convert(characterKeyName)
	keycode := uint16(key)
	log.Debug(key)
	log.Debug(mkey)
	var err error = nil
	switch mkey {
	case MOD:
		err = updateModifiers(keycode, state, true)
	case FUNC:
		updateStates(keycode, state, true, false)
	}

	return err
}

func updateModifiers(keycode uint16, state []byte, keyDown bool) error {
	if keycode > 8 { // length of 8 bits
		return &DeviceError{msg: "bitpos(kev.Keycode) > 8", method: "updateModifiers()"}
	}

	if keyDown {
		state[2] |= byte(1 << keycode)
		return nil
	}
	state[2] &= byte(^(1 << keycode))

	return nil
}

func updateStates(keycode uint16, state []byte, keyDown bool, keyUp bool) {
	for i := 4; i < len(state); i++ {
		switch {
		case keyUp && byte(keycode) == state[i]:
			state[i] = 0x00
		case keyDown && state[i] == 0x00:
			state[i] = byte(keycode)
			return
		}
	}
}
