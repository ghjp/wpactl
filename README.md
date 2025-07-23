# wpactl

## Introduction

Control WPA supplicant through d-bus interface

Call `wpactl --help` to get an overview of possible commands.

Shell (bash) completion is also supported to simplify handling.

To build the application execute `make`. To cross compile the tool for ARM execute `make ARCH=arm`.

## Debian packages

To generate a Debian binary package for the actual platform call the following

`make deb`

To build a package for armv7 machines execute the following

`make deb ARCH=arm`

If you want to use the tool as a normal user the user must be part of the group ´netdev´. This can be done by `adduser username netdev`.

## Usage

### Infrastructure mode

This mode is the normal one. You connect to a wlan access point (AP).

#### Add any WLAN without password authentification

`wpactl networks add --key_mgmt NONE --prio 1 wlan0`

#### Add a WLAN with a preshared key (aka password/phasephrase)

The first example is the widely used procedure WPA2 auth method

`wpactl networks add --ssid NetworkAP --key_mgmt WPA-PSK --pairwise CCMP --psk pw12345678 wlan0`

The newer and more secure WPA3-personal auth method is configured as following

`wpactl networks add --ssid NetworkAP --key_mgmt SAE --ieee80211w 2 --sae_password pw12345678 wlan0`

#### IEEE 802.1X using EAP authentication

`wpactl networks add --key_mgmt IEEE8021X --eap TLS --identity host/myhost.example.com --client_cert mycert.pem --private_key TopSecret wlan0`

### Access point mode (AP)

With this mode you can create your own wlan access point. The wlan network card must support this mode.

`wpactl networks add --ssid MyOwnWlanNet --mode 2 --frequency 2432 --key_mgmt WPA-PSK --pairwise CCMP --proto RSN --psk 1234567890  wlan0`

## Activation

After loading all the network configurations into the wpa_supplicant daemon, you can trigger the wpa_supplicant service to perform a network connection.

`wpactl reassociate wlan0`
