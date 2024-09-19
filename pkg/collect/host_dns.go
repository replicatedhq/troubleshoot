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
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/klog/v2"
)

type CollectHostDNS struct {
	hostCollector *troubleshootv1beta2.HostDNS
	BundlePath    string
}

type DNSResult struct {
	Query DNSQuery `json:"query"`
}

type DNSSummary struct {
	Resolved ResolvedSearches `json:"resolved"`
}

// ResolvedSearches is a map of name to search domains that resolved the name
type ResolvedSearches map[string]string

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

func (c *CollectHostDNS) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {

	names := c.hostCollector.Hostnames
	if len(names) == 0 {
		// if no names are provided, query a wilcard to detect wildcard DNS if any
		names = append(names, "*")
	}
	output := NewResult()

	// first, get DNS config from /etc/resolv.conf
	dnsConfig, err := getDNSConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read DNS resolve config")
	}

	// query DNS for each name
	dnsEntries := make(map[string][]DNSEntry)
	dnsSummary := &DNSSummary{Resolved: make(ResolvedSearches)}

	for _, name := range names {
		entries, resolvedSearches, err := resolveName(name, dnsConfig)
		if err != nil {
			klog.V(2).Infof("Failed to resolve name %s: %v", name, err)
		}
		dnsEntries[name] = entries
		dnsSummary.Resolved[name] = resolvedSearches
	}
	dnsResult := DNSResult{Query: dnsEntries}

	// convert dnsResult to a JSON string
	dnsResultJSON, err := json.MarshalIndent(dnsResult, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal DNS query result to JSON")
	}

	outputFile := c.getOutputFilePath("result.json")
	output.SaveResult(c.BundlePath, outputFile, bytes.NewBuffer(dnsResultJSON))

	// convert dnsSummary to a JSON string
	dnsSummaryJSON, err := json.MarshalIndent(dnsSummary, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal DNS summary to JSON")
	}
	outputFile = c.getOutputFilePath("summary.json")
	output.SaveResult(c.BundlePath, outputFile, bytes.NewBuffer(dnsSummaryJSON))

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

func resolveName(name string, config *dns.ClientConfig) ([]DNSEntry, string, error) {

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
			m := &dns.Msg{}
			m.SetQuestion(dns.Fqdn(query), dns.TypeA)
			in, err := dns.Exchange(m, server+":"+config.Port)

			entry := DNSEntry{Name: query, Server: server, Answer: ""}

			// e.g. foo.test.com -> test.com
			entry.Search = strings.Replace(query, name, "", 1)

			if err != nil {
				klog.Errorf("failed to query DNS server %s for name %s: %v", server, query, err)
				results = append(results, entry)
				continue
			}
			if len(in.Answer) == 0 {
				results = append(results, entry)
				continue
			}
			entry.Answer = in.Answer[0].String()
			record, ok := in.Answer[0].(*dns.A)
			if ok {
				klog.V(2).Infof("Resolved %s to %s", query, record.A.String())
				entry.Record = record.A.String()
				resolvedSearches = append(resolvedSearches, entry.Search)
			}

			results = append(results, entry)
		}
	}
	return results, strings.Join(resolvedSearches, ","), nil
}

func getResolvConf() ([]byte, error) {
	data, err := os.ReadFile(resolvConf)
	if err != nil {
		return nil, err
	}
	return data, nil
}
