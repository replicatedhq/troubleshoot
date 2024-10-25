package supportbundle

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

func portForward(ctx context.Context, rc *rest.Config, pod corev1.Pod, port string) (net.Conn, error) {
	cs, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return nil, fmt.Errorf("error creating http client: %w", err)
	}

	req := cs.RESTClient().
		Post().
		Prefix("api/v1").
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(rc)
	if err != nil {
		return nil, fmt.Errorf("error getting transport/upgrader from restconfig: %w", err)
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())
	conn, _, err := dialer.Dial(portforward.PortForwardProtocolV1Name)
	if err != nil {
		return nil, fmt.Errorf("error dialing for conn %w", err)
	}

	headers := http.Header{}
	headers.Set(v1.StreamType, v1.StreamTypeError)
	headers.Set(v1.PortHeader, port)
	headers.Set(v1.PortForwardRequestIDHeader, "1")

	errorStream, err := conn.CreateStream(headers)
	if err != nil {
		return nil, fmt.Errorf("error creating err stream: %w", err)
	}
	// we're not writing to this stream
	errorStream.Close()

	headers.Set(v1.StreamType, v1.StreamTypeData)
	dataStream, err := conn.CreateStream(headers)
	if err != nil {
		return nil, fmt.Errorf("error creating data stream: %w", err)
	}

	fc := &fakeConn{
		parent: conn,
		port:   port,
		err:    errorStream,
		errch:  make(chan error),
		data:   dataStream,
		pod:    pod,
	}
	go fc.watchErr(ctx)

	return fc, nil
}

// This is a FakeAddr type used just in case anything asks for the net.Addr on
// either side of this "network connection." It's there for debug and helps to
// show that the source is memory and the destination is a k8s pod in a specific
// namespace. `Network` returns "memory" because it's in-memory rather than tcp/udp.
type fakeAddr string

func (f fakeAddr) Network() string {
	return "memory"
}
func (f fakeAddr) String() string {
	return string(f)
}

// FakeConn is the guts of our connection. Most of this code is for handling
// channels and the fact that two things may error, resulting in a problem for
// our callers.
type fakeConn struct {
	parent    httpstream.Connection
	data, err httpstream.Stream
	errch     chan error
	port      string
	pod       v1.Pod
}

func (f *fakeConn) watchErr(ctx context.Context) {
	// This should only return if an err comes back.
	bs, err := io.ReadAll(f.err)
	if err != nil {
		select {
		case <-ctx.Done():
		case f.errch <- fmt.Errorf("error during read: %w", err):
		}
	}
	if len(bs) > 0 {
		select {
		case <-ctx.Done():
		case f.errch <- fmt.Errorf("error during read: %s", string(bs)):
		}
	}
}

func (f *fakeConn) Read(b []byte) (n int, err error) {
	select {
	case err := <-f.errch:
		return 0, err
	default:
	}
	return f.data.Read(b)
}

func (f *fakeConn) Write(b []byte) (n int, err error) {
	select {
	case err := <-f.errch:
		return 0, err
	default:
	}
	return f.data.Write(b)
}

func (f *fakeConn) Close() error {
	var errs []error
	select {
	case err := <-f.errch:
		if err != nil {
			errs = append(errs, err)
		}
	default:
	}
	err := f.data.Close()
	if err != nil {
		errs = append(errs, err)
	}
	f.parent.RemoveStreams(f.data, f.err)
	err = f.parent.Close()
	if err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (f *fakeConn) LocalAddr() net.Addr {
	return fakeAddr("memory:" + f.port)
}

func (f *fakeConn) RemoteAddr() net.Addr {
	return fakeAddr(fmt.Sprintf("k8s/%s/%s:%s", f.pod.Namespace, f.pod.Name, f.port))
}

func (f *fakeConn) SetDeadline(t time.Time) error {
	f.parent.SetIdleTimeout(time.Until(t))
	return nil
}

func (f *fakeConn) SetReadDeadline(t time.Time) error {
	f.parent.SetIdleTimeout(time.Until(t))
	return nil
}

func (f *fakeConn) SetWriteDeadline(t time.Time) error {
	f.parent.SetIdleTimeout(time.Until(t))
	return nil
}
