// Package internal provides shared cryptographic primitives and hardware
// identity functions for vault-t2.
//
// Bounded context: Tier-2 secrets — hardware-sealed, no network auth required.
// The hardware fingerprint is the root of trust for all sealing operations.
package internal

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
)

// ReadHardwareFingerprint derives a 32-byte hardware fingerprint for this
// specific Raspberry Pi by combining two stable hardware identifiers:
//
//   - /etc/machine-id  — a stable UUID assigned at first boot
//   - /proc/cpuinfo Serial — the SoC serial number burned in at manufacture
//
// The fingerprint is:
//
//	SHA-256( machine_id || ":" || cpu_serial )
//
// This fingerprint is used as the AES-256 key when sealing the tier-2 seed,
// so secrets are irrecoverably lost if both identifiers change (e.g. full OS
// reinstall on different hardware) unless the seed was backed up.
func ReadHardwareFingerprint() ([]byte, error) {
	machineID, err := readMachineID()
	if err != nil {
		return nil, fmt.Errorf("reading machine-id: %w", err)
	}

	cpuSerial, err := readCPUSerial()
	if err != nil {
		return nil, fmt.Errorf("reading cpu serial: %w", err)
	}

	combined := machineID + ":" + cpuSerial
	digest := sha256.Sum256([]byte(combined))
	return digest[:], nil
}

// readMachineID reads and trims /etc/machine-id.
// This file is a 32-character lowercase hex string assigned at first boot by
// systemd-machine-id-setup and is stable across reboots.
func readMachineID() (string, error) {
	raw, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		return "", fmt.Errorf("open /etc/machine-id: %w", err)
	}
	id := strings.TrimSpace(string(raw))
	if id == "" {
		return "", fmt.Errorf("/etc/machine-id is empty")
	}
	return id, nil
}

// readCPUSerial parses the "Serial" field from /proc/cpuinfo.
// On Raspberry Pi hardware this is the unique 16-character hex serial burned
// into the SoC at manufacture. It is absent on x86 and most VMs — callers
// should handle the not-found case as an error for this Pi-specific tool.
func readCPUSerial() (string, error) {
	raw, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return "", fmt.Errorf("open /proc/cpuinfo: %w", err)
	}

	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "Serial") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				serial := strings.TrimSpace(parts[1])
				if serial != "" {
					return serial, nil
				}
			}
		}
	}

	return "", fmt.Errorf("Serial field not found in /proc/cpuinfo (is this a Raspberry Pi?)")
}
