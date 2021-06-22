package collect

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseReplicaChecksums(t *testing.T) {
	data := []byte(`
7cc93e21d84bb7d0db0a72281f21500ba3847dea6467631cca91523d01ace8c9  /host/var/lib/longhorn/replicas/pvc-1f9ee2f6-078e-42a6-bf5c-3eaa0722fbfc-68bd18ca/revision.counter
7637cb563f796f8d6358ff4fc635ce596e5326a7f940cc2ea2eaee0acff843ce  /host/var/lib/longhorn/replicas/pvc-1f9ee2f6-078e-42a6-bf5c-3eaa0722fbfc-68bd18ca/volume-head-000.img
ca21027be32ef389de0b21d0c4713e824cad7114a287e05e56de49c948492fc9  /host/var/lib/longhorn/replicas/pvc-1f9ee2f6-078e-42a6-bf5c-3eaa0722fbfc-68bd18ca/volume-head-000.img.meta
e9ce811b3f11dfe3af0bdd46581f23ba2c570be5dc3b807652ad6142322c706b  /host/var/lib/longhorn/replicas/pvc-1f9ee2f6-078e-42a6-bf5c-3eaa0722fbfc-68bd18ca/volume.meta
`)
	got, err := ParseReplicaChecksum(data)
	assert.Nil(t, err)
	want := map[string]string{
		"revision.counter":         "7cc93e21d84bb7d0db0a72281f21500ba3847dea6467631cca91523d01ace8c9",
		"volume-head-000.img":      "7637cb563f796f8d6358ff4fc635ce596e5326a7f940cc2ea2eaee0acff843ce",
		"volume-head-000.img.meta": "ca21027be32ef389de0b21d0c4713e824cad7114a287e05e56de49c948492fc9",
		"volume.meta":              "e9ce811b3f11dfe3af0bdd46581f23ba2c570be5dc3b807652ad6142322c706b",
	}
	assert.Equal(t, want, got)
}
