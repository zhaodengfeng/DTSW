package domain

import (
	"net"
	"sort"
)

func Lookup(domain string) ([]string, error) {
	ips, err := net.LookupIP(domain)
	if err != nil {
		return nil, err
	}
	unique := map[string]struct{}{}
	for _, ip := range ips {
		unique[ip.String()] = struct{}{}
	}
	out := make([]string, 0, len(unique))
	for ip := range unique {
		out = append(out, ip)
	}
	sort.Strings(out)
	return out, nil
}
