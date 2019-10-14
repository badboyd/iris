package main

import (
	"errors"
	"net"
	"sync"
	"time"
)

const (
	idLen          = 63 // int64
	timestampLen   = 39 // atm we use 38 bits (10ms) for timestamp, it tooks hundred years to get full 2^39-1
	sequenceLen    = 8  // per 10ms we can have most 255 id
	machineIDlen   = 16 // second half of ip address for host machine run this program
	maskSequence   = uint16(1<<sequenceLen - 1)
	maskMachineID  = uint64(1<<machineIDlen - 1)
	maskIDSequence = uint64((1<<sequenceLen - 1) << machineIDlen)
)

type idGenerator struct {
	m         sync.Mutex
	sequence  uint8 // no needs mask
	machineID uint16
}

func newIDGenerator() (*idGenerator, error) {
	machineID, err := lower16BitPrivateIP()
	if err != nil {
		return nil, err
	}

	return &idGenerator{
		sequence:  0,
		machineID: machineID,
	}, nil
}

func (i *idGenerator) next() uint64 {
	i.m.Lock()
	defer i.m.Unlock()

	// increase the sequence
	i.sequence++

	// 10ms
	// hopefully we won't process more than 255 image per 10ms per ip
	// LOL 255/10ms = 25500/s
	t := time.Now().UnixNano() / int64(10*time.Millisecond)

	return uint64(t)<<(sequenceLen+machineIDlen) |
		uint64(i.sequence)<<machineIDlen | uint64(i.machineID)
}

func decompose(id uint64) map[string]uint64 {
	msb := id >> 63
	time := id >> (sequenceLen + machineIDlen)
	sequence := id & maskIDSequence >> machineIDlen
	machineID := id & maskMachineID

	return map[string]uint64{
		"id":         id,
		"msb":        msb,
		"time":       time,
		"sequence":   sequence,
		"machine_id": machineID,
	}
}

func privateIPv4() (net.IP, error) {
	as, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, a := range as {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}

		ip := ipnet.IP.To4()
		if isPrivateIPv4(ip) {
			return ip, nil
		}
	}
	return nil, errors.New("no private ip address")
}

func isPrivateIPv4(ip net.IP) bool {
	return ip != nil &&
		(ip[0] == 10 || ip[0] == 172 && (ip[1] >= 16 && ip[1] < 32) || ip[0] == 192 && ip[1] == 168)
}

func lower16BitPrivateIP() (uint16, error) {
	ip, err := privateIPv4()
	if err != nil {
		return 0, err
	}

	return uint16(ip[2])<<8 + uint16(ip[3]), nil
}
