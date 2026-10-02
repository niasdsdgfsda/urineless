package e2e

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"path/filepath"

	"golang.org/x/crypto/scrypt"

	"github.com/GoCodeAlone/libsignal-go/address"
	"github.com/GoCodeAlone/libsignal-go/curve"
	"github.com/GoCodeAlone/libsignal-go/kem"
	"github.com/GoCodeAlone/libsignal-go/protocol"
	"github.com/GoCodeAlone/libsignal-go/session"
	"github.com/GoCodeAlone/libsignal-go/stores"
)

const (
	magic        = "IGS1"
	maxHandshake = 100 // so viele ungenutzte Handshake-Schlüssel werden vorgehalten
)

type ecEntry struct{ Pub, Priv []byte }
type kyberEntry struct{ Pub, Secret []byte }

// data ist der Klartext-Inhalt der Schlüsseldatei (wird nur verschlüsselt gespeichert).
type data struct {
	RegID uint32
	IdPub []byte
	IdPriv []byte

	SPID   uint32
	SPPub  []byte
	SPPriv []byte
	SPSig  []byte

	NextID     uint32
	OneTime    map[uint32]ecEntry
	Kyber      map[uint32]kyberEntry
	Sessions   map[string][]byte // Peer-Nick (klein) -> SessionRecord
	Identities map[string][]byte // Peer-Nick (klein) -> Identity-Public-Key (TOFU)
}

// store implementiert session.Store und stores.IdentityKeyStore und speichert
// alles AES-GCM-verschlüsselt (Schlüssel via scrypt aus dem Passwort).
// Der Aufrufer (Manager) serialisiert alle Zugriffe.
type store struct {
	path string
	key  []byte
	salt []byte
	d    data
	id   curve.KeyPair
}

func openStore(path, pass string) (*store, error) {
	s := &store{path: path}
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := s.load(raw, pass); err != nil {
			return nil, err
		}
	case errors.Is(err, os.ErrNotExist):
		if err := s.create(pass); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}
	return s, nil
}

func deriveKey(pass string, salt []byte) ([]byte, error) {
	return scrypt.Key([]byte(pass), salt, 1<<15, 8, 1, 32)
}

func (s *store) create(pass string) error {
	id, err := curve.GenerateKeyPair(rand.Reader)
	if err != nil {
		return err
	}
	sp, err := curve.GenerateKeyPair(rand.Reader)
	if err != nil {
		return err
	}
	sig, err := id.PrivateKey.CalculateSignature(rand.Reader, sp.PublicKey.Serialize())
	if err != nil {
		return err
	}
	n, err := rand.Int(rand.Reader, big.NewInt(16380))
	if err != nil {
		return err
	}
	s.id = id
	s.d = data{
		RegID:  uint32(n.Int64()) + 1,
		IdPub:  id.PublicKey.Serialize(),
		IdPriv: id.PrivateKey.Serialize(),
		SPID:   1,
		SPPub:  sp.PublicKey.Serialize(),
		SPPriv: sp.PrivateKey.Serialize(),
		SPSig:  sig,
		NextID: 1,
	}
	s.initMaps()
	s.salt = make([]byte, 16)
	if _, err := rand.Read(s.salt); err != nil {
		return err
	}
	if s.key, err = deriveKey(pass, s.salt); err != nil {
		return err
	}
	return s.save()
}

func (s *store) initMaps() {
	if s.d.OneTime == nil {
		s.d.OneTime = map[uint32]ecEntry{}
	}
	if s.d.Kyber == nil {
		s.d.Kyber = map[uint32]kyberEntry{}
	}
	if s.d.Sessions == nil {
		s.d.Sessions = map[string][]byte{}
	}
	if s.d.Identities == nil {
		s.d.Identities = map[string][]byte{}
	}
}

func (s *store) load(raw []byte, pass string) error {
	const hdr = len(magic) + 16 + 12
	if len(raw) < hdr+16 || string(raw[:len(magic)]) != magic {
		return errors.New("Schlüsseldatei beschädigt")
	}
	s.salt = append([]byte(nil), raw[len(magic):len(magic)+16]...)
	nonce := raw[len(magic)+16 : hdr]
	key, err := deriveKey(pass, s.salt)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	plain, err := gcm.Open(nil, nonce, raw[hdr:], []byte(magic))
	if err != nil {
		return errors.New("falsches Passwort oder beschädigte Schlüsseldatei")
	}
	if err := json.Unmarshal(plain, &s.d); err != nil {
		return err
	}
	s.initMaps()
	s.key = key
	s.id, err = curve.KeyPairFromPublicAndPrivate(s.d.IdPub, s.d.IdPriv)
	return err
}

func (s *store) save() error {
	plain, err := json.Marshal(&s.d)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	out := append([]byte(magic), s.salt...)
	out = append(out, nonce...)
	out = gcm.Seal(out, nonce, plain, []byte(magic))

	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// ---- session.Store ----

func (s *store) LoadSession(_ context.Context, a address.ProtocolAddress) (*session.SessionRecord, error) {
	b, ok := s.d.Sessions[a.Name()]
	if !ok {
		return nil, nil
	}
	return session.DeserializeSessionRecord(b)
}

func (s *store) StoreSession(_ context.Context, a address.ProtocolAddress, r *session.SessionRecord) error {
	if r == nil {
		return errors.New("e2e: leerer Session-Record")
	}
	b, err := r.Serialize()
	if err != nil {
		return err
	}
	s.d.Sessions[a.Name()] = b
	return s.save()
}

// ---- stores.IdentityKeyStore ----

func (s *store) GetIdentityKeyPair(context.Context) (curve.KeyPair, error) { return s.id, nil }

func (s *store) GetLocalRegistrationID(context.Context) (uint32, error) { return s.d.RegID, nil }

func (s *store) SaveIdentity(_ context.Context, a address.ProtocolAddress, id curve.PublicKey) (stores.IdentityChange, error) {
	old, had := s.d.Identities[a.Name()]
	nb := id.Serialize()
	s.d.Identities[a.Name()] = nb
	if err := s.save(); err != nil {
		return stores.NewOrUnchanged, err
	}
	return stores.IdentityChangeFromReplaced(had && !bytes.Equal(old, nb)), nil
}

// IsTrustedIdentity: Trust-on-first-use. Ein bekannter Nick mit anderem Schlüssel ist NICHT vertrauenswürdig.
func (s *store) IsTrustedIdentity(_ context.Context, a address.ProtocolAddress, id curve.PublicKey, _ stores.Direction) (bool, error) {
	old, ok := s.d.Identities[a.Name()]
	if !ok {
		return true, nil
	}
	return bytes.Equal(old, id.Serialize()), nil
}

func (s *store) GetIdentity(_ context.Context, a address.ProtocolAddress) (curve.PublicKey, bool, error) {
	b, ok := s.d.Identities[a.Name()]
	if !ok {
		return curve.PublicKey{}, false, nil
	}
	pk, err := curve.DeserializePublicKey(b)
	return pk, err == nil, err
}

// ---- Prekeys ----

// newBundle erzeugt pro Handshake frische One-Time- und Kyber-Prekeys und liefert das Paket für den Peer.
func (s *store) newBundle() ([]byte, error) {
	ec, err := curve.GenerateKeyPair(rand.Reader)
	if err != nil {
		return nil, err
	}
	ky, err := kem.GenerateKeyPair(kem.KeyTypeKyber1024, rand.Reader)
	if err != nil {
		return nil, err
	}
	kySig, err := s.id.PrivateKey.CalculateSignature(rand.Reader, ky.PublicKey.Serialize())
	if err != nil {
		return nil, err
	}
	s.d.NextID++
	id := s.d.NextID
	s.d.OneTime[id] = ecEntry{Pub: ec.PublicKey.Serialize(), Priv: ec.PrivateKey.Serialize()}
	s.d.Kyber[id] = kyberEntry{Pub: ky.PublicKey.Serialize(), Secret: ky.SecretKey.Serialize()}
	for len(s.d.OneTime) > maxHandshake { // älteste verwerfen
		var oldest uint32
		first := true
		for k := range s.d.OneTime {
			if first || k < oldest {
				oldest, first = k, false
			}
		}
		delete(s.d.OneTime, oldest)
		delete(s.d.Kyber, oldest)
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return encodeBundle(bundleFields{
		Reg: s.d.RegID, PreID: id, SPID: s.d.SPID, KyberID: id,
		Pre: s.d.OneTime[id].Pub, SP: s.d.SPPub, SPSig: s.d.SPSig,
		Kyber: s.d.Kyber[id].Pub, KyberSig: kySig, Ident: s.d.IdPub,
	}), nil
}

// bobState baut aus einer PreKeySignalMessage die Empfänger-Seite der Sitzung (PQXDH).
func (s *store) bobState(pk *protocol.PreKeySignalMessage) (*session.SessionState, error) {
	if pk.SignedPreKeyID() != s.d.SPID {
		return nil, errors.New("unbekannter Signed-Prekey")
	}
	sp, err := curve.KeyPairFromPublicAndPrivate(s.d.SPPub, s.d.SPPriv)
	if err != nil {
		return nil, err
	}
	var oneTime *curve.KeyPair
	if id := pk.PreKeyID(); id != nil {
		e, ok := s.d.OneTime[*id]
		if !ok {
			return nil, errors.New("unbekannter One-Time-Prekey")
		}
		kp, err := curve.KeyPairFromPublicAndPrivate(e.Pub, e.Priv)
		if err != nil {
			return nil, err
		}
		oneTime = &kp
	}
	kid := pk.KyberPreKeyID()
	if kid == nil {
		return nil, errors.New("Kyber-Prekey fehlt")
	}
	ke, ok := s.d.Kyber[*kid]
	if !ok {
		return nil, errors.New("unbekannter Kyber-Prekey")
	}
	ky, err := kem.KeyPairFromPublicAndSecret(ke.Pub, ke.Secret)
	if err != nil {
		return nil, err
	}
	return session.InitializeBobSession(session.BobParams{
		OurIdentity:   s.id,
		OurSignedPre:  sp,
		OurOneTime:    oneTime,
		OurKyber:      ky,
		TheirIdentity: pk.IdentityKey(),
		TheirBaseKey:  pk.BaseKey(),
		KyberCipher:   pk.KyberCiphertext(),
	})
}

// consume löscht die verbrauchten Einmal-Schlüssel.
func (s *store) consume(pk *protocol.PreKeySignalMessage) error {
	if id := pk.PreKeyID(); id != nil {
		delete(s.d.OneTime, *id)
	}
	if id := pk.KyberPreKeyID(); id != nil {
		delete(s.d.Kyber, *id)
	}
	return s.save()
}
