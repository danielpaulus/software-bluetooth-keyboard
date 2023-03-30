package main

import (
	"bytes"
	"io/ioutil"

	"os"
	"os/signal"

	gobt "github.com/danielpaulus/software-bluetooth-keyboard"
	"github.com/danielpaulus/software-bluetooth-keyboard/api"
	"github.com/danielpaulus/software-bluetooth-keyboard/bluetooth"
	"github.com/danielpaulus/software-bluetooth-keyboard/hid"
	"github.com/godbus/dbus"
	uuid "github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
)

func main() {
	adapter := hid.NewBluetoothKeyboardAdapter()
	go api.StartServer(adapter)

	log.SetLevel(log.DebugLevel)
	connIntr, err := bluetooth.Listen(bluetooth.PSMINTR, 1, false)
	if err != nil {
		log.Fatal("Listen failed", err, bluetooth.PSMINTR)
	}

	hidp := gobt.NewHidProfile("/red/potch/profile", connIntr, adapter)

	conn, err := dbus.SystemBus()
	if err != nil {
		log.Fatal("Failed to connect to system bus", err)
	}

	if err := conn.Export(hidp, hidp.Path(), "org.bluez.Profile1"); err != nil {
		log.Fatal(err)
	}
	log.Debug("org.bluez.Profile1 exported")

	s, err := os.Open("./sdp_record.xml")
	if err != nil {
		log.Fatal(err)
	}
	sdp, err := ioutil.ReadAll(s)
	if err != nil {
		log.Fatal(err)
	}

	//Adding AutoConnect here does not make the device auto connect
	opts := map[string]dbus.Variant{
		"PSM":                   dbus.MakeVariant(uint16(bluetooth.PSMCTRL)),
		"RequireAuthentication": dbus.MakeVariant(true),
		"RequireAuthorization":  dbus.MakeVariant(true),
		"ServiceRecord":         dbus.MakeVariant(bytes.NewBuffer(sdp).String()),
	}
	uid := uuid.NewV4()

	dObjCh := make(chan *dbus.Call, 1)
	dObj := conn.Object("org.bluez", "/org/bluez")
	regObjCall := dObj.Go("org.bluez.ProfileManager1.RegisterProfile", 0, dObjCh, hidp.Path(), uid.String(), opts)
	log.Debug(regObjCall)
	var r interface{}
	if regObjCall.Err != nil {
		log.Fatal(regObjCall.Store(&r), r, regObjCall.Err)
	}
	log.Debug("HID Profile registered")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	evloop := true
	for evloop {
		select {
		case dObjCall := <-dObjCh:
			if dObjCall.Err != nil {
				log.Debug(dObjCall.Err)
				evloop = false
			}
		case <-sig:
			log.Debug("Will Quit Program")
			evloop = false
		default:
		}
	}

	// Probably no need of closing profile
	log.Debug("Trying to Close Profile")
	unregObjCall := dObj.Call("org.bluez.ProfileManager1.UnregisterProfile", 0, hidp.Path())
	log.Debug(unregObjCall)
	if unregObjCall.Err != nil {
		log.Debug(unregObjCall.Store(&r), r, regObjCall.Err)
	}
	log.Debug("HID Profile unregistered", "Trying to Destroy Profile Obj")
	hidp.Close()

	close(dObjCh)
	conn.Close()
}
