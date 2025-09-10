package lib

type TestState string

const (
	TestNotStarted TestState = "TestNotStarted"
	TestRunning    TestState = "TestRunning"
	TestPassed     TestState = "TestPassed"
	TestFailed     TestState = "TestFailed"
)

type TestID int

const (
	BasicConnectivityTest               TestID = 1
	BasicMultipathTest                  TestID = 2
	MinimizeCarbonIntensity             TestID = 10
	MaximizeBandwidthWithBoundedLatency TestID = 11
	FabridConnectivityTest              TestID = 20
	FabridPolicy1Test                   TestID = 21
	FabridPolicy2Test                   TestID = 22
	FabridPolicy3Test                   TestID = 23
	ASFinderTest                        TestID = 30
	EpicHiddenPathTest                  TestID = 40
)

type Test struct {
	ID      TestID `json:"ID"`
	Payload any    `json:"Payload"`
}

type TestResult struct {
	ID      TestID    `json:"ID"`
	Payload any       `json:"Payload"`
	State   TestState `json:"State"`
}
