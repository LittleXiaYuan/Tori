package packruntime

import (
	"encoding/base64"
)

// embeddedPublishers is the compile-time list of publishers whose packs the
// running binary trusts without any user-side configuration. Adding an entry
// here means: this binary will install signed packs from that publisher
// without requiring the user to manually trust the key first.
//
// Rotating keys: append a new entry; do NOT remove the old one until every
// pack signed by it has been re-released under the new key. The (publisher,
// keyID) tuple is what's checked, so old artifacts keep verifying against
// the old key even after the new one is added.
var embeddedPublishers = []struct {
	PublisherID string
	PublicKeyID string
	PubBase64   string
}{
	{
		PublisherID: "yunque-official",
		PublicKeyID: "yunque-official-2026",
		PubBase64:   "SZIi69O41hmQZzG/jvHGi94uPGtXzBiftWjgFeyZM+s=",
	},
}

func init() {
	for _, p := range embeddedPublishers {
		pub, err := base64.StdEncoding.DecodeString(p.PubBase64)
		if err != nil {
			panic("packruntime: invalid embedded public key for " + p.PublisherID + "/" + p.PublicKeyID + ": " + err.Error())
		}
		if err := RegisterEmbeddedTrust(p.PublisherID, p.PublicKeyID, pub); err != nil {
			panic("packruntime: register embedded trust failed: " + err.Error())
		}
	}
}
