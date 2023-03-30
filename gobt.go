package gobt

import (
	"time"

	"github.com/danielpaulus/software-bluetooth-keyboard/bluetooth"
	"github.com/danielpaulus/software-bluetooth-keyboard/hid"
	log "github.com/sirupsen/logrus"
)

type GoBtPollState byte

const (
	STOP GoBtPollState = iota
)

const (
	HIDPHEADERTRANSMASK = 0xf0

	HIDPTRANSHANDSHAKE   = 0x00
	HIDPTRANSSETPROTOCOL = 0x60
	HIDPTRANSDATA        = 0xa0

	HIDPHSHKSUCCESSFUL = 0x00
	HIDPHSHKERRUNKNOWN = 0x0e
)

type GoBt struct {
	sintr *bluetooth.Bluetooth
	sctrl *bluetooth.Bluetooth

	cctl            chan GoBtPollState
	keyboardAdapter *hid.BluetoothKeyboardAdapter
}

func NewGoBt(sintr, sctrl *bluetooth.Bluetooth, keyboardAdapter *hid.BluetoothKeyboardAdapter) *GoBt {
	gobt := GoBt{
		sintr:           sintr,
		sctrl:           sctrl,
		cctl:            make(chan GoBtPollState, 2),
		keyboardAdapter: keyboardAdapter,
	}

	keyboardAdapter.SetBtConnection(sintr)

	log.Debug("Sending hello on ctrl channel")
	if _, err := gobt.sctrl.Write([]byte{0xa1, 0x13, 0x03}); err != nil {
		log.Debug("Failure on Sending Hello on Ctrl 1", err)
		return nil
	}
	if _, err := gobt.sctrl.Write([]byte{0xa1, 0x13, 0x02}); err != nil {
		log.Debug("Failure on Sending Hello on Ctrl 2", err)
		return nil
	}
	time.Sleep(1 * time.Second)

	go gobt.startProcessCtrlEvent()
	return &gobt
}

func (gb *GoBt) startProcessCtrlEvent() {
	for {
		select {
		case <-gb.cctl:
			log.Debug("Will Quit GoBt Process loop")
			return
		default:
			r := make([]byte, bluetooth.BUFSIZE)
			d, err := gb.sctrl.Read(r)
			if err != nil || d < 1 {
				log.Debug("GoBt.procesCtrlEvent: no data received - quitting event loop")
				gb.Close()
				return
			}

			hsk := []byte{HIDPTRANSHANDSHAKE}
			msgTyp := r[0] & HIDPHEADERTRANSMASK

			switch {
			case (msgTyp & HIDPTRANSSETPROTOCOL) != 0:
				log.Debug("GoBt.procesCtrlEvent: handshake set protocol")
				hsk[0] |= HIDPHSHKSUCCESSFUL
				if _, err := gb.sctrl.Write(hsk); err != nil {
					log.Debug("GoBt.procesCtrlEvent: handshake set protocol: failure on reply")
				}
			case (msgTyp & HIDPTRANSDATA) != 0:
				log.Debug("GoBt.procesCtrlEvent: handshake data")
			default:
				log.Debug("GoBt.procesCtrlEvent: unknown handshake message")
				hsk[0] |= HIDPHSHKERRUNKNOWN
				gb.sctrl.Write(hsk)
			}
		}
	}
}

func (gb *GoBt) Close() {

	log.Debug("Stopped HIDevices")

	log.Debug("Trying to Stop GoBt evevnt loop")
	gb.cctl <- STOP
}
