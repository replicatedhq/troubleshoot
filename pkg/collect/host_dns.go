package collect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/miekg/dns"
	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/internal/util"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/klog/v2"
)

type CollectHostDNS struct {
	hostCollector *troubleshootv1beta2.HostDNS
	BundlePath    string
}

type DNSResult struct {
	Query              DNSQuery `json:"query"`
	ResolvedFromSearch string   `json:"resolvedFromSearch"`
}

type DNSQuery map[string][]DNSEntry

type DNSEntry struct {
	Server string `json:"server"`
	Search string `json:"search"`
	Name   string `json:"name"`
	Answer string `json:"answer"`
	Record string `json:"record"`
}

const (
	HostDNSPath = "host-collectors/dns/"
	resolvConf  = "/etc/resolv.conf"
)

func (c *CollectHostDNS) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "dns")
}

func (c *CollectHostDNS) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostDNS) SkipRedaction() bool {
	return c.hostCollector.SkipRedaction
}

func (c *CollectHostDNS) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {

	names := c.hostCollector.Hostnames
	if len(names) == 0 {
		return nil, errors.New("hostnames is required")
	}

	// first, get DNS config from /etc/resolv.conf
	dnsConfig, err := getDNSConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read DNS resolve config")
	}

	// query DNS for each name
	dnsEntries := make(map[string][]DNSEntry)
	dnsResult := DNSResult{Query: dnsEntries}
	allResolvedSearches := []string{}

	for _, name := range names {
		entries, resolvedSearches, err := resolveName(name, dnsConfig)
		if err != nil {
			klog.V(2).Infof("Failed to resolve name %s: %v", name, err)
		}
		dnsEntries[name] = entries
		allResolvedSearches = append(allResolvedSearches, resolvedSearches...)
	}

	// deduplicate resolved searches
	dnsResult.ResolvedFromSearch = strings.Join(util.Dedup(allResolvedSearches), ", ")

	// convert dnsResult to a JSON string
	dnsResultJSON, err := json.MarshalIndent(dnsResult, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal DNS query result to JSON")
	}

	output := NewResult()
	outputFile := c.getOutputFilePath("result.json")
	output.SaveResult(c.BundlePath, outputFile, bytes.NewBuffer(dnsResultJSON))

	// write /etc/resolv.conf to a file
	resolvConfData, err := getResolvConf()
	if err != nil {
		klog.V(2).Infof("failed to read DNS resolve config: %v", err)
	} else {
		outputFile = c.getOutputFilePath("resolv.conf")
		output.SaveResult(c.BundlePath, outputFile, bytes.NewBuffer(resolvConfData))
	}

	return output, nil
}

func (c *CollectHostDNS) getOutputFilePath(name string) string {
	// normalize title to be used as a directory name, replace spaces with underscores
	title := strings.ReplaceAll(c.Title(), " ", "_")
	return filepath.Join(HostDNSPath, title, name)
}

func getDNSConfig() (*dns.ClientConfig, error) {
	file, err := os.Open(resolvConf)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config, err := dns.ClientConfigFromFile(file.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to parse resolv.conf: %v", err)
	}

	return config, nil
}

func resolveName(name string, config *dns.ClientConfig) ([]DNSEntry, []string, error) {

	results := []DNSEntry{}
	resolvedSearches := []string{}

	// get a name list based on the config
	queryList := config.NameList(name)
	klog.V(2).Infof("DNS query list: %v", queryList)

	// for each name in the list, query all the servers
	// we will query all search domains for each name
	for _, query := range queryList {
		for _, server := range config.Servers {
			klog.V(2).Infof("Querying DNS server %s for name %s", server, query)

			entry := queryDNS(name, query, server+":"+config.Port)
			results = append(results, entry)

			if entry.Search != "" {
				resolvedSearches = append(resolvedSearches, entry.Search)
			}
		}
	}
	return results, resolvedSearches, nil
}

func getResolvConf() ([]byte, error) {
	data, err := os.ReadFile(resolvConf)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func queryDNS(name, query, server string) DNSEntry {
	recordTypes := []uint16{dns.TypeA, dns.TypeAAAA, dns.TypeCNAME}
	entry := DNSEntry{Name: query, Server: server, Answer: ""}

	for _, rec := range recordTypes {
		m := &dns.Msg{}
		m.SetQuestion(dns.Fqdn(query), rec)
		in, err := dns.Exchange(m, server)

		if err != nil {
			klog.Errorf("failed to query DNS server %s for name %s: %v", server, query, err)
			continue
		}

		if len(in.Answer) == 0 {
			continue
		}

		entry.Answer = in.Answer[0].String()

		// remember the search domain that resolved the query
		// e.g. foo.test.com -> test.com
		entry.Search = extractSearchFromFQDN(query, name)

		// populate record detail
		switch rec {
		case dns.TypeA:
			record, ok := in.Answer[0].(*dns.A)
			if ok {
				entry.Record = record.A.String()
			}
		case dns.TypeAAAA:
			record, ok := in.Answer[0].(*dns.AAAA)
			if ok {
				entry.Record = record.AAAA.String()
			}
		case dns.TypeCNAME:
			record, ok := in.Answer[0].(*dns.CNAME)
			if ok {
				entry.Record = record.Target
			}
		}

		// break on the first successful query
		break
	}
	return entry
}

func (c *CollectHostDNS) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}

func extractSearchFromFQDN(fqdn, name string) string {
	// no search domain
	if fqdn == name {
		return ""
	}
	search := strings.TrimPrefix(fqdn, name+".") // remove name
	search = strings.TrimSuffix(search, ".")     // remove root dot
	return search
}
