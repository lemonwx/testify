package suite

import (
	"flag"
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"runtime/debug"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var allTestsFilter = func(_, _ string) (bool, error) { return true, nil }
var matchMethod = flag.String("testify.m", "", "regular expression to select tests of the testify suite to run")

// Suite is a basic testing suite with methods for storing and
// retrieving the current *testing.T context.
type Suite struct {
	*assert.Assertions
	require *require.Assertions
	t       *testing.T
}

// T retrieves the current *testing.T context.
func (suite *Suite) T() *testing.T {
	return suite.t
}

// SetT sets the current *testing.T context.
func (suite *Suite) SetT(t *testing.T) {
	suite.t = t
	suite.Assertions = assert.New(t)
	suite.require = require.New(t)
}

// Require returns a require context for suite.
func (suite *Suite) Require() *require.Assertions {
	if suite.require == nil {
		suite.require = require.New(suite.T())
	}
	return suite.require
}

// Assert returns an assert context for suite.  Normally, you can call
// `suite.NoError(expected, actual)`, but for situations where the embedded
// methods are overridden (for example, you might want to override
// assert.Assertions with require.Assertions), this method is provided so you
// can call `suite.Assert().NoError()`.
func (suite *Suite) Assert() *assert.Assertions {
	if suite.Assertions == nil {
		suite.Assertions = assert.New(suite.T())
	}
	return suite.Assertions
}

func failOnPanic(t *testing.T) {
	r := recover()
	if r != nil {
		t.Errorf("test panicked: %v\n%s", r, debug.Stack())
		t.FailNow()
	}
}

// Run provides suite functionality around golang subtests.  It should be
// called in place of t.Run(name, func(t *testing.T)) in test suite code.
// The passed-in func will be executed as a subtest with a fresh instance of t.
// Provides compatibility with go test pkg -run TestSuite/TestName/SubTestName.
func (suite *Suite) Run(name string, subtest func()) bool {
	oldT := suite.T()
	defer suite.SetT(oldT)
	return oldT.Run(name, func(t *testing.T) {
		suite.SetT(t)
		subtest()
	})
}

// Run takes a testing suite and runs all of the tests attached
// to it.
func Run(t *testing.T, suite TestingSuite) {
	log.SetFlags(log.Llongfile)
	log.Default().Println("1")
	defer failOnPanic(t)
	log.Default().Println("1")

	suite.SetT(t)

	var suiteSetupDone bool

	var stats *SuiteInformation
	if _, ok := suite.(WithStats); ok {
		stats = newSuiteInformation()
	}

	log.Default().Println("1")
	tests := []testing.InternalTest{}
	methodFinder := reflect.TypeOf(suite)
	suiteName := methodFinder.Elem().Name()

	log.Default().Println("1", methodFinder.NumMethod())
	for i := 0; i < methodFinder.NumMethod(); i++ {
		method := methodFinder.Method(i)

		ok, err := methodFilter(method.Name)
		log.Default().Println("1", i, methodFinder.NumMethod(), ok,err, method.Name, )
		if err != nil {
			fmt.Fprintf(os.Stderr, "testify: invalid regexp for -m: %s\n", err)
			os.Exit(1)
		}

		if !ok {
			continue
		}

		log.Default().Println("set up", suiteSetupDone)
		if !suiteSetupDone {
			if stats != nil {
				stats.Start = time.Now()
			}

			if setupAllSuite, ok := suite.(SetupAllSuite); ok {
				log.Default().Println("set up", suiteSetupDone)
				setupAllSuite.SetupSuite()
				log.Default().Println("set up", suiteSetupDone)
			}

			suiteSetupDone = true
		}
		log.Default().Println("set up", suiteSetupDone)

		test := testing.InternalTest{
			Name: method.Name,
			F: func(t *testing.T) {
				parentT := suite.T()
				suite.SetT(t)
				defer failOnPanic(t)
				defer func() {
					if stats != nil {
						passed := !t.Failed()
						stats.end(method.Name, passed)
					}

					if afterTestSuite, ok := suite.(AfterTest); ok {
						afterTestSuite.AfterTest(suiteName, method.Name)
					}

					if tearDownTestSuite, ok := suite.(TearDownTestSuite); ok {
						tearDownTestSuite.TearDownTest()
					}

					suite.SetT(parentT)
				}()

				if setupTestSuite, ok := suite.(SetupTestSuite); ok {
					setupTestSuite.SetupTest()
				}
				if beforeTestSuite, ok := suite.(BeforeTest); ok {
					beforeTestSuite.BeforeTest(methodFinder.Elem().Name(), method.Name)
				}

				if stats != nil {
					stats.start(method.Name)
				}

				method.Func.Call([]reflect.Value{reflect.ValueOf(suite)})
			},
		}
		tests = append(tests, test)
	}
	if suiteSetupDone {
		defer func() {
			if tearDownAllSuite, ok := suite.(TearDownAllSuite); ok {
				tearDownAllSuite.TearDownSuite()
			}

			if suiteWithStats, measureStats := suite.(WithStats); measureStats {
				stats.End = time.Now()
				suiteWithStats.HandleStats(suiteName, stats)
			}
		}()
	}
	log.Default().Println("1", tests)
	runTests(t, tests)
	log.Default().Println("1", tests)
}

// Filtering method according to set regular expression
// specified command-line argument -m
func methodFilter(name string) (bool, error) {
	if ok, _ := regexp.MatchString("^Test", name); !ok {
		return false, nil
	}
	return regexp.MatchString(*matchMethod, name)
}

func runTests(t testing.TB, tests []testing.InternalTest) {
	log.Default().Println("1", tests)
	if len(tests) == 0 {
		t.Log("warning: no tests to run")
		return
	}

	r, ok := t.(runner)
	log.Default().Println("1", tests,ok)
	if !ok { // backwards compatibility with Go 1.6 and below
		if !testing.RunTests(allTestsFilter, tests) {
			log.Default().Println("1", tests)
			t.Fail()
		}
		return
	}

	log.Default().Println("1", tests)
	for _, test := range tests {
		log.Default().Println("1", test)
		r.Run(test.Name, test.F)
		log.Default().Println("1", test)
	}
	log.Default().Println("1", test)
}

type runner interface {
	Run(name string, f func(t *testing.T)) bool
}
