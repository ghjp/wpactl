package main

import (
	"fmt"
	"github.com/godbus/dbus/v5"
	"github.com/urfave/cli/v2"
	"log"
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

func get_obj_iface_path_of_iface(c *cli.Context, obj dbus.BusObject) (string, dbus.ObjectPath) {
	ifname := get_network_interface(c)
	return ifname, get_iface_path(obj, ifname)
}

func perform_netop(c *cli.Context, conn *dbus.Conn, obj dbus.BusObject) {
	ifn, oip := get_obj_iface_path_of_iface(c, obj)
	bo := conn.Object(DbusService, oip)
	cmd := strings.Title(c.Command.Name)
	if err := bo.Call(DbusIface+".Interface."+cmd, 0).Err; err != nil {
		log.Fatal(err)
	}
	log.Println(cmd, "interface", ifn)
}

func get_managed_ifaces(c *cli.Context, conn *dbus.Conn, obj dbus.BusObject) []string {
	var result []string
	log.Println("====== Managed interfaces ======")
	if managed_ifaces, err := obj.GetProperty(DbusIface + ".Interfaces"); err == nil {
		for _, iface_opath := range managed_ifaces.Value().([]dbus.ObjectPath) {
			bo := conn.Object(DbusService, iface_opath)
			if ifname, err := bo.GetProperty(DbusIface + ".Interface.Ifname"); err == nil {
				if state, err := bo.GetProperty(DbusIface + ".Interface.State"); err == nil {
					line := fmt.Sprintf("%-20v %v", ifname.Value(), state.Value())
					result = append(result, line)
				} else {
					log.Fatal(err)
				}
			} else {
				log.Fatal(err)
			}
		}
	} else {
		log.Fatal(err)
	}
	return result
}

func list_ifaces(ifnames []string) {
	for i, iname := range ifnames {
		log.Println(i, iname)
	}
	log.Print("\tHint: use command ´up´ or ´down´ to integrate or disintegrate an interface")
}

func show_scan_results(c *cli.Context, conn *dbus.Conn, obj dbus.BusObject) {
	_, oip := get_obj_iface_path_of_iface(c, obj)
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
}

func get_network_properties(c *cli.Context, conn *dbus.Conn, netobj dbus.ObjectPath) (retval map[string]dbus.Variant) {
	bo := conn.Object(DbusService, netobj)
	if nwprops, err := bo.GetProperty(DbusIface + ".Network.Properties"); err == nil {
		retval = nwprops.Value().(map[string]dbus.Variant)
	} else {
		log.Fatal(err)
	}
	return
}

func network_get_obj_list(c *cli.Context, conn *dbus.Conn, obj dbus.BusObject) []dbus.ObjectPath {
	_, oip := get_obj_iface_path_of_iface(c, obj)
	bo := conn.Object(DbusService, oip)
	if nwlist, err := bo.GetProperty(DbusIface + ".Interface.Networks"); err == nil {
		return nwlist.Value().([]dbus.ObjectPath)
	} else {
		log.Fatal(err)
	}
	return nil
}

func network_show_list(c *cli.Context, conn *dbus.Conn, obj dbus.BusObject) {
	log.Printf("Id SSID                             Dis")
	log.Printf("=======================================")
	for i, nwobj := range network_get_obj_list(c, conn, obj) {
		nprops := get_network_properties(c, conn, nwobj)
		log.Printf("% 2d %-32v %-3v", i, nprops["ssid"].Value(), nprops["disabled"])
	}
}

func network_set_state(c *cli.Context, conn *dbus.Conn, obj dbus.BusObject, state bool) {
	_, oip := get_obj_iface_path_of_iface(c, obj)
	bo := conn.Object(DbusService, oip)
	if nwlist, err := bo.GetProperty(DbusIface + ".Interface.Networks"); err == nil {
		to_change_id := c.Int("id")
		for i, nwobj := range nwlist.Value().([]dbus.ObjectPath) {
			if i == to_change_id {
				bo = conn.Object(DbusService, nwobj)
				if err = bo.SetProperty(DbusService+".Network.Enabled", dbus.MakeVariant(state)); err != nil {
					log.Fatal(err)
				}
				break
			}
		}
		if c.Bool("results") {
			network_show_list(c, conn, obj)
		}
	} else {
		log.Fatal(err)
	}
}

func show_status(c *cli.Context, conn *dbus.Conn, obj dbus.BusObject) {
	ifn, oip := get_obj_iface_path_of_iface(c, obj)
	bo := conn.Object(DbusService, oip)
	log.Print("Interface status")
	log.Print("================")
	if state, err := bo.GetProperty(DbusIface + ".Interface.State"); err == nil {
		log.Printf("%-24s %s", "interface", ifn)
		log.Printf("%-24s %v", "dbus interface", oip)
		log.Printf("%-24s %v", "state", state)
	} else {
		log.Fatal(err)
	}
	if cnp, err := bo.GetProperty(DbusIface + ".Interface.CurrentNetwork"); err == nil {
		cnp_opath := cnp.Value().(dbus.ObjectPath)
		if cnp_opath == "/" {
			// Doesn't use given network yet. We are done
			return
		}
		prop_map := get_network_properties(c, conn, cnp_opath)
		for _, pname := range []string{"ssid", "pairwise", "group", "key_mgmt"} {
			log.Printf("%-24s %s", pname, prop_map[pname].Value().(string))
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
			list_ifaces(get_managed_ifaces(c, conn, obj))
			return nil
		},
		Usage: "control WPA supplicant through d-bus interface",
		Commands: []*cli.Command{
			{
				Name: "interface",
				Action: func(c *cli.Context) error {
					list_ifaces(get_managed_ifaces(c, conn, obj))
					return nil
				},
				Usage: "list managed network interfaces",
			},
			{
				Name:    "status",
				Aliases: []string{"st"},
				Action: func(c *cli.Context) error {
					show_status(c, conn, obj)
					return nil
				},
				Usage:       "get current WPA/EAPOL/EAP status",
				ArgsUsage:   "<ifname>",
				Description: "Show actual state of the given interface",
			},
			{
				Name: "up",
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
				Aliases: []string{"dn"},
				Action: func(c *cli.Context) error {
					ifname, oip := get_obj_iface_path_of_iface(c, obj)
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
					ifn, oip := get_obj_iface_path_of_iface(c, obj)
					iface_obj := conn.Object(DbusService, oip)
					scan_args := make(map[string]interface{})
					scan_args["Type"] = c.String("type")
					scan_args["AllowRoam"] = c.Bool("allow-roam")

					log.Println("Trigger scan on interface", ifn)
					if err = iface_obj.Call(DbusIface+".Interface.Scan", 0, scan_args).Err; err != nil {
						log.Fatal(err)
					}
					if c.Bool("results") {
						show_scan_results(c, conn, obj)
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
					&cli.BoolFlag{
						Name:    "results",
						Aliases: []string{"r"},
						Value:   false,
						Usage:   "Wait and show scan results",
					},
				},
			},
			{
				Name:    "scan-results",
				Aliases: []string{"sr", "scr"},
				Action: func(c *cli.Context) error {
					show_scan_results(c, conn, obj)
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
			{
				Name: "signal_poll",
				Action: func(c *cli.Context) error {
					_, oip := get_obj_iface_path_of_iface(c, obj)
					bo := conn.Object(DbusService, oip)
					var siginfo map[string]dbus.Variant
					if err := bo.Call(DbusIface+".Interface.SignalPoll", 0).Store(&siginfo); err != nil {
						log.Fatal(err)
					}
					for signame, sigval := range siginfo {
						log.Printf("%-10s %v", signame, sigval)
					}
					return nil
				},
				Usage:       "force reassociation back to the same BSS",
				ArgsUsage:   "<ifname>",
				Description: "Reattach the given interface",
			},
			{
				Name: "networks",
				Subcommands: []*cli.Command{
					{
						Name:    "list",
						Aliases: []string{"ls"},
						Action: func(c *cli.Context) error {
							network_show_list(c, conn, obj)
							return nil
						},
						Usage:       "list configured networks",
						ArgsUsage:   "<ifname>",
						Description: "Report networks which are defined in the config files or added afterwards for the given interface",
					},
					{
						Name: "disable",
						Action: func(c *cli.Context) error {
							network_set_state(c, conn, obj, false)
							return nil
						},
						Usage:       "disable a network entry",
						ArgsUsage:   "<ifname>",
						Description: "Disable the network by given index name",
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:     "id",
								Aliases:  []string{"i"},
								Required: true,
								Usage:    "Id number of the network to disable",
							},
							&cli.BoolFlag{
								Name:    "results",
								Aliases: []string{"r"},
								Usage:   "Show resulting network list",
							},
						},
					},
					{
						Name: "enable",
						Action: func(c *cli.Context) error {
							network_set_state(c, conn, obj, true)
							return nil
						},
						Usage:       "enable a network entry",
						ArgsUsage:   "<ifname>",
						Description: "Enable the network by given index",
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:     "id",
								Aliases:  []string{"i"},
								Required: true,
								Usage:    "Id number of the network to enable",
							},
							&cli.BoolFlag{
								Name:    "results",
								Aliases: []string{"r"},
								Usage:   "Show resulting network list",
							},
						},
					},
					{
						Name: "remove",
						Action: func(c *cli.Context) error {
							_, oip := get_obj_iface_path_of_iface(c, obj)
							bo := conn.Object(DbusService, oip)
							to_remove_id := c.Int("id")
							for idx, netobj := range network_get_obj_list(c, conn, obj) {
								if idx == to_remove_id {
									if err := bo.Call(DbusIface+".Interface.RemoveNetwork", 0, netobj).Err; err != nil {
										log.Fatal(err)
									}
									break
								}
							}
							if c.Bool("results") {
								network_show_list(c, conn, obj)
							}
							return nil
						},
						Usage:       "remove a network entry",
						ArgsUsage:   "<ifname>",
						Description: "Remove the network by given index",
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:     "id",
								Aliases:  []string{"i"},
								Required: true,
								Usage:    "Id number of the network to remove",
							},
							&cli.BoolFlag{
								Name:    "results",
								Aliases: []string{"r"},
								Usage:   "Show resulting network list",
							},
						},
					},
					{
						Name: "select",
						Action: func(c *cli.Context) error {
							_, oip := get_obj_iface_path_of_iface(c, obj)
							bo := conn.Object(DbusService, oip)
							to_select_id := c.Int("id")
							for idx, netobj := range network_get_obj_list(c, conn, obj) {
								if idx == to_select_id {
									if err := bo.Call(DbusIface+".Interface.SelectNetwork", 0, netobj).Err; err != nil {
										log.Fatal(err)
									}
									break
								}
							}
							if c.Bool("results") {
								network_show_list(c, conn, obj)
							}
							if c.Bool("status") {
								show_status(c, conn, obj)
							}
							return nil
						},
						Usage:       "select a network entry and disable the others",
						ArgsUsage:   "<ifname>",
						Description: "Select the network by given index. The others are disabled automatically",
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:     "id",
								Aliases:  []string{"i"},
								Required: true,
								Usage:    "Id number of the network to select",
							},
							&cli.BoolFlag{
								Name:    "results",
								Aliases: []string{"r"},
								Usage:   "Show resulting network list",
							},
							&cli.BoolFlag{
								Name:    "status",
								Aliases: []string{"s"},
								Value:   false,
								Usage:   "Show status of interface",
							},
						},
					},
					{
						Name: "add",
						Action: func(c *cli.Context) error {
							_, oip := get_obj_iface_path_of_iface(c, obj)
							bo := conn.Object(DbusService, oip)
							add_args := make(map[string]interface{})
							s := c.String("ssid")
							if len(s) > 0 {
								add_args["ssid"] = s
							}
							psk := c.String("psk")
							if len(psk) > 0 {
								add_args["psk"] = c.String("psk")
							}
							if c.Bool("disabled") {
								add_args["disabled"] = 1
							} else {
								add_args["disabled"] = 0
							}
							var result string
							if err := bo.Call(DbusIface+".Interface.AddNetwork", 0, add_args).Store(&result); err != nil {
								log.Fatal(err)
							}
							log.Print(result)
							return nil
						},
						Usage:     "add a network entry",
						ArgsUsage: "<ifname>",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "ssid",
								Aliases: []string{"s"},
								Usage:   "SSID of the entry",
							},
							&cli.StringFlag{
								Name:    "psk",
								Aliases: []string{"password", "pw", "p"},
								Usage:   "Preshared key (aka. password)",
							},
							&cli.BoolFlag{
								Name:  "disabled",
								Value: false,
								Usage: "Initial state of the entry",
							},
							&cli.BoolFlag{
								Name:    "results",
								Aliases: []string{"r"},
								Usage:   "Show resulting network list",
							},
						},
					},
				},
				Usage:     "operation on configured networks",
				ArgsUsage: "<ifname>",
			},
		},
	}

	app.RunAndExitOnError()
}
