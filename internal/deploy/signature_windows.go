//go:build windows

package deploy

import (
	"crypto/x509"
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Validate both Authenticode trust and the signer chosen by WinTrust, rather
// than accepting any certificate merely embedded in a signed executable.
func verifyPublisher(file string, publishers ...string) error {
	p, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	info := windows.WinTrustFileInfo{Size: uint32(unsafe.Sizeof(windows.WinTrustFileInfo{})), FilePath: p}
	signature := windows.WinTrustSignatureSettings{Size: uint32(unsafe.Sizeof(windows.WinTrustSignatureSettings{})), Index: 0, Flags: 1 /* WSS_VERIFY_SPECIFIC */}
	data := windows.WinTrustData{Size: uint32(unsafe.Sizeof(windows.WinTrustData{})), UIChoice: windows.WTD_UI_NONE, RevocationChecks: windows.WTD_REVOKE_WHOLECHAIN, UnionChoice: windows.WTD_CHOICE_FILE, StateAction: windows.WTD_STATEACTION_VERIFY, FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(&info), SignatureSettings: &signature}
	err = windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &data)
	defer func() {
		data.StateAction = windows.WTD_STATEACTION_CLOSE
		_ = windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, &data)
	}()
	if err != nil {
		return fmt.Errorf("installer signature verification failed: %w", err)
	}
	// Resolve the primary signature's actual signer by issuer and serial number;
	// accepting an arbitrary embedded vendor certificate would not prove authorship.
	var store, message windows.Handle
	err = windows.CryptQueryObject(windows.CERT_QUERY_OBJECT_FILE, unsafe.Pointer(p), windows.CERT_QUERY_CONTENT_FLAG_PKCS7_SIGNED_EMBED, windows.CERT_QUERY_FORMAT_FLAG_BINARY, 0, nil, nil, nil, &store, &message, nil)
	if err != nil {
		return err
	}
	defer windows.CertCloseStore(store, 0)
	lib := windows.NewLazySystemDLL("crypt32.dll")
	defer lib.NewProc("CryptMsgClose").Call(uintptr(message))
	get := lib.NewProc("CryptMsgGetParam")
	var size uint32
	ok, _, callErr := get.Call(uintptr(message), 7 /* CMSG_SIGNER_CERT_INFO_PARAM */, 0, 0, uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return fmt.Errorf("read installer signer: %w", callErr)
	}
	if size < uint32(unsafe.Sizeof(windows.CertInfo{})) || size > 65536 {
		return errors.New("invalid installer signer information size")
	}
	buffer := make([]byte, size)
	ok, _, callErr = get.Call(uintptr(message), 7, 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return fmt.Errorf("read installer signer: %w", callErr)
	}
	cert, err := windows.CertFindCertificateInStore(store, windows.X509_ASN_ENCODING|windows.PKCS_7_ASN_ENCODING, 0, windows.CERT_FIND_SUBJECT_CERT, unsafe.Pointer(&buffer[0]), nil)
	if err != nil {
		return err
	}
	defer windows.CertFreeCertificateContext(cert)
	parsed, err := x509.ParseCertificate(unsafe.Slice(cert.EncodedCert, cert.Length))
	if err != nil {
		return err
	}
	for _, org := range parsed.Subject.Organization {
		for _, want := range publishers {
			if org == want {
				return nil
			}
		}
	}
	return fmt.Errorf("installer signature publisher %q does not match the expected vendor", parsed.Subject.Organization)
}
