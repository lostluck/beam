// fromyaml generates a resource file from the standard_coders.yaml
// file for use in these coder regression tests.
package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"

	"golang.org/x/text/encoding/charmap"

	"github.com/apache/beam/sdks/go/pkg/beam/core/graph/mtime"
	"github.com/apache/beam/sdks/go/pkg/beam/core/graph/window"
	"github.com/apache/beam/sdks/go/pkg/beam/core/runtime/exec"
	"github.com/apache/beam/sdks/go/pkg/beam/core/runtime/graphx"
	"github.com/apache/beam/sdks/go/pkg/beam/core/util/reflectx"
	pipepb "github.com/apache/beam/sdks/go/pkg/beam/model/pipeline_v1"
	"github.com/google/go-cmp/cmp"
	yaml "gopkg.in/yaml.v2"
)

// TODO(lostluck): move this to the gen call.
const yamlPath = "../../../../../../model/fn-execution/src/main/resources/org/apache/beam/model/fnexecution/v1/standard_coders.yaml"

// Coder is a representation a serialized beam coder.
type Coder struct {
	Urn              string  `yaml:"urn,omitempty"`
	Payload          string  `yaml:"payload,omitempty"`
	Components       []Coder `yaml:"components,omitempty"`
	NonDeterministic bool    `yaml:"non_deterministic,omitempty"`
}

// Spec is a set of conditions that a coder must pass.
type Spec struct {
	Coder    Coder         `yaml:"coder,omitempty"`
	Nested   *bool         `yaml:"nested,omitempty"`
	Examples yaml.MapSlice `yaml:"examples,omitempty"`

	id       int // for generating coder ids.
	coderPBs map[string]*pipepb.Coder
}

func (s *Spec) nextID() string {
	ret := fmt.Sprintf("%d", s.id)
	s.id++
	return ret
}

func (s *Spec) testStandardCoder() (err error) {
	// Construct the coder proto equivalents.

	// Only unnested tests need to be run, since
	// nestedness is a pre-portability concept.
	if s.Nested != nil && !*s.Nested && s.Coder.Urn != "beam:coder:row:v1" {
		log.Printf("skipping unnested coder spec: %v\n", s.Coder)
		return nil
	}

	s.coderPBs = make(map[string]*pipepb.Coder)
	id := s.parseCoder(s.Coder)
	b := graphx.NewCoderUnmarshaller(s.coderPBs)
	underTest, err := b.Coder(id)
	if err != nil {
		return fmt.Errorf("unable to create coder: %v", err)
	}

	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("panicked on coder %v || %v:\n\t%v :\n%s", underTest, s.Coder, e, debug.Stack())
		}
	}()

	var decFails, encFails int
	for _, eg := range s.Examples {

		// Test Decoding
		// Ideally we'd use the beam package coders, but KVs make that complicated.
		// This can be cleaned up once a type parametered beam.KV type exists.
		dec := exec.MakeElementDecoder(underTest)
		encoded := eg.Key.(string)
		var elem exec.FullValue

		recoded, err := charmap.ISO8859_1.NewEncoder().String(encoded)
		if err != nil {
			return err
		}
		r := strings.NewReader(recoded)
		// What I would have expected.
		//		r := charmap.ISO8859_1.NewDecoder().Reader(strings.NewReader(encoded))
		if err := dec.DecodeTo(r, &elem); err != nil {
			return err
		}
		if !diff(s.Coder, &elem, eg) {
			//log.Printf("failed %v:%v %q;%T %q", underTest, i, eg.Key, eg.Value, eg.Value)
			decFails++
			continue
		}

		// Test Encoding
		if s.Coder.NonDeterministic {
			// Skip verifying nondeterministic encodings.
			continue
		}
		_ = exec.MakeElementEncoder(underTest)
	}
	if decFails+encFails > 0 {
		return fmt.Errorf("failed to decode %v times, and encode %v times", decFails, encFails)
	}

	return nil
}

var cmpOpts = []cmp.Option{
	cmp.Transformer("uint8s2string", func(in []uint8) (out string) {
		return string(in)
	}),
	cmp.Transformer("bytes2string", func(in []byte) (out string) {
		return string(in)
	}),
}

func diff(c Coder, elem *exec.FullValue, eg yaml.MapItem) bool {
	var got, want interface{}
	switch c.Urn {
	case "beam:coder:bytes:v1":
		got = string(elem.Elm.([]byte))
		switch egv := eg.Value.(type) {
		case string:
			want = egv
		case []byte:
			want = string(egv)
		}
	case "beam:coder:varint:v1":
		got, want = elem.Elm.(int64), int64(eg.Value.(int))
	case "beam:coder:double:v1":
		got = elem.Elm.(float64)
		switch v := eg.Value.(string); v {
		case "NaN":
			// Do the NaN comparison here since NaN by definition != NaN.
			if math.IsNaN(got.(float64)) {
				want, got = 1, 1
			} else {
				want = math.NaN()
			}
		case "-Infinity":
			want = math.Inf(-1)
		case "Infinity":
			want = math.Inf(1)
		default:
			want, _ = strconv.ParseFloat(v, 64)
		}

	case "beam:coder:kv:v1":
		v := eg.Value.(yaml.MapSlice)
		pass := true
		if !diff(c.Components[0], &exec.FullValue{Elm: elem.Elm}, v[0]) {
			pass = false
		}
		if !diff(c.Components[1], &exec.FullValue{Elm: elem.Elm2}, v[1]) {
			pass = false
		}
		return pass

	case "beam:coder:iterable:v1":
		pass := true
		gotrv := reflect.ValueOf(elem.Elm)
		wantrv := reflect.ValueOf(eg.Value)
		if gotrv.Len() != wantrv.Len() {
			log.Printf("Lengths don't match. got %v, want %v;  %v, %v", gotrv.Len(), wantrv.Len(), gotrv, wantrv)
			return false
		}
		for i := 0; i < wantrv.Len(); i++ {
			if !diff(c.Components[0],
				&exec.FullValue{Elm: gotrv.Index(i).Interface()},
				yaml.MapItem{Value: wantrv.Index(i).Interface()}) {
				pass = false
			}

		}
		return pass
	case "beam:coder:interval_window:v1":
		var a, b int
		val := eg.Value
		if is, ok := eg.Value.([]interface{}); ok {
			val = is[0]
		}
		v := val.(yaml.MapSlice)

		a = v[0].Value.(int)
		b = v[1].Value.(int)
		end := mtime.FromMilliseconds(int64(a))
		start := end - mtime.Time(int64(b))
		want = window.IntervalWindow{Start: start, End: end}
		// If this is nested in an iterable, windows won't be populated.
		if len(elem.Windows) == 0 {
			got = elem.Elm
		} else {
			got = elem.Windows[0]
		}

	case "beam:coder:global_window:v1":
		want = window.GlobalWindow{}
		// If this is nested in an iterable, windows won't be populated.
		if len(elem.Windows) == 0 {
			got = window.GlobalWindow(elem.Elm.(struct{}))
		} else {
			got = elem.Windows[0]
		}
	case "beam:coder:windowed_value:v1", "beam:coder:param_windowed_value:v1":
		// elem contains all the information, but we need to compare the element+timestamp
		// separately from the windows, to avoid repeated expected value parsing logic.
		pass := true
		vs := eg.Value.(yaml.MapSlice)
		if !diff(c.Components[0], elem, vs[0]) {
			pass = false
		}
		if d := cmp.Diff(
			mtime.FromMilliseconds(int64(vs[1].Value.(int))),
			elem.Timestamp, cmpOpts...); d != "" {

			pass = false
		}
		if !diff(c.Components[1], elem, vs[3]) {
			pass = false
		}
		// TODO compare pane information.
		return pass
	case "beam:coder:row:v1":
		fs := eg.Value.(yaml.MapSlice)
		var rfs []reflect.StructField
		// There are only 2 pointer examples, but they reuse field names,
		// so we key off the proto hash to know which example we're handling.
		ptrEg := strings.Contains(c.Payload, "51ace21c7393")
		for _, rf := range fs {
			name := rf.Key.(string)
			t := nameToType[name]
			if ptrEg {
				t = reflect.PtrTo(t)
			}
			rfs = append(rfs, reflect.StructField{
				Name: strings.ToUpper(name[:1]) + name[1:],
				Type: t,
				Tag:  reflect.StructTag(fmt.Sprintf("beam:\"%v\"", name)),
			})
		}
		rv := reflect.New(reflect.StructOf(rfs)).Elem()
		for i, rf := range fs {
			setField(rv, i, rf.Value)
		}

		got, want = elem.Elm, rv.Interface()
	default:
		got, want = elem.Elm, eg.Value
	}
	if d := cmp.Diff(want, got, cmpOpts...); d != "" {
		log.Printf("Decoding error: diff(-want,+got): %v\n", d)
		return false
	}
	return true
}

// standard_coders.yaml uses the name for type indication, except for nullability.
var nameToType = map[string]reflect.Type{
	"str": reflectx.String,
	"i32": reflectx.Int32,
	"f64": reflectx.Float64,
	"arr": reflect.SliceOf(reflectx.String),
}

func setField(rv reflect.Value, i int, v interface{}) {
	if v == nil {
		return
	}
	rf := rv.Field(i)
	if rf.Kind() == reflect.Ptr {
		// Ensure it's initialized.
		rf.Set(reflect.New(rf.Type().Elem()))
		rf = rf.Elem()
	}
	switch rf.Kind() {
	case reflect.String:
		rf.SetString(v.(string))
	case reflect.Int32:
		rf.SetInt(int64(v.(int)))
	case reflect.Float64:
		c, err := strconv.ParseFloat(v.(string), 64)
		if err != nil {
			panic(err)
		}
		rf.SetFloat(c)
	case reflect.Slice:
		// Value is a []interface{} with string values.
		var arr []string
		for _, a := range v.([]interface{}) {
			arr = append(arr, a.(string))
		}
		rf.Set(reflect.ValueOf(arr))
	}
}

func (s *Spec) parseCoder(c Coder) string {
	id := s.nextID()
	var compIDs []string
	for _, comp := range c.Components {
		compIDs = append(compIDs, s.parseCoder(comp))
	}
	s.coderPBs[id] = &pipepb.Coder{
		Spec: &pipepb.FunctionSpec{
			Urn:     c.Urn,
			Payload: []byte(c.Payload),
		},
		ComponentCoderIds: compIDs,
	}
	return id
}

func main() {
	data, err := ioutil.ReadFile(yamlPath)
	if err != nil {
		log.Fatalf("Couldn't read %v: %v", yamlPath, err)
	}
	specs := bytes.Split(data, []byte("\n---\n"))

	for _, data := range specs {
		var cs Spec
		if err := yaml.Unmarshal(data, &cs); err != nil {
			log.Printf("unable to parse yaml: %v %q", err, data)
			continue
		}
		if err := cs.testStandardCoder(); err != nil {
			log.Printf("Failed %q: %v", cs.Coder, err)
		}
	}
}
