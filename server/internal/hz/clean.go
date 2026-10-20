package hz

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"
)

// dnsblZones are the blocklists consulted for IPv4 reputation.
var dnsblZones = []string{
	"zen.spamhaus.org",
	"bl.spamcop.net",
	"b.barracudacentral.org",
	"cbl.abuseat.org",
	"dnsbl.sorbs.net",
	"dnsbl-1.uceprotect.net",
	"psbl.surriel.com",
}

// CleanResult is the outcome of a DNSBL reputation check for one IPv4 address.
type CleanResult struct {
	IP       string   `json:"ip"`
	Clean    bool     `json:"clean"`
	Listed   []string `json:"listed,omitempty"`
	Unknown  []string `json:"unknown,omitempty"`
	Answered int      `json:"answered"`
	At       int64    `json:"at"`
}

// CheckIPv4 looks an IPv4 address up on several DNSBL zones concurrently.
// An address is considered clean when no zone lists it and at least 4 zones
// actually answered (so a total DNS outage never marks an IP as clean).
func CheckIPv4(ip string) CleanResult {
	res := CleanResult{IP: ip, At: time.Now().Unix()}
	parts := strings.Split(strings.TrimSpace(ip), ".")
	if len(parts) != 4 {
		return res
	}
	rev := parts[3] + "." + parts[2] + "." + parts[1] + "." + parts[0]
	ctx, cancel := context.WithTimeout(context.Background(), 14*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, z := range dnsblZones {
		wg.Add(1)
		go func(zone string) {
			defer wg.Done()
			addrs, err := net.DefaultResolver.LookupHost(ctx, rev+"."+zone)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				var de *net.DNSError
				if errors.As(err, &de) && de.IsNotFound {
					// NXDOMAIN is a valid "not listed" answer.
					res.Answered++
					return
				}
				res.Unknown = append(res.Unknown, zone)
				return
			}
			if len(addrs) == 0 {
				res.Answered++
				return
			}
			listed, refused := false, false
			for _, a := range addrs {
				if strings.HasPrefix(a, "127.255.255.") {
					refused = true // zone blocked the query (public resolver)
					continue
				}
				if strings.HasPrefix(a, "127.") {
					listed = true
				}
			}
			if listed {
				res.Listed = append(res.Listed, zone)
			}
			if refused && !listed {
				res.Unknown = append(res.Unknown, zone)
				return
			}
			res.Answered++
		}(z)
	}
	wg.Wait()
	res.Clean = len(res.Listed) == 0 && res.Answered >= 4
	return res
}

// Reason returns a short Persian explanation for the result.
func (c CleanResult) Reason() string {
	if c.Clean {
		return "پاک"
	}
	if len(c.Listed) > 0 {
		return "در بلک‌لیست: " + strings.Join(c.Listed, ", ")
	}
	if c.Answered < 4 {
		return "بررسی نشد (پاسخ کافی از DNSBL نرسید)"
	}
	return "نامشخص"
}
