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

	output := NewResult()

	// first, read default /etc/resolv.conf file
	dnsConfig, err := readResolvConf()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read DNS resolve config")
	}

	// from this dns config, get a name list to query
	queryList := dnsConfig.NameList(nonResolvableDomain)
	klog.V(2).Infof("DNS query list: %v", queryList)

	// for each name in the list, query all the servers
	dnsResult := make(map[string]string)
	for _, name := range queryList {
		for _, server := range dnsConfig.Servers {
			m := &dns.Msg{}
			m.SetQuestion(dns.Fqdn(name), dns.TypeA)
			klog.V(2).Infof("Querying DNS server %s for name %s", server, name)
			in, err := dns.Exchange(m, server+":"+dnsConfig.Port)
			if err != nil {
				klog.Errorf("failed to query DNS server %s for name %s: %v", server, name, err)
				continue
			}
			if len(in.Answer) == 0 {
				dnsResult[name] = ""
			}
			for _, answer := range in.Answer {
				dnsResult[name] = answer.String()
			}
		}
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
