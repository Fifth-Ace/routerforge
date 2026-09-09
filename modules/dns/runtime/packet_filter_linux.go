package main

import (
	"fmt"
	"syscall"
)

func attachPacketFilter(fd int, name string, program []classicBPFInstruction, log *EventLogger) {
	if len(program) == 0 {
		if log != nil {
			log.Event("CAPTURE_BPF", fmt.Sprintf("%s filter is empty; continuing unfiltered", name))
		}
		return
	}

	filters := make([]syscall.SockFilter, len(program))
	for i, instruction := range program {
		filters[i] = syscall.SockFilter{
			Code: instruction.Code,
			Jt:   instruction.Jt,
			Jf:   instruction.Jf,
			K:    instruction.K,
		}
	}

	if err := syscall.AttachLsf(fd, filters); err != nil {
		if log != nil {
			log.Event("CAPTURE_BPF", fmt.Sprintf("%s attach failed: %v; continuing unfiltered", name, err))
		}
		return
	}

	if log != nil {
		log.Event("CAPTURE_BPF", fmt.Sprintf("%s attached / instructions=%d", name, len(filters)))
	}
}
