package types

type CertInfo struct {
	Issuer    string `json:"issuer"`
	Subject   string `json:"subject"`
	Serial    string `json:"serial"`
	NotBefore string `json:"not_before"`
	NotAfter  string `json:"not_after"`
	IsCA      bool   `json:"is_ca"`
	Raw       []byte `json:"raw"`
}

type TLSInfo struct {
	PeerCertificates []CertInfo `json:"peer_certificates"`
}
