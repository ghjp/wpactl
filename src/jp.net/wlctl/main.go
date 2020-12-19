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
	optCfgFile = flag.String("c", "/etc/wpa_supplicant/wpa_supplicant.conf", "Configuration file path")
)

const (
	DbusService = "fi.w1.wpa_supplicant1"
	DbusPath    = "/fi/w1/wpa_supplicant1"
	DbusIface   = DbusService
)

func get_iface_path(bo dbus.BusObject) (obj_iface_path dbus.ObjectPath) {
	if err := bo.Call(DbusIface+".GetInterface", 0, *optIface).Store(&obj_iface_path); err != nil {
		log.Fatal(err)
	}
	return
}

func main() {
	flag.Parse()

	conn, err := dbus.SystemBus()
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	obj := conn.Object(DbusService, DbusPath)
	switch *optMode {
	case "scan":
		obj_iface_path := get_iface_path(obj)
		iface_obj := conn.Object(DbusService, obj_iface_path)
		scan_args := make(map[string]interface{})
		scan_args["Type"] = "active"
		//scan_args["AllowRoam"] = true

		log.Println("Trigger scan on interface", *optIface)
		if err = iface_obj.Call(DbusIface+".Interface.Scan", 0, scan_args).Err; err != nil {
			log.Fatal(err)
		}
	case "up":
		ci_args := make(map[string]interface{})
		ci_args["Ifname"] = *optIface
		ci_args["ConfigFile"] = *optCfgFile
		if err = obj.Call(DbusIface+".CreateInterface", 0, ci_args).Err; err != nil {
			log.Fatal(err)
		}
		log.Println("Interface", *optIface, "now managed")
	case "down":
		oip := get_iface_path(obj)
		if err = obj.Call(DbusIface+".RemoveInterface", 0, oip).Err; err != nil {
			log.Fatal(err)
		}
		log.Println("Interface", *optIface, "no longer managed")
	default:
		log.Fatal("Wrong mode specified")
	}
}
