package main

import (
	"github.com/godbus/dbus/v5"
	"github.com/urfave/cli/v2"
	"log"
	"os"
	"strings"
	"time"
)

const (
	DbusService = "fi.w1.wpa_supplicant1"
	DbusPath    = "/fi/w1/wpa_supplicant1"
	DbusIface   = DbusService
)

func get_iface_path(bo dbus.BusObject, ifname string) (obj_iface_path dbus.ObjectPath) {
	if err := bo.Call(DbusIface+".GetInterface", 0, ifname).Store(&obj_iface_path); err != nil {
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

func get_network_interface(c *cli.Context) string {
	args := c.Args()
	if !args.Present() {
		log.Fatal("No interface name given")
	}
	return args.First()
}

func perform_netop(c *cli.Context, conn *dbus.Conn, obj dbus.BusObject) {
	ifname := get_network_interface(c)
	oip := get_iface_path(obj, ifname)
	bo := conn.Object(DbusService, oip)
	cmd := strings.Title(c.Command.Name)
	if err := bo.Call(DbusIface+".Interface."+cmd, 0).Err; err != nil {
		log.Fatal(err)
	}
	log.Println(cmd, "interface", ifname)
}

func list_managed_ifaces(c *cli.Context, conn *dbus.Conn, obj dbus.BusObject) error {
	log.Println("====== Managed interfaces ======")
	if managed_ifaces, err := obj.GetProperty(DbusIface + ".Interfaces"); err == nil {
		for i, iface_opath := range managed_ifaces.Value().([]dbus.ObjectPath) {
			bo := conn.Object(DbusService, iface_opath)
			if ifname, err := bo.GetProperty(DbusIface + ".Interface.Ifname"); err == nil {
				log.Println(i, ifname)
			} else {
				log.Fatal(err)
			}
		}
	} else {
		log.Fatal(err)
	}
	log.Print("Hint: use command ´up´ or ´down´ to integrate or disintegrate an interface")
	return nil
}

func main() {
	conn, err := dbus.SystemBus()
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	obj := conn.Object(DbusService, DbusPath)

	app := &cli.App{
		Version:              "0.0.1",
		EnableBashCompletion: true,
		Authors: []*cli.Author{
			{Name: "Dr. Johann Pfefferl", Email: "pfefferl@gmx.net"},
		},
		Action: func(c *cli.Context) error {
			return list_managed_ifaces(c, conn, obj)
		},
		Usage: "control WPA supplicant through d-bus interface",
		Commands: []*cli.Command{
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Action: func(c *cli.Context) error {
					return list_managed_ifaces(c, conn, obj)
				},
				Usage: "list managed network interfaces",
			},
			{
				Name:    "status",
				Aliases: []string{"st"},
				Action: func(c *cli.Context) error {
					oip := get_iface_path(obj, get_network_interface(c))
					bo := conn.Object(DbusService, oip)
					if state, err := bo.GetProperty(DbusIface + ".Interface.State"); err == nil {
						log.Printf("%-24s %v", "state", state)
					} else {
						log.Fatal(err)
					}
					if cnp, err := bo.GetProperty(DbusIface + ".Interface.CurrentNetwork"); err == nil {
						cnp_opath := cnp.Value().(dbus.ObjectPath)
						if cnp_opath == "/" {
							// Doesn't use given network yet. We are done
							return nil
						}
						nobj := conn.Object(DbusService, cnp_opath)
						if nprops, err := nobj.GetProperty(DbusIface + ".Network.Properties"); err == nil {
							prop_map := nprops.Value().(map[string]dbus.Variant)
							for _, pname := range []string{"ssid", "pairwise", "group", "key_mgmt"} {
								log.Printf("%-24s %s", pname, prop_map[pname].Value().(string))
							}
						} else {
							log.Fatal(err)
						}
					} else {
						log.Fatal(err)
					}
					if cbss, err := bo.GetProperty(DbusIface + ".Interface.CurrentBSS"); err == nil {
						cbss_opath := cbss.Value().(dbus.ObjectPath)
						bssid := get_bss_property(conn, cbss_opath, "BSSID")
						frequency := get_bss_property(conn, cbss_opath, "Frequency")
						mode := get_bss_property(conn, cbss_opath, "Mode")
						signal := get_bss_property(conn, cbss_opath, "Signal")
						privacy := get_bss_property(conn, cbss_opath, "Privacy")
						age := get_bss_property(conn, cbss_opath, "Age")
						log.Printf("%-24s %02x", "bssid", bssid)
						log.Printf("%-24s %v", "mode", mode)
						log.Printf("%-24s %v", "freq", frequency)
						log.Printf("%-24s %v", "signal", signal)
						log.Printf("%-24s %v", "privacy", privacy)
						log.Printf("%-24s %vs", "age", age)
					}
					return nil
				},
				Usage:       "get current WPA/EAPOL/EAP status",
				ArgsUsage:   "<ifname>",
				Description: "Show actual state of the given interface",
			},
			{
				Name:    "up",
				Aliases: []string{"u"},
				Action: func(c *cli.Context) error {
					ci_args := make(map[string]interface{})
					ci_args["Ifname"] = get_network_interface(c)
					ci_args["ConfigFile"] = c.Path("config")
					drv := c.String("driver")
					if len(drv) > 0 {
						ci_args["Driver"] = drv
					}
					brif := c.String("bridge")
					if len(brif) > 0 {
						ci_args["BridgeIfname"] = brif
					}

					if err = obj.Call(DbusIface+".CreateInterface", 0, ci_args).Err; err != nil {
						log.Fatal(err)
					}
					log.Println("Interface", ci_args["Ifname"], "now managed")
					return nil
				},
				Flags: []cli.Flag{
					&cli.PathFlag{
						Name:      "config",
						Aliases:   []string{"c"},
						TakesFile: true,
						Value:     "/etc/wpa_supplicant/wpa_supplicant.conf",
						Usage:     "Configuration file path",
					},
					&cli.StringFlag{
						Name:    "driver",
						Aliases: []string{"D"},
						Usage:   "Driver name which the interface uses, e.g., nl80211",
					},
					&cli.StringFlag{
						Name:    "bridge",
						Aliases: []string{"b"},
						Usage:   "Name of the bridge interface to control, e.g., br0",
					},
				},
				Usage:       "bring up network interface",
				ArgsUsage:   "<ifname>",
				Description: "Integrate the given interface into the WPA supplicant management",
			},
			{
				Name:    "down",
				Aliases: []string{"d"},
				Action: func(c *cli.Context) error {
					ifname := get_network_interface(c)
					oip := get_iface_path(obj, ifname)
					if err = obj.Call(DbusIface+".RemoveInterface", 0, oip).Err; err != nil {
						log.Fatal(err)
					}
					log.Println("Interface", ifname, "no longer managed")
					return nil
				},
				Usage:       "bring down network interface",
				ArgsUsage:   "<ifname>",
				Description: "Disintegrate the given interface into the WPA supplicant management",
			},
			{
				Name:    "scan",
				Aliases: []string{"sc"},
				Action: func(c *cli.Context) error {
					ifname := get_network_interface(c)
					obj_iface_path := get_iface_path(obj, ifname)
					iface_obj := conn.Object(DbusService, obj_iface_path)
					scan_args := make(map[string]interface{})
					scan_args["Type"] = c.String("type")
					scan_args["AllowRoam"] = c.Bool("allow-roam")

					log.Println("Trigger scan on interface", ifname)
					if err = iface_obj.Call(DbusIface+".Interface.Scan", 0, scan_args).Err; err != nil {
						log.Fatal(err)
					}
					return nil
				},
				Usage:     "search for wlan networks on given interface",
				ArgsUsage: "<ifname>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "type",
						Aliases: []string{"t"},
						Value:   "active",
						Usage:   "Type of the scan. Possible values: ´active´, ´passive´",
					},
					&cli.BoolFlag{
						Name:    "allow-roam",
						Aliases: []string{"a"},
						Value:   true,
						Usage:   "´true´ (or absent) to allow a roaming decision based on the results of this scan, ´false´ to prevent a roaming decision.",
					},
				},
			},
			{
				Name:    "scan-results",
				Aliases: []string{"sr", "scr"},
				Action: func(c *cli.Context) error {
					ifname := get_network_interface(c)
					oip := get_iface_path(obj, ifname)
					bo := conn.Object(DbusService, oip)
					for {
						if scan_is_ongoing, err := bo.GetProperty(DbusIface + ".Interface.Scanning"); err == nil {
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
					if v, err := bo.GetProperty(DbusIface + ".Interface.BSSs"); err == nil {
						log.Print("SSID                             BSSID        Freq Sig")
						log.Print("======================================================")
						bss_list := v.Value().([]dbus.ObjectPath)
						for _, bss := range bss_list {
							ssid := string(get_bss_property(conn, bss, "SSID").([]byte))
							bssid := get_bss_property(conn, bss, "BSSID").([]byte)
							freq := get_bss_property(conn, bss, "Frequency").(uint16)
							signal := get_bss_property(conn, bss, "Signal").(int16)
							log.Printf("%-32s %02x %d %d\n", ssid, bssid, freq, signal)
						}
					} else {
						log.Fatal(err)
					}
					return nil
				},
				Usage:       "get latest scan results",
				ArgsUsage:   "<ifname>",
				Description: "Show results of last network scan of given interface",
			},
			{
				Name:    "reconnect",
				Aliases: []string{"rc"},
				Action: func(c *cli.Context) error {
					perform_netop(c, conn, obj)
					return nil
				},
				Usage:       "like reassociate, but only takes effect if already disconnected",
				ArgsUsage:   "<ifname>",
				Description: "Reconnect the given interface",
			},
			{
				Name:    "disconnect",
				Aliases: []string{"dc"},
				Action: func(c *cli.Context) error {
					perform_netop(c, conn, obj)
					return nil
				},
				Usage:       "disconnect and wait for reassociate/reconnect command before",
				ArgsUsage:   "<ifname>",
				Description: "Disconnect the given interface",
			},
			{
				Name:    "reassociate",
				Aliases: []string{"ra"},
				Action: func(c *cli.Context) error {
					perform_netop(c, conn, obj)
					return nil
				},
				Usage:       "force reassociation",
				ArgsUsage:   "<ifname>",
				Description: "Reassociate the given interface",
			},
			{
				Name:    "reattach",
				Aliases: []string{"rat"},
				Action: func(c *cli.Context) error {
					perform_netop(c, conn, obj)
					return nil
				},
				Usage:       "force reassociation back to the same BSS",
				ArgsUsage:   "<ifname>",
				Description: "Reattach the given interface",
			},
		},
	}

	app.Run(os.Args)
}
