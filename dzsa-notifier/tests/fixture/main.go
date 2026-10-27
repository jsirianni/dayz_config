// fixture is a local HTTPS CONNECT proxy for integration tests. It never forwards
// requests or contacts DZSA. Its generated CA is trusted only by test processes.
package main

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	dir := flag.String("directory", "", "test certificate output directory")
	listen := flag.String("listen", "127.0.0.1:18443", "proxy listen address")
	flag.Parse()
	if *dir == "" {
		log.Fatal("--directory is required")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	must(err)
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Notifier test CA"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	must(err)
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "dayzsalauncher.com"}, DNSNames: []string{"dayzsalauncher.com"}, NotBefore: ca.NotBefore, NotAfter: ca.NotAfter, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, leaf, ca, &key.PublicKey, key)
	must(err)
	private, err := x509.MarshalECPrivateKey(key)
	must(err)
	pair, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: private}))
	must(err)
	must(os.MkdirAll(*dir, 0755))
	must(os.WriteFile(filepath.Join(*dir, "ca.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0644))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "CONNECT" || r.Host != "dayzsalauncher.com:443" {
			http.Error(w, "fixture only permits DZSA CONNECT", 403)
			return
		}
		connection, buffer, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer connection.Close()
		_ = connection.SetDeadline(time.Now().Add(10 * time.Second))
		_, _ = buffer.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		_ = buffer.Flush()
		secure := tls.Server(connection, &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12})
		if secure.Handshake() != nil {
			return
		}
		request, err := http.ReadRequest(bufio.NewReader(secure))
		if err != nil {
			return
		}
		address := strings.TrimPrefix(request.URL.Path, "/api/v1/query/")
		ap, err := netip.ParseAddrPort(address)
		if err != nil {
			return
		}
		status := 200
		body := map[string]any{"status": 0, "result": map[string]any{"endpoint": map[string]any{"ip": ap.Addr().String(), "port": ap.Port()}, "game": "dayz", "name": "Fixture DayZ", "mods": []any{map[string]any{"name": "Fixture mod"}}}}
		if ap.Port() == 27017 {
			status = 503
		}
		data, _ := json.Marshal(body)
		response := &http.Response{StatusCode: status, ProtoMajor: 1, ProtoMinor: 1, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(string(data))), ContentLength: int64(len(data)), Close: true}
		response.Header.Set("Content-Type", "application/json")
		_ = response.Write(secure)
	})
	fmt.Println("fixture ready", *listen)
	log.Fatal(http.ListenAndServe(*listen, handler))
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
