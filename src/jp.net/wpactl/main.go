package main

import (
	"fmt"
	"github.com/godbus/dbus/v5"
	"github.com/urfave/cli/v2"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	DbusService = "fi.w1.wpa_supplicant1"
	DbusPath    = "/fi/w1/wpa_supplicant1"
	DbusIface   = DbusService
)

type cliExtended struct {
	// This is a derived object
	*cli.Context
	*dbus.Conn
}

func (ce *cliExtended) get_iface_path(ifname string) (obj_iface_path dbus.ObjectPath) {
	bo := ce.Object(DbusService, DbusPath)
	if err := bo.Call(DbusIface+".GetInterface", 0, ifname).Store(&obj_iface_path); err != nil {
		log.Fatal(err)
	}
	return
}

func (ce *cliExtended) get_bss_property(bss dbus.ObjectPath, prop string) interface{} {
	bss_obj := ce.Object(DbusService, bss)
	propval, err := bss_obj.GetProperty(DbusIface + ".BSS." + prop)
	if err != nil {
		log.Fatal(err)
	}
	return propval.Value()
}

func (ce *cliExtended) get_network_interface() string {
	args := ce.Args()
	if !args.Present() {
		log.Fatal("No interface name given")
	}
	return args.First()
}

func (ce *cliExtended) get_obj_iface_path_of_iface() (string, dbus.ObjectPath) {
	ifname := ce.get_network_interface()
	return ifname, ce.get_iface_path(ifname)
}

func (ce *cliExtended) perform_netop() {
	ifn, oip := ce.get_obj_iface_path_of_iface()
	bo := ce.Object(DbusService, oip)
	cmd := strings.Title(ce.Command.Name)
	if err := bo.Call(DbusIface+".Interface."+cmd, 0).Err; err != nil {
		log.Fatal(err)
	}
	log.Println(cmd, "interface", ifn)
}

func (ce *cliExtended) get_managed_ifaces() []string {
	var result []string
	if managed_ifaces, err := ce.Object(DbusService, DbusPath).GetProperty(DbusIface + ".Interfaces"); err == nil {
		for _, iface_opath := range managed_ifaces.Value().([]dbus.ObjectPath) {
			bo := ce.Object(DbusService, iface_opath)
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
	log.Println("====== Managed interfaces ======")
	for i, iname := range ifnames {
		log.Println(i, iname)
	}
	log.Print("\tHint: use command ´up´ or ´down´ to integrate or disintegrate an interface")
}

func (ce *cliExtended) show_scan_results() {
	_, oip := ce.get_obj_iface_path_of_iface()
	bo := ce.Object(DbusService, oip)
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
			ssid := string(ce.get_bss_property(bss, "SSID").([]byte))
			bssid := ce.get_bss_property(bss, "BSSID").([]byte)
			freq := ce.get_bss_property(bss, "Frequency").(uint16)
			signal := ce.get_bss_property(bss, "Signal").(int16)
			log.Printf("%-32s %02x %d %d\n", ssid, bssid, freq, signal)
		}
	} else {
		log.Fatal(err)
	}
}

func (ce *cliExtended) get_network_properties(netobj dbus.ObjectPath) (retval map[string]dbus.Variant) {
	bo := ce.Object(DbusService, netobj)
	if nwprops, err := bo.GetProperty(DbusIface + ".Network.Properties"); err == nil {
		retval = nwprops.Value().(map[string]dbus.Variant)
	} else {
		log.Fatal(err)
	}
	return
}

func (ce *cliExtended) network_get_obj_list() []dbus.ObjectPath {
	_, oip := ce.get_obj_iface_path_of_iface()
	bo := ce.Object(DbusService, oip)
	if nwlist, err := bo.GetProperty(DbusIface + ".Interface.Networks"); err == nil {
		return nwlist.Value().([]dbus.ObjectPath)
	} else {
		log.Fatal(err)
	}
	return nil
}

func (ce *cliExtended) network_show_list() {
	log.Printf("Id SSID                             Dis")
	log.Printf("=======================================")
	for i, nwobj := range ce.network_get_obj_list() {
		nprops := ce.get_network_properties(nwobj)
		log.Printf("% 2d %-32v %-3v", i, nprops["ssid"].Value(), nprops["disabled"])
	}
}

func (ce *cliExtended) network_set_state(state bool) {
	_, oip := ce.get_obj_iface_path_of_iface()
	bo := ce.Object(DbusService, oip)
	if nwlist, err := bo.GetProperty(DbusIface + ".Interface.Networks"); err == nil {
		to_change_id := ce.Int("id")
		for i, nwobj := range nwlist.Value().([]dbus.ObjectPath) {
			if i == to_change_id {
				bo = ce.Object(DbusService, nwobj)
				if err = bo.SetProperty(DbusService+".Network.Enabled", dbus.MakeVariant(state)); err != nil {
					log.Fatal(err)
				}
				break
			}
		}
		if ce.Bool("results") {
			ce.network_show_list()
		}
	} else {
		log.Fatal(err)
	}
}

func (ce *cliExtended) show_status() {
	ifn, oip := ce.get_obj_iface_path_of_iface()
	bo := ce.Object(DbusService, oip)
	log.Print("Interface status")
	log.Print("================")
	if state, err := bo.GetProperty(DbusIface + ".Interface.State"); err == nil {
		log.Printf("%-16s %s", "interface", ifn)
		log.Printf("%-16s %v", "dbus interface", oip)
		log.Printf("%-16s %v", "state", state)
	} else {
		log.Fatal(err)
	}
	if cbss, err := bo.GetProperty(DbusIface + ".Interface.CurrentBSS"); err == nil {
		cbss_opath := cbss.Value().(dbus.ObjectPath)
		/* Check if interface is really associated with a BSS */
		if cbss_opath != "/" {
			ssid := string(ce.get_bss_property(cbss_opath, "SSID").([]uint8))
			bssid := ce.get_bss_property(cbss_opath, "BSSID")
			frequency := ce.get_bss_property(cbss_opath, "Frequency")
			mode := ce.get_bss_property(cbss_opath, "Mode")
			signal := ce.get_bss_property(cbss_opath, "Signal")
			privacy := ce.get_bss_property(cbss_opath, "Privacy")
			age := ce.get_bss_property(cbss_opath, "Age")
			rsn := ce.get_bss_property(cbss_opath, "RSN").(map[string]dbus.Variant)
			log.Printf("%-16s %02x", "bssid", bssid)
			log.Printf("%-16s %v", "freq", frequency)
			log.Printf("%-16s %v", "ssid", ssid)
			log.Printf("%-16s %v", "mode", mode)
			log.Printf("%-16s %v", "pairwise cipher", rsn["Pairwise"])
			log.Printf("%-16s %v", "group cipher", rsn["Group"])
			log.Printf("%-16s %v", "key mgmt", rsn["KeyMgmt"])
			log.Printf("%-16s %v", "signal", signal)
			log.Printf("%-16s %v", "privacy", privacy)
			log.Printf("%-16s %vs", "age", age)
			if iface, err := net.InterfaceByName(ifn); err == nil {
				if addrlist, err := iface.Addrs(); err == nil {
					for idx, name := range addrlist {
						log.Printf("%-16s %s", "ipaddr"+strconv.Itoa(idx), name)
					}
				} else {
					log.Fatal(err)
				}
			} else {
				log.Fatal(err)
			}
		}
	}
}

func main() {
	conn, err := dbus.SystemBus()
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	ce := cliExtended{}
	ce.Conn = conn

	app := &cli.App{
		Version:              "0.0.2",
		EnableBashCompletion: true,
		Authors: []*cli.Author{
			{Name: "Dr. Johann Pfefferl", Email: "pfefferl@gmx.net"},
		},
		Action: func(c *cli.Context) error {
			ce.Context = c
			list_ifaces(ce.get_managed_ifaces())
			return nil
		},
		Usage: "control WPA supplicant through d-bus interface",
		Commands: []*cli.Command{
			{
				Name: "interface",
				Action: func(c *cli.Context) error {
					ce.Context = c
					list_ifaces(ce.get_managed_ifaces())
					return nil
				},
				Usage: "list managed network interfaces",
			},
			{
				Name:    "status",
				Aliases: []string{"st"},
				Action: func(c *cli.Context) error {
					ce.Context = c
					ce.show_status()
					return nil
				},
				Usage:       "get current WPA/EAPOL/EAP status",
				ArgsUsage:   "<ifname>",
				Description: "Show actual state of the given interface",
			},
			{
				Name: "up",
				Action: func(c *cli.Context) error {
					ce.Context = c
					ci_args := make(map[string]interface{})
					ci_args["Ifname"] = ce.get_network_interface()
					ci_args["ConfigFile"] = ce.Path("config")
					drv := ce.String("driver")
					if len(drv) > 0 {
						ci_args["Driver"] = drv
					}
					brif := ce.String("bridge")
					if len(brif) > 0 {
						ci_args["BridgeIfname"] = brif
					}

					if err = ce.Object(DbusService, DbusPath).Call(DbusIface+".CreateInterface", 0, ci_args).Err; err != nil {
						log.Fatal(err)
					}
					log.Println("Interface", ci_args["Ifname"], "now managed")
					return nil
				},
				Flags: []cli.Flag{
					&cli.PathFlag{
						Name:      "config",
						Aliases:   []string{"ce"},
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
					ce.Context = c
					ifname, oip := ce.get_obj_iface_path_of_iface()
					if err = ce.Object(DbusService, DbusPath).Call(DbusIface+".RemoveInterface", 0, oip).Err; err != nil {
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
					ce.Context = c
					ifn, oip := ce.get_obj_iface_path_of_iface()
					iface_obj := ce.Object(DbusService, oip)
					scan_args := make(map[string]interface{})
					scan_args["Type"] = ce.String("type")
					scan_args["AllowRoam"] = ce.Bool("allow-roam")

					log.Println("Trigger scan on interface", ifn)
					if err = iface_obj.Call(DbusIface+".Interface.Scan", 0, scan_args).Err; err != nil {
						log.Fatal(err)
					}
					if ce.Bool("results") {
						ce.show_scan_results()
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
					ce.Context = c
					ce.show_scan_results()
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
					ce.Context = c
					ce.perform_netop()
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
					ce.Context = c
					ce.perform_netop()
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
					ce.Context = c
					ce.perform_netop()
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
					ce.Context = c
					ce.perform_netop()
					return nil
				},
				Usage:       "force reassociation back to the same BSS",
				ArgsUsage:   "<ifname>",
				Description: "Reattach the given interface",
			},
			{
				Name: "signal_poll",
				Action: func(c *cli.Context) error {
					ce.Context = c
					_, oip := ce.get_obj_iface_path_of_iface()
					bo := ce.Object(DbusService, oip)
					var siginfo map[string]dbus.Variant
					if err := bo.Call(DbusIface+".Interface.SignalPoll", 0).Store(&siginfo); err != nil {
						log.Fatal(err)
					}
					for signame, sigval := range siginfo {
						log.Printf("%-10s %v", signame, sigval)
					}
					return nil
				},
				Usage:     "get signal parameters",
				ArgsUsage: "<ifname>",
			},
			{
				Name: "networks",
				Subcommands: []*cli.Command{
					{
						Name:    "list",
						Aliases: []string{"ls"},
						Action: func(c *cli.Context) error {
							ce.Context = c
							ce.network_show_list()
							return nil
						},
						Usage:       "list configured networks",
						ArgsUsage:   "<ifname>",
						Description: "Report networks which are defined in the config files or added afterwards for the given interface",
					},
					{
						Name: "disable",
						Action: func(c *cli.Context) error {
							ce.Context = c
							ce.network_set_state(false)
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
							ce.Context = c
							ce.network_set_state(true)
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
							ce.Context = c
							_, oip := ce.get_obj_iface_path_of_iface()
							bo := ce.Object(DbusService, oip)
							to_remove_id := ce.Int("id")
							if ce.Bool("all") {
								if err := bo.Call(DbusIface+".Interface.RemoveAllNetworks", 0).Err; err != nil {
									log.Fatal(err)
								}
							} else {
								for idx, netobj := range ce.network_get_obj_list() {
									if idx == to_remove_id {
										if err := bo.Call(DbusIface+".Interface.RemoveNetwork", 0, netobj).Err; err != nil {
											log.Fatal(err)
										}
										break
									}
								}
							}
							if ce.Bool("results") {
								ce.network_show_list()
							}
							return nil
						},
						Usage:       "remove a network entry",
						ArgsUsage:   "<ifname>",
						Description: "Remove the network by given index",
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:    "id",
								Aliases: []string{"i"},
								Value:   -1,
								Usage:   "Id number of the network to remove",
							},
							&cli.BoolFlag{
								Name:  "all",
								Usage: "Remove all configured networks from the interface",
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
							ce.Context = c
							_, oip := ce.get_obj_iface_path_of_iface()
							bo := ce.Object(DbusService, oip)
							to_select_id := ce.Int("id")
							for idx, netobj := range ce.network_get_obj_list() {
								if idx == to_select_id {
									if err := bo.Call(DbusIface+".Interface.SelectNetwork", 0, netobj).Err; err != nil {
										log.Fatal(err)
									}
									break
								}
							}
							if ce.Bool("results") {
								ce.network_show_list()
							}
							if ce.Bool("status") {
								ce.show_status()
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
							ce.Context = c
							_, oip := ce.get_obj_iface_path_of_iface()
							bo := ce.Object(DbusService, oip)
							add_args := make(map[string]interface{})
							s := ce.String("ssid")
							if len(s) > 0 {
								add_args["ssid"] = s
							}
							psk := ce.String("psk")
							if len(psk) > 0 {
								add_args["psk"] = psk
							}
							if ce.Bool("disabled") {
								add_args["disabled"] = 1
							} else {
								add_args["disabled"] = 0
							}
							proto := ce.String("proto")
							if len(proto) > 0 {
								add_args["proto"] = proto
							}
							km := ce.String("key_mgmt")
							if len(km) > 0 {
								add_args["key_mgmt"] = km
							}
							var result string
							if err := bo.Call(DbusIface+".Interface.AddNetwork", 0, add_args).Store(&result); err != nil {
								log.Fatal(err)
							}
							if ce.Bool("results") {
								ce.network_show_list()
							}
							return nil
						},
						Usage:     "add a network entry",
						ArgsUsage: "<ifname>",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "ssid",
								Usage: "SSID of the entry",
							},
							&cli.StringFlag{
								Name:  "psk",
								Usage: "Preshared key (aka. password)",
							},
							&cli.StringFlag{
								Name:  "proto",
								Usage: "list of accepted protocols",
							},
							&cli.StringFlag{
								Name:  "key_mgmt",
								Usage: "Key management method",
							},
							&cli.UintFlag{
								Name:  "ieee80211w",
								Usage: "management frame protection mode (0: disabled, 1: optional, 2: required)",
							},
							&cli.BoolFlag{
								Name:  "disabled",
								Value: false,
								Usage: "Initial state of the entry",
							},
							&cli.BoolFlag{
								Name:  "results",
								Usage: "Show resulting network list",
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
