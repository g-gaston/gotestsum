package testjson

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
)

func TestPackage_Elapsed(t *testing.T) {
	pkg := &Package{
		Failed: []TestCase{
			{Elapsed: 300 * time.Millisecond},
		},
		Passed: []TestCase{
			{Elapsed: 200 * time.Millisecond},
			{Elapsed: 2500 * time.Millisecond},
		},
		Skipped: []TestCase{
			{Elapsed: 100 * time.Millisecond},
		},
	}
	assert.Equal(t, pkg.Elapsed(), 3100*time.Millisecond)
}

func TestExecution_Add_PackageCoverage(t *testing.T) {
	exec := newExecution()
	exec.add(TestEvent{
		Package: "mytestpkg",
		Action:  ActionOutput,
		Output:  "coverage: 33.1% of statements\n",
	})

	pkg := exec.Package("mytestpkg")
	expected := &Package{
		coverage: "coverage: 33.1% of statements",
		output: map[int][]string{
			0: {"coverage: 33.1% of statements\n"},
		},
		running: map[string]TestCase{},
	}
	assert.DeepEqual(t, pkg, expected, cmpPackage)
}

var cmpPackage = cmp.Options{
	cmp.AllowUnexported(Package{}),
	cmpopts.EquateEmpty(),
}

func TestScanTestOutput_MinimalConfig(t *testing.T) {
	in := bytes.NewReader(golden.Get(t, "go-test-json.out"))
	exec, err := ScanTestOutput(ScanConfig{Stdout: in})
	assert.NilError(t, err)
	// a weak check to show that all the stdout was scanned
	assert.Equal(t, exec.Total(), 46)
}

func TestScanTestOutput_CallsStopOnError(t *testing.T) {
	var called bool
	stop := func() {
		called = true
	}
	cfg := ScanConfig{
		Stdout:  bytes.NewReader(golden.Get(t, "go-test-json.out")),
		Handler: &handlerFails{},
		Stop:    stop,
	}
	_, err := ScanTestOutput(cfg)
	assert.Error(t, err, "something failed")
	assert.Assert(t, called)
}

type handlerFails struct {
	count int
}

func (s *handlerFails) Event(_ TestEvent, _ *Execution) error {
	if s.count > 1 {
		return fmt.Errorf("something failed")
	}
	s.count++
	return nil
}

func (s *handlerFails) Err(_ string) error {
	return nil
}

func TestParseEvent(t *testing.T) {
	// nolint: lll
	raw := `{"Time":"2018-03-22T22:33:35.168308334Z","Action":"output","Package":"example.com/good","Test": "TestOk","Output":"PASS\n"}`
	event, err := parseEvent([]byte(raw))
	assert.NilError(t, err)
	expected := TestEvent{
		Time:    time.Date(2018, 3, 22, 22, 33, 35, 168308334, time.UTC),
		Action:  "output",
		Package: "example.com/good",
		Test:    "TestOk",
		Output:  "PASS\n",
		raw:     []byte(raw),
	}
	cmpTestEvent := cmp.AllowUnexported(TestEvent{})
	assert.DeepEqual(t, event, expected, cmpTestEvent)
}

func TestPackage_AddEvent(t *testing.T) {
	type testCase struct {
		name     string
		event    string
		expected Package
	}

	var cmpPackage = cmp.Options{
		cmp.AllowUnexported(Package{}),
		cmpopts.EquateEmpty(),
	}

	run := func(t *testing.T, tc testCase) {
		te, err := parseEvent([]byte(tc.event))
		assert.NilError(t, err)

		p := newPackage()
		p.addEvent(te)
		assert.DeepEqual(t, p, &tc.expected, cmpPackage)
	}

	var testCases = []testCase{
		{
			name:  "coverage with -cover",
			event: `{"Action":"output","Package":"gotest.tools/testing","Output":"coverage: 4.2% of statements\n"}`,
			expected: Package{
				coverage: "coverage: 4.2% of statements",
				output:   pkgOutput(0, "coverage: 4.2% of statements\n"),
			},
		},
		{
			name:  "coverage with -coverpkg",
			event: `{"Action":"output","Package":"gotest.tools/testing","Output":"coverage: 7.5% of statements in ./testing\n"}`,
			expected: Package{
				coverage: "coverage: 7.5% of statements in ./testing",
				output:   pkgOutput(0, "coverage: 7.5% of statements in ./testing\n"),
			},
		},
		{
			name:     "package failed",
			event:    `{"Action":"fail","Package":"gotest.tools/testing","Elapsed":0.012}`,
			expected: Package{action: ActionFail},
		},
		{
			name:  "package is cached",
			event: `{"Action":"output","Package":"gotest.tools/testing","Output":"ok  \tgotest.tools/testing\t(cached)\n"}`,
			expected: Package{
				cached: true,
				output: pkgOutput(0, "ok  \tgotest.tools/testing\t(cached)\n"),
			},
		},
		{
			name:     "package pass",
			event:    `{"Action":"pass","Package":"gotest.tools/testing","Elapsed":0.012}`,
			expected: Package{action: ActionPass},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func pkgOutput(id int, line string) map[int][]string {
	return map[int][]string{id: {line}}
}
