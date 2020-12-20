package main

import (
	"flag"
	"github.com/godbus/dbus/v5"
	"log"
	"time"
)

var (
	optIface   = flag.String("i", "wlxf0b0148be468", "WLAN network interface name")
	optMode    = flag.String("m", "scan", "mode of operation: ´scan´, ´scan-results´, ´up´ or ´down´")
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

func get_bss_property(conn *dbus.Conn, bss dbus.ObjectPath, prop string) interface{} {
	bss_obj := conn.Object(DbusService, bss)
	propval, err := bss_obj.GetProperty(DbusIface + ".BSS." + prop)
	if err != nil {
		log.Fatal(err)
	}
	return propval.Value()
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
	case "scan-results":
		oip := get_iface_path(obj)
		o := conn.Object(DbusService, oip)
		for {
			if scan_is_ongoing, err := o.GetProperty(DbusIface + ".Interface.Scanning"); err == nil {
				if scan_is_ongoing.Value().(bool) {
					log.Print("Interface is still scanning. Waiting ...")
					time.Sleep(2 * time.Second)
				} else {
					break
				}
			} else {
				log.Fatal(err)
			}
		}
		if v, err := o.GetProperty(DbusIface + ".Interface.BSSs"); err == nil {
			log.Print("SSID                             BSSID             Freq Sig")
			log.Print("===========================================================")
			bss_list := v.Value().([]dbus.ObjectPath)
			for _, bss := range bss_list {
				ssid := string(get_bss_property(conn, bss, "SSID").([]byte))
				bssid := get_bss_property(conn, bss, "BSSID").([]byte)
				freq := get_bss_property(conn, bss, "Frequency").(uint16)
				signal := get_bss_property(conn, bss, "Signal").(int16)
				log.Printf("%-32s %02x:%02x:%02x:%02x:%02x:%02x %d %d\n", ssid, bssid[0], bssid[1], bssid[2], bssid[3], bssid[4], bssid[5], freq, signal)
			}
		} else {
			log.Fatal(err)
		}
	default:
		log.Fatal("Wrong mode specified")
	}
}
