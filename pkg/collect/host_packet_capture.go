package collect

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcap"
	"github.com/gopacket/gopacket/pcapgo"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

const HostPacketCapturePath = `host-collectors/system/host_pcap.pcap`
const HostPacketCaptureResultPath = `host-collectors/system/host_pcap.json`
const HostPacketCaptureFileName = `host_pcap.pcap`

type CollectHostPacketCapture struct {
	hostCollector *troubleshootv1beta2.HostPacketCapture
	BundlePath    string
}

func (c *CollectHostPacketCapture) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Host Packet Capture")
}

func (c *CollectHostPacketCapture) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostPacketCapture) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	// Use WaitGroup to wait for packet capture from all devices
	var wg sync.WaitGroup

	//pcapFilePath := filepath.Join(c.BundlePath, HostPacketCapturePath)
	pcapFilePath := "./host_pcap.pcap"
	// Ensure the directory exists
	dir := filepath.Dir(pcapFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create directories for pcap file")
	}

	// Now create the file
	f, err := os.Create(pcapFilePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create pcap file")
	}
	defer f.Close()

	// Create a pcap writer for the file
	w := pcapgo.NewWriter(f)

	// Write the PCAP file header (link type and snapshot length)
	err = w.WriteFileHeader(1600, layers.LinkTypeEthernet)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write pcap file header")
	}

	// Start packet capture on all network devices concurrently for 5 seconds
	devices := []string{"lo0", "en0"} // Add other devices here, replace with actual devices to capture
	for _, device := range devices {
		wg.Add(1)
		go func(deviceName string) {
			defer wg.Done()
			err := capturePacketsToPcap(deviceName, w, 5*time.Second)
			if err != nil {
				fmt.Printf("Error capturing packets on device %s: %v\n", deviceName, err)
			}
		}(device)
	}

	// Wait for all packet captures to complete
	wg.Wait()

	// Open the saved pcap file for reading
	pcapFile, err := os.Open(pcapFilePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open pcap file for reading")
	}
	defer pcapFile.Close()

	// Use the pcap file as the reader for SaveResult
	output := NewResult()
	if err := output.SaveResult(c.BundlePath, HostPacketCapturePath, pcapFile); err != nil {
		return nil, errors.Wrap(err, "failed to save result")
	}

	return output, nil
}

// Helper function to capture packets using pcapgo on a specific device
func capturePacketsToPcap(device string, pcapw *pcapgo.Writer, duration time.Duration) error {
	// Open the Ethernet handle for packet capture (use pcapgo)
	handle, err := pcap.OpenLive(device, 1600, true, pcap.BlockForever)
	if err != nil {
		return errors.Wrapf(err, "error opening live capture for device %s", device)
	}
	defer handle.Close()

	// Set up packet source for capturing packets
	pkgsrc := gopacket.NewPacketSource(handle, layers.LayerTypeEthernet)

	// Context for packet capturing
	timeout := time.After(duration)
	for {
		select {
		case packet := <-pkgsrc.Packets():
			// Write each captured packet to the pcap file
			err := pcapw.WritePacket(packet.Metadata().CaptureInfo, packet.Data())
			if err != nil {
				return errors.Wrap(err, "error writing packet to pcap file")
			}
		case <-timeout:
			// Timeout reached, stop capturing
			return nil
		}
	}
}
