package collect

import (
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	"github.com/replicatedhq/troubleshoot/pkg/docrewrite"
	"k8s.io/apimachinery/pkg/runtime"
)

var decoder runtime.Decoder

func init() {
	decoder = scheme.Codecs.UniversalDeserializer()
}

func ParseCollectorFromDoc(doc []byte) (*troubleshootv1beta2.Collector, error) {
	doc, err := docrewrite.ConvertToV1Beta2(doc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert to v1beta2")
	}

	obj, _, err := decoder.Decode(doc, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse document")
	}

	collector, ok := obj.(*troubleshootv1beta2.Collector)
	if ok {
		return collector, nil
	}

	return nil, errors.New("spec was not parseable as a collector kind")
}

func ParseHostCollectorFromDoc(doc []byte) (*troubleshootv1beta2.HostCollector, error) {
	doc, err := docrewrite.ConvertToV1Beta2(doc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert to v1beta2")
	}

	obj, _, err := decoder.Decode(doc, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse document")
	}

	collector, ok := obj.(*troubleshootv1beta2.HostCollector)
	if ok {
		return collector, nil
	}

	return nil, errors.New("spec was not parseable as a host collector kind")
}

func ParseRemoteCollectorFromDoc(doc []byte) (*troubleshootv1beta2.RemoteCollector, error) {
	doc, err := docrewrite.ConvertToV1Beta2(doc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert to v1beta2")
	}

	obj, _, err := decoder.Decode(doc, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse document")
	}

	collector, ok := obj.(*troubleshootv1beta2.RemoteCollector)
	if ok {
		return collector, nil
	}

	return nil, errors.New("spec was not parseable as a remote collector kind")
}
