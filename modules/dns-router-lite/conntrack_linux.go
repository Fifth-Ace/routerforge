//go:build linux

package lite

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"
)

type netlinkCTTable struct{ sequence atomic.Uint32 }

func newSystemConntrack() *ConntrackCleaner { return &ConntrackCleaner{Table: &netlinkCTTable{}} }
func (t *netlinkCTTable) exchange(ctx context.Context, kind uint16, family byte, attrs []byte, dump bool) ([]CTFlow, error) {
	ctx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()
	fd, err := unix.Socket(unix.AF_NETLINK, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, unix.NETLINK_NETFILTER)
	if err != nil {
		return nil, fmt.Errorf("conntrack netlink socket unavailable: %w", err)
	}
	defer unix.Close(fd)
	if err = unix.Bind(fd, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return nil, err
	}
	seq := t.sequence.Add(1)
	flags := uint16(unix.NLM_F_REQUEST | unix.NLM_F_ACK)
	if dump {
		flags = unix.NLM_F_REQUEST | unix.NLM_F_DUMP
	}
	b := make([]byte, 20+len(attrs))
	binary.NativeEndian.PutUint32(b, uint32(len(b)))
	binary.NativeEndian.PutUint16(b[4:], 0x100|kind)
	binary.NativeEndian.PutUint16(b[6:], flags)
	binary.NativeEndian.PutUint32(b[8:], seq)
	b[16] = family
	copy(b[20:], attrs)
	if err = unix.Sendto(fd, b, 0, &unix.SockaddrNetlink{Family: unix.AF_NETLINK}); err != nil {
		return nil, err
	}
	flows := []CTFlow{}
	buffer := make([]byte, 65536)
	total := 0
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		n, e := unix.Poll(poll, 100)
		if e == unix.EINTR {
			continue
		}
		if e != nil {
			return nil, e
		}
		if n == 0 {
			continue
		}
		n, from, e := unix.Recvfrom(fd, buffer, 0)
		if e == unix.EAGAIN || e == unix.EINTR {
			continue
		}
		if e != nil {
			return nil, e
		}
		peer, ok := from.(*unix.SockaddrNetlink)
		if !ok || peer.Pid != 0 {
			return nil, errors.New("untrusted conntrack netlink sender")
		}
		total += n
		if total > 16<<20 {
			return nil, errors.New("conntrack dump exceeds bounded scan limit")
		}
		data := buffer[:n]
		for len(data) > 0 {
			if len(data) < 16 {
				return nil, errors.New("short conntrack netlink header")
			}
			length := int(binary.NativeEndian.Uint32(data))
			if length < 16 || align4(length) > len(data) {
				return nil, errors.New("invalid netlink message size")
			}
			typ := binary.NativeEndian.Uint16(data[4:])
			msgFlags := binary.NativeEndian.Uint16(data[6:])
			messageSeq := binary.NativeEndian.Uint32(data[8:])
			payload := data[16:length]
			data = data[align4(length):]
			if messageSeq != seq {
				continue
			}
			if msgFlags&unix.NLM_F_DUMP_INTR != 0 {
				return nil, errors.New("conntrack dump interrupted; no verified snapshot")
			}
			if typ == unix.NLMSG_DONE {
				if len(payload) >= 4 && int32(binary.NativeEndian.Uint32(payload)) < 0 {
					return nil, errors.New("incomplete conntrack dump")
				}
				return flows, nil
			}
			if typ == unix.NLMSG_ERROR {
				if len(payload) < 4 {
					return nil, errors.New("short netlink error")
				}
				code := int32(binary.NativeEndian.Uint32(payload))
				if code == 0 {
					return flows, nil
				}
				if !dump && code == -int32(unix.ENOENT) {
					return flows, nil
				}
				return nil, fmt.Errorf("conntrack netlink: %w", unix.Errno(-code))
			}
			if dump && typ == 0x100 {
				f, e := parseCTFlow(payload)
				if e != nil {
					return nil, e
				}
				// Keep DNS flows only; streaming the dump has bounded resident memory.
				if f.Original.DstPort == 53 && (f.Original.Proto == 6 || f.Original.Proto == 17) {
					if len(flows) >= 4096 {
						return nil, errors.New("too many DNS conntrack flows")
					}
					flows = append(flows, f)
				}
			}
		}
	}
}
func (t *netlinkCTTable) Dump(ctx context.Context) ([]CTFlow, error) {
	return t.exchange(ctx, 1, unix.AF_UNSPEC, nil, true)
}
func (t *netlinkCTTable) Delete(ctx context.Context, f CTFlow) error {
	// Missing tuple/ID is forbidden: the kernel interprets a tuple-less DELETE
	// as a global flush. CTA_ID also prevents deleting a reused five-tuple.
	parsed, err := parseCTFlow(append([]byte{f.Family, 0, 0, 0}, f.DeleteAttrs...))
	if err != nil || len(parsed.ID) != 4 {
		return errors.New("refusing unsafe conntrack delete")
	}
	_, err = t.exchange(ctx, 2, f.Family, f.DeleteAttrs, false)
	return err
}
