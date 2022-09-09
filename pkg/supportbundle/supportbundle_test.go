package supportbundle

import (
	"reflect"
	"testing"
)

func Test_LoadAndConcatSpec(t *testing.T) {

    bundle1doc,err := LoadSupportBundleSpec("test/supportbundle1.yaml")
    if err != nil {
        t.Error("couldn't load bundle1 from file")
    }

    bundle2doc,err := LoadSupportBundleSpec("test/supportbundle2.yaml")
    if err != nil {
        t.Error("couldn't load bundle2 from file")
    }

    bundle1,err := ParseSupportBundleFromDoc(bundle1doc)
    if err != nil {
        t.Error("couldn't parse bundle 1")
    }

    bundle2,err := ParseSupportBundleFromDoc(bundle2doc)
    if err != nil {
        t.Error("couldn't parse bundle 2")
    }

    fulldoc,err := LoadSupportBundleSpec("test/completebundle.yaml")
    if err != nil {
        t.Error("couldn't load full bundle from file")
    }

    fullbundle,err := ParseSupportBundleFromDoc(fulldoc)
    if err != nil {
        t.Error("couldn't parse full bundle")
    }

    bundle3 := ConcatSpec(bundle1,bundle2)

    if reflect.DeepEqual(fullbundle, bundle3) == false {
        t.Error("Full bundle and concatenated bundle are not the same.")
    }

}
