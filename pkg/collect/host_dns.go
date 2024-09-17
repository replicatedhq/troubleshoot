package collect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/miekg/dns"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/klog/v2"
)

type CollectHostDNS struct {
	hostCollector *troubleshootv1beta2.HostDNS
	BundlePath    string
}

const (
	HostDNSPath = "host-collectors/dns/"
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

	// first, read default /etc/resolv.conf file
	dnsConfig, err := readResolvConf()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read DNS resolve config")
	}

	// query DNS for each name
	dnsResult := make(map[string]string)
	for _, name := range names {
		ip, err := resolveName(name, dnsConfig)
		if err != nil {
			klog.V(2).Infof("Failed to resolve name %s: %v", name, err)
			dnsResult[name] = ""
		}
		dnsResult[name] = ip
	}

	// convert dnsResult to a JSON string
	dnsResultJSON, err := json.MarshalIndent(dnsResult, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal DNS query result to JSON")
	}

	outputFile := filepath.Join(HostDNSPath, "dns.json")
	output.SaveResult(c.BundlePath, outputFile, bytes.NewBuffer(dnsResultJSON))

	return output, nil
}

func readResolvConf() (*dns.ClientConfig, error) {
	defaultResolvPath := "/etc/resolv.conf"

	file, err := os.Open(defaultResolvPath)
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

func resolveName(name string, config *dns.ClientConfig) (string, error) {
	// get a name list based on the config
	queryList := config.NameList(name)
	klog.V(2).Infof("DNS query list: %v", queryList)

	// for each name in the list, query all the servers
	// return as soon as a result is found
	for _, n := range queryList {
		for _, server := range config.Servers {
			klog.V(2).Infof("Querying DNS server %s for name %s", server, n)
			m := &dns.Msg{}
			m.SetQuestion(dns.Fqdn(n), dns.TypeA)
			in, err := dns.Exchange(m, server+":"+config.Port)
			if err != nil {
				klog.Errorf("failed to query DNS server %s for name %s: %v", server, n, err)
				continue
			}
			if len(in.Answer) == 0 {
				continue
			}
			return in.Answer[0].String(), nil
		}
	}
	return "", nil
}
