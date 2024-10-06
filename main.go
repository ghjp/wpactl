package main

import (
	"fmt"
	"github.com/godbus/dbus/v5"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"log"
	"net"
	"os"
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
	fmt.Println(cmd, "interface", ifn)
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
	fmt.Println("====== Managed interfaces ======")
	for i, iname := range ifnames {
		fmt.Println(i, iname)
	}
	fmt.Println("\tHint: use command ´up´ or ´down´ to integrate or disintegrate a link")
}

func (ce *cliExtended) show_scan_results() {
	sigch := make(chan *dbus.Signal, 16)
	if err := ce.AddMatchSignal(dbus.WithMatchInterface(DbusService + ".Interface")); err != nil {
		log.Fatal(err)
	}
	ce.Signal(sigch)
	defer ce.RemoveSignal(sigch)
	_, oip := ce.get_obj_iface_path_of_iface()
	bo := ce.Object(DbusService, oip)
	if scan_is_ongoing, err := bo.GetProperty(DbusIface + ".Interface.Scanning"); err == nil {
		if scan_is_ongoing.Value().(bool) {
			fmt.Print("Interface is still scanning. Waiting ")
		ScanWaitLoop:
			for {
				select {
				case sig := <-sigch:
					if sig.Name == DbusService+".Interface.ScanDone" && sig.Body[0].(bool) {
						fmt.Println(" done")
						break ScanWaitLoop
					} else {
						fmt.Print("+")
					}
				case <-time.After(2 * time.Second):
					fmt.Print("-")
				}
			}
		}
	} else {
		log.Fatal(err)
	}
	if v, err := bo.GetProperty(DbusIface + ".Interface.BSSs"); err == nil {
		fmt.Println("SSID                             BSSID        Freq Sig Age Flags")
		fmt.Println("================================================================")
		bss_list := v.Value().([]dbus.ObjectPath)
		for _, bss := range bss_list {
			ssid := string(ce.get_bss_property(bss, "SSID").([]byte))
			bssid := ce.get_bss_property(bss, "BSSID").([]byte)
			freq := ce.get_bss_property(bss, "Frequency").(uint16)
			signal := ce.get_bss_property(bss, "Signal").(int16)
			age := ce.get_bss_property(bss, "Age").(uint32)
			rsn := ce.get_bss_property(bss, "RSN").(map[string]dbus.Variant)
			fmt.Printf("%-32s %02x %d %d %3v %v %v\n", ssid, bssid, freq, signal, age, rsn["KeyMgmt"].Value(), rsn["Pairwise"].Value())
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
	long_listing := ce.Bool("long")
	var header string
	if long_listing {
		header = "Id SSID                             Dis dbus-obj-path\n====================================================="
	} else {
		header = "Id SSID                             Dis\n======================================="
	}
	fmt.Println(header)
	for i, nwobj := range ce.network_get_obj_list() {
		nprops := ce.get_network_properties(nwobj)
		if long_listing {
			fmt.Printf("% 2d %-32v %-3v %v\n", i, nprops["ssid"].Value(), nprops["disabled"], nwobj)
		} else {
			fmt.Printf("% 2d %-32v %-3v\n", i, nprops["ssid"].Value(), nprops["disabled"])
		}
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
	fmt.Println("Interface status")
	fmt.Println("================")
	if state, err := bo.GetProperty(DbusIface + ".Interface.State"); err == nil {
		fmt.Printf("%-16s %s\n", "interface", ifn)
		fmt.Printf("%-16s %v\n", "dbus interface", oip)
		fmt.Printf("%-16s %v\n", "state", state)
	} else {
		log.Fatal(err)
	}
	if cam, err := bo.GetProperty(DbusIface + ".Interface.CurrentAuthMode"); err == nil {
		fmt.Printf("%-16v %v\n", "auth mode", cam.Value())
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
			fmt.Printf("%-16s %02x\n", "bssid", bssid)
			fmt.Printf("%-16s %v\n", "freq", frequency)
			fmt.Printf("%-16s %v\n", "ssid", ssid)
			fmt.Printf("%-16s %v\n", "mode", mode)
			fmt.Printf("%-16s %v\n", "pairwise cipher", rsn["Pairwise"])
			fmt.Printf("%-16s %v\n", "group cipher", rsn["Group"])
			fmt.Printf("%-16s %v\n", "key mgmt", rsn["KeyMgmt"])
			fmt.Printf("%-16s %v\n", "signal", signal)
			fmt.Printf("%-16s %v\n", "privacy", privacy)
			fmt.Printf("%-16s %vs\n", "age", age)
		}
	}
	if iface, err := net.InterfaceByName(ifn); err == nil {
		if addrlist, err := iface.Addrs(); err == nil {
			for idx, name := range addrlist {
				fmt.Printf("%-16s %s\n", "ipaddr"+strconv.Itoa(idx), name)
			}
		} else {
			log.Fatal(err)
		}
	} else {
		log.Fatal(err)
	}
}

func (ce *cliExtended) set_interface_property(name string, value interface{}) {
	_, oip := ce.get_obj_iface_path_of_iface()
	bo := ce.Object(DbusService, oip)
	if err := bo.SetProperty(DbusIface+".Interface."+name, dbus.MakeVariant(value)); err != nil {
		log.Fatal(err)
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
				Subcommands: []*cli.Command{
					{
						Name: "list",
						Action: func(c *cli.Context) error {
							ce.Context = c
							list_ifaces(ce.get_managed_ifaces())
							return nil
						},
						Usage: "list managed networks",
					},
					{
						Name: "set",
						Action: func(c *cli.Context) error {
							ce.Context = c
							if apscan := ce.Int("ap_scan"); apscan >= 0 {
								ce.set_interface_property("ApScan", uint32(apscan))
							}
							if country := ce.String("country"); len(country) > 0 {
								ce.set_interface_property("Country", country)
							}
							return nil
						},
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:  "ap_scan",
								Usage: "AP scanning/selection: 0, 1 or 2 are valid values",
								Value: -1,
							},
							&cli.StringFlag{
								Name:  "country",
								Usage: "The ISO/IEC alpha2 country code",
							},
						},
						Usage:     "set properties",
						ArgsUsage: "<ifname>",
					},
				},
				Usage: "list managed network interfaces",
			},
			{
				Name:    "status",
				Aliases: []string{"st"},
				Action: func(c *cli.Context) error {
					ce.Context = c
					ce.show_status()
					for loop := c.Duration("loop"); loop > 0; {
						hour, min, sec := time.Now().Clock()
						fmt.Printf("clock            %02v:%02v:%02v\n", hour, min, sec)
						time.Sleep(loop)
						ce.show_status()
					}
					return nil
				},
				Usage:       "get current WPA/EAPOL/EAP status",
				ArgsUsage:   "<ifname>",
				Description: "Show actual state of the given interface",
				Flags: []cli.Flag{
					&cli.DurationFlag{
						Name:  "loop",
						Usage: "periodically print the status",
					},
				},
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
					fmt.Println("Interface", ci_args["Ifname"], "now managed")
					return nil
				},
				Flags: []cli.Flag{
					&cli.PathFlag{
						Name:      "config",
						TakesFile: true,
						Value:     os.DevNull,
						Usage:     "Configuration file path",
					},
					&cli.StringFlag{
						Name:  "driver",
						Usage: "Driver name which the interface uses, e.g. ´nl80211´ or ´wired´",
					},
					&cli.StringFlag{
						Name:  "bridge",
						Usage: "Name of the bridge interface to control, e.g. br0",
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
					fmt.Println("Interface", ifname, "no longer managed")
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

					fmt.Println("Trigger scan on interface", ifn)
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
						fmt.Printf("%-10s %v\n", signame, sigval)
					}
					return nil
				},
				Usage:     "get signal parameters",
				ArgsUsage: "<ifname>",
			},
			{
				Name: "flush_bss",
				Action: func(c *cli.Context) error {
					ce.Context = c
					_, oip := ce.get_obj_iface_path_of_iface()
					bo := ce.Object(DbusService, oip)
					if err := bo.Call(DbusIface+".Interface.FlushBSS", 0, c.Uint("age")).Err; err != nil {
						log.Fatal(err)
					}
					return nil
				},
				Usage:     "Flush BSS entries from the cache",
				ArgsUsage: "<ifname>",
				Flags: []cli.Flag{
					&cli.UintFlag{
						Name:  "age",
						Usage: "Maximum age in seconds for BSS entries to keep in cache (0 = remove all entries)",
					},
				},
			},
			{
				Name: "networks",
				Subcommands: []*cli.Command{
					{
						Name: "list",
						Action: func(c *cli.Context) error {
							ce.Context = c
							ce.network_show_list()
							return nil
						},
						Usage:       "list configured networks",
						ArgsUsage:   "<ifname>",
						Description: "Report networks which are defined in the config files or added afterwards for the given interface",
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:  "long",
								Usage: "use a long listing format",
							},
						},
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
							to_remove_ssid := `"` + ce.String("ssid") + `"`
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
									} else if to_remove_ssid != `""` {
										nprops := ce.get_network_properties(netobj)
										if nprops["ssid"].Value() == to_remove_ssid {
											if err := bo.Call(DbusIface+".Interface.RemoveNetwork", 0, netobj).Err; err != nil {
												log.Fatal(err)
											}
										}
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
							&cli.StringFlag{
								Name:  "ssid",
								Usage: "SSID of the network to remove",
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
							for _, s := range []string{"psk", "ssid", "bssid", "proto", "key_mgmt", "pairwise", "eap", "identity", "client_cert", "private_key", "private_key_passwd", "sae_password"} {
								v := ce.String(s)
								if len(v) > 0 {
									add_args[s] = v
								}
							}
							add_args["disabled"] = 0
							if c.Bool("disabled") {
								add_args["disabled"] = 1
							}
							add_args["ieee80211w"] = c.Uint("ieee80211w")
							add_args["priority"] = c.Uint("prio")
							add_args["mode"] = c.Uint("mode")
							freq := c.Uint("frequency")
							if freq > 0 {
								add_args["frequency"] = freq
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
								Name:  "bssid",
								Usage: "BSSID of the entry",
							},
							&cli.StringFlag{
								Name:  "psk",
								Usage: "Preshared key (aka. password)",
							},
							&cli.StringFlag{
								Name:  "sae_password",
								Usage: "SAE password",
							},
							&cli.StringFlag{
								Name:  "proto",
								Usage: "list of accepted protocols",
							},
							&cli.StringFlag{
								Name:  "key_mgmt",
								Usage: "Key management method",
							},
							&cli.StringFlag{
								Name:  "pairwise",
								Usage: "list of accepted pairwise (unicast) ciphers for WPA",
							},
							&cli.UintFlag{
								Name:  "frequency",
								Usage: "channel frequency in megahertz",
							},
							&cli.UintFlag{
								Name:  "mode",
								Usage: "IEEE 802.11 operation mode: 0=infrastructure, 1=IBSS, 2=AP",
							},
							&cli.UintFlag{
								Name:  "ieee80211w",
								Value: 3, /* MGMT_FRAME_PROTECTION_DEFAULT */
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
							&cli.StringFlag{
								Name:  "eap",
								Usage: "space-separated list of accepted EAP methods",
							},
							&cli.StringFlag{
								Name:  "identity",
								Usage: "identity string for EAP",
							},
							&cli.StringFlag{
								Name:  "client_cert",
								Usage: "file path to client certificate file (PEM/DER)",
							},
							&cli.StringFlag{
								Name:  "private_key",
								Usage: "file path to client private key file (PEM/DER/PFX)",
							},
							&cli.StringFlag{
								Name:  "private_key_passwd",
								Usage: "password for private key file",
							},
							&cli.UintFlag{
								Name:  "prio",
								Usage: "priority group",
							},
						},
					},
				},
				Usage:     "operation on configured networks",
				ArgsUsage: "<ifname>",
			},
			{
				Name:      "blob",
				Usage:     "manage blobs",
				ArgsUsage: "<ifname>",
				Subcommands: []*cli.Command{
					{
						Name: "list",
						Action: func(c *cli.Context) error {
							ce.Context = c
							_, oip := ce.get_obj_iface_path_of_iface()
							bo := ce.Object(DbusService, oip)
							if bloblist, err := bo.GetProperty(DbusIface + ".Interface.Blobs"); err == nil {
								blobmap := bloblist.Value().(map[string][]uint8)
								if ce.Bool("no-legend") {
									for bname, _ := range blobmap {
										fmt.Println(bname)
									}
								} else {
									fmt.Println("Name                             Length\n========================================")
									for bname, bdata := range blobmap {
										fmt.Printf("%-32s %d\n", bname, len(bdata))
									}
								}
							} else {
								log.Fatal(err)
							}
							return nil
						},
						Usage:     "show list of added blobs",
						ArgsUsage: "<ifname>",
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:  "no-legend",
								Usage: "Do not show the headers and footers",
							},
						},
					},
					{
						Name: "add",
						Action: func(c *cli.Context) error {
							ce.Context = c
							_, oip := ce.get_obj_iface_path_of_iface()
							bo := ce.Object(DbusService, oip)
							if content, err := ioutil.ReadFile(ce.Path("data")); err == nil {
								if err := bo.Call(DbusIface+".Interface.AddBlob", 0, ce.String("name"), content).Err; err != nil {
									log.Fatal(err)
								}
							} else {
								log.Fatal(err)
							}
							return nil
						},
						Usage:     "adds a blob to the interface.",
						ArgsUsage: "<ifname>",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Required: true,
								Usage:    "identifier of the blob",
							},
							&cli.PathFlag{
								Name:      "data",
								TakesFile: true,
								Required:  true,
								Usage:     "file name containing the data",
							},
						},
					},
					{
						Name: "remove",
						Action: func(c *cli.Context) error {
							ce.Context = c
							_, oip := ce.get_obj_iface_path_of_iface()
							bo := ce.Object(DbusService, oip)
							if err := bo.Call(DbusIface+".Interface.RemoveBlob", 0, ce.String("name")).Err; err != nil {
								log.Fatal(err)
							}
							return nil
						},
						Usage:     "remove a blob from the interface.",
						ArgsUsage: "<ifname>",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Required: true,
								Usage:    "identifier of the blob",
							},
						},
					},
					{
						Name: "get",
						Action: func(c *cli.Context) error {
							ce.Context = c
							_, oip := ce.get_obj_iface_path_of_iface()
							bo := ce.Object(DbusService, oip)
							var blobdata []byte
							if err = bo.Call(DbusIface+".Interface.GetBlob", 0, ce.String("name")).Store(&blobdata); err != nil {
								log.Fatal(err)
							}
							if err = ioutil.WriteFile(ce.Path("output"), blobdata, 0664); err != nil {
								log.Fatal(err)
							}
							return nil
						},
						Usage:     "get the data from a previously added blob",
						ArgsUsage: "<ifname>",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:     "name",
								Required: true,
								Usage:    "identifier of the blob",
							},
							&cli.PathFlag{
								Name:      "output",
								TakesFile: true,
								Required:  true,
								Usage:     "output file name",
							},
						},
					},
				},
			},
			{
				Name: "monitor",
				Action: func(c *cli.Context) error {
					ce.Context = c
					sigch := make(chan *dbus.Signal, 4)
					if err := ce.AddMatchSignal(dbus.WithMatchInterface(DbusService + ".Interface")); err != nil {
						log.Fatal(err)
					}
					ce.Signal(sigch)
					defer ce.RemoveSignal(sigch)
					for sig := range sigch {
						log.Println(sig)
					}
					return nil
				},
				Usage:       "show dbus signals for all managed interfaces",
				Description: "this is a raw dump of the signals sent by wpa_supplicant",
			},
		},
	}

	app.RunAndExitOnError()
}
