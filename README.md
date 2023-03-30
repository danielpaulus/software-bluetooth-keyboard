Go Bluetooth Keyboard
====

BT Keyboard Emulator


## Research: 

### Interesting repos:
https://github.com/quangthanh010290/keyboard_mouse_emulate_on_raspberry

https://github.com/benizi/hidclient

http://mulliner.org/bluetooth/xkbdbthid.php

https://github.com/007durgesh219/BTGamepad

explanation about bluez and hidclient: https://askubuntu.com/questions/229287/how-do-i-make-ubuntu-appear-as-a-bluetooth-keyboard

https://github.com/muka/go-bluetooth

emulate a keyboard with go
https://github.com/potch8228/gobt

backboardd logs events

https://github.com/paypal/gatt

### Observations:
the on screen keyboard disappears (might be nice for crawls)
you can toggle the onscreenkeyboard
volume controls, taking screenshots, brightness controsl work, locking screen and unlocking including typing a pin work,
cmd+h == homescreen

### How SDP (Bluetooth Service Discovery Protocol) Records work

Here is some documentation on how to set up this record 
https://www.bluetooth.com/specifications/assigned-numbers/service-discovery/
http://read.pudn.com/downloads25/doc/comm/82404/hid_mouse/hid_mouse.sdp__.htm
https://notes.iopush.net/custom-usb-hid-device-descriptor-media-keyboard/
taken from: https://github.com/lvht/btk/blob/master/sdp_record.xml

