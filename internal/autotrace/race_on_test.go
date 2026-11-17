//go:build race

package autotrace

// raceDetector says the tests run under the race detector, which makes the
// pixel loops ten to twenty times slower.
const raceDetector = true
