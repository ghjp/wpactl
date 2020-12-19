package main

import (
	"flag"
	"github.com/godbus/dbus/v5"
	"log"
)

var (
	optIface   = flag.String("i", "wlxf0b0148be468", "WLAN network interface name")
	optMode    = flag.String("m", "scan", "mode of operation: ´scan´, ´up´ or ´down´")
	optVerbose = flag.Bool("v", false, "Be verbose")
)

const (
	iface        = "wlxf0b0148be468"
	dbus_service = "fi.w1.wpa_supplicant1"
	dbus_path    = "/fi/w1/wpa_supplicant1"
	dbus_iface   = dbus_service
)

func main() {
	flag.Parse()

	conn, err := dbus.SystemBus()
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	obj := conn.Object(dbus_service, dbus_path)
	var obj_iface_path dbus.ObjectPath
	if err = obj.Call(dbus_iface+".GetInterface", 0, *optIface).Store(&obj_iface_path); err == nil {
		log.Println(obj_iface_path)
		iface_obj := conn.Object(dbus_service, obj_iface_path)
		switch *optMode {
		case "scan":
			scan_args := make(map[string]interface{})
			scan_args["Type"] = "active"
			//scan_args["AllowRoam"] = true

			if err = iface_obj.Call(dbus_iface+".Interface.Scan", 0, scan_args).Err; err != nil {
				log.Fatal(err)
			}
		case "up":
			log.Print("UP")
		case "down":
			log.Print("DOWN")
		default:
			log.Fatal("Wrong mode specified")
		}
	} else {
		log.Fatal(err)
	}
}
