package e2e

import (
	"bytes"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/GoCodeAlone/libsignal-go/curve"
	"github.com/GoCodeAlone/libsignal-go/kem"
	"github.com/GoCodeAlone/libsignal-go/session"
)

// Prefix kennzeichnet E2E-Zeilen in PRIVMSG-Texten.
const Prefix = "E2E1 "

const (
	chunkSize = 300 // Base64-Zeichen pro IRC-Zeile (Limit der Zeile: 512 Bytes inkl. Server-Präfix)
	maxChunks = 40
	deviceNum = 1
)

// Nachrichtenarten (erstes Byte der Nutzlast).
const (
	kindRequest = 'R' // "schick mir dein Schlüsselpaket"
	kindBundle  = 'B' // Schlüsselpaket
	kindPreKey  = 'P' // erste Nachricht einer Sitzung (PreKeySignalMessage)
	kindSignal  = 'S' // normale Nachricht (SignalMessage)
)

// frame verpackt kind+data base64-kodiert in IRC-taugliche Zeilen:
//
//	E2E1 <id> <i>/<n> <chunk>
func frame(kind byte, payload []byte) []string {
	raw := append([]byte{kind}, payload...)
	enc := base64.RawStdEncoding.EncodeToString(raw)
	var idb [2]byte
	_, _ = rand.Read(idb[:])
	id := fmt.Sprintf("%x", idb[:])
	total := (len(enc) + chunkSize - 1) / chunkSize
	lines := make([]string, 0, total)
	for i := 0; i < total; i++ {
		lo, hi := i*chunkSize, min((i+1)*chunkSize, len(enc))
		lines = append(lines, fmt.Sprintf("%s%s %d/%d %s", Prefix, id, i+1, total, enc[lo:hi]))
	}
	return lines
}

// ---- Schlüsselpaket (eigenes, kompaktes Binärformat) ----

type bundleFields struct {
	Reg, PreID, SPID, KyberID uint32
	Pre, SP, SPSig            []byte
	Kyber, KyberSig, Ident    []byte
}

func encodeBundle(f bundleFields) []byte {
	var buf bytes.Buffer
	for _, v := range []uint32{f.Reg, f.PreID, f.SPID, f.KyberID} {
		_ = binary.Write(&buf, binary.BigEndian, v)
	}
	for _, b := range [][]byte{f.Pre, f.SP, f.SPSig, f.Kyber, f.KyberSig, f.Ident} {
		_ = binary.Write(&buf, binary.BigEndian, uint16(len(b)))
		buf.Write(b)
	}
	return buf.Bytes()
}

func parseBundle(data []byte) (*session.PreKeyBundle, error) {
	r := bytes.NewReader(data)
	var u [4]uint32
	for i := range u {
		if err := binary.Read(r, binary.BigEndian, &u[i]); err != nil {
			return nil, err
		}
	}
	var parts [6][]byte
	for i := range parts {
		var n uint16
		if err := binary.Read(r, binary.BigEndian, &n); err != nil {
			return nil, err
		}
		parts[i] = make([]byte, n)
		if _, err := io.ReadFull(r, parts[i]); err != nil {
			return nil, err
		}
	}
	pre, err := curve.DeserializePublicKey(parts[0])
	if err != nil {
		return nil, err
	}
	sp, err := curve.DeserializePublicKey(parts[1])
	if err != nil {
		return nil, err
	}
	ky, err := kem.DeserializePublicKey(parts[3])
	if err != nil {
		return nil, err
	}
	ident, err := curve.DeserializePublicKey(parts[5])
	if err != nil {
		return nil, err
	}
	preID := u[1]
	return session.NewPreKeyBundle(session.PreKeyBundleParams{
		RegistrationID:  u[0],
		DeviceID:        deviceNum,
		PreKeyID:        &preID,
		PreKey:          &pre,
		SignedPreKeyID:  u[2],
		SignedPreKey:    sp,
		SignedPreKeySig: parts[2],
		KyberPreKeyID:   u[3],
		KyberPreKey:     ky,
		KyberPreKeySig:  parts[4],
		IdentityKey:     ident,
	})
}

// fingerprint ist eine Sicherheitsnummer, die beide Seiten identisch berechnen
// (NICHT kompatibel mit den Safety Numbers der Signal-App).
func fingerprint(a, b []byte) string {
	if bytes.Compare(a, b) > 0 {
		a, b = b, a
	}
	h := sha512.New()
	h.Write([]byte("ircgram-e2e-v1"))
	h.Write(a)
	h.Write(b)
	d := h.Sum(nil)
	groups := make([]string, 12)
	for i := range groups {
		groups[i] = fmt.Sprintf("%05d", binary.BigEndian.Uint32(d[i*4:])%100000)
	}
	return strings.Join(groups[:6], " ") + " / " + strings.Join(groups[6:], " ")
}
