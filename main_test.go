package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var (
	goodServer           *httptest.Server
	goodEventuallyServer *httptest.Server
	badServer            *httptest.Server
	genericQuitServer    *httptest.Server
	slowQuitServer       *httptest.Server
	callCount            counter
	envoyDelayTimestamp  int64 = 0
	envoyDelayMax        int64 = 15
)

type counter int64

func (c *counter) increment() int {
	atomic.AddInt64((*int64)(c), 1)
	return c.int()
}

func (c *counter) int() int {
	return int(*c)
}

func TestMain(m *testing.M) {
	initTestHTTPServers()
	os.Exit(m.Run())
}

func initTestHTTPServers() {
	fmt.Println("Initiating test HTTP servers")

	// Always 200 and live envoy state
	goodServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.increment()
		w.Write([]byte("{\"state\": \"LIVE\"}")) // Envoy live response
	}))

	// 503 for 5 requests, then 200 + live envoy state
	goodEventuallyServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.increment()
		timeSinceStarted := time.Now().Unix() - envoyDelayTimestamp
		if timeSinceStarted < envoyDelayMax {
			fmt.Println("Status Unavailable")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("{\"state\": \"LIVE\"}")) // Envoy live response
	}))

	// Always 503
	badServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.increment()
		fmt.Println("Status Unavailable")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))

	genericQuitServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.increment()
		fmt.Println("Status Ok")
		w.WriteHeader(http.StatusOK)
	}))

	slowQuitServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.increment()
		time.Sleep(2 * time.Second)
		fmt.Println("Status Ok")
		w.WriteHeader(http.StatusOK)
	}))
}

// Sets up minimum env variables and sets the global config from env.
func initTestingEnv() {
	// Always update env variables for new test
	os.Setenv("SCUTTLE_LOGGING", "true")
	config = getConfig()
}

// Reset all the environment variables
func clearTestingEnv() {
	os.Setenv("START_WITHOUT_ENVOY", "")
	os.Setenv("ENVOY_ADMIN_API", "")
	os.Setenv("QUIT_WITHOUT_ENVOY_TIMEOUT", "")
	os.Setenv("WAIT_FOR_ENVOY_TIMEOUT", "")
	os.Setenv("GENERIC_QUIT_ENDPOINTS", "")
	os.Setenv("QUIT_REQUEST_TIMEOUT", "")
	callCount = 0
}

// Inits the test environment and starts the blocking
// Set any env variables for your specific tests prior to calling this
// Pass in a negative integer to block but skip kill
func initAndRun(exitCode int) {
	initTestingEnv()
	if blockingCtx := waitForEnvoy(); blockingCtx != nil {
		<-blockingCtx.Done()
		err := blockingCtx.Err()
		defer clearTestingEnv()
		if err == nil || errors.Is(err, context.Canceled) {
			log("Blocking finished, Envoy has started")
		} else if errors.Is(err, context.DeadlineExceeded) {
			panic(errors.New("timeout reached while waiting for Envoy to start"))
		} else {
			panic(err.Error())
		}
	}
	if exitCode >= 0 {
		kill(exitCode)
	}
}

// Tests START_WITHOUT_ENVOY works with failing envoy mock server
func TestBlockingDisabled(t *testing.T) {
	fmt.Println("Starting TestBlockingDisabled")
	os.Setenv("START_WITHOUT_ENVOY", "true")
	initAndRun(-1)
	// If your tests hang and never finish, this test "failed"
	// Also try go test -timeout <seconds>s
}

// Tests block function with working envoy mock server
func TestBlockingEnabled(t *testing.T) {
	fmt.Println("Starting TestBlockingEnabled")
	os.Setenv("START_WITHOUT_ENVOY", "false")
	os.Setenv("ENVOY_ADMIN_API", goodServer.URL)
	initAndRun(-1)
}

// Tests block function with envoy mock server that fails for 15 seconds, then works
func TestSlowEnvoy(t *testing.T) {
	fmt.Println("Starting TestSlowEnvoy")
	os.Setenv("START_WITHOUT_ENVOY", "false")
	os.Setenv("ENVOY_ADMIN_API", goodEventuallyServer.URL)
	envoyDelayTimestamp = time.Now().Unix()
	initAndRun(-1)
}

// Tests generic quit endpoints are sent
func TestGenericQuitEndpoints(t *testing.T) {
	fmt.Println("Starting TestGenericQuitEndpoints")
	os.Setenv("START_WITHOUT_ENVOY", "false")
	os.Setenv("ENVOY_ADMIN_API", goodServer.URL)
	// Valid URLs don't matter, just need something that will generate an HTTP response
	// 127.0.0.1:1111/idontexist is to verify we don't panic if a nonexistent URL is given
	// notaurl^^ is to verify a malformatted URL does not result in panic
	os.Setenv("GENERIC_QUIT_ENDPOINTS", genericQuitServer.URL+", https://google.com/, https://github.com/, 127.0.0.1:1111/idontexist, notaurl^^ ")
	initTestingEnv()
	killGenericEndpoints()
	if callCount != 1 {
		t.Errorf("Expected 1 call to genericQuitServer got %d", callCount)
	}
	clearTestingEnv()
}

// Tests GenericQuitOnly triggers GenericQuitEndpoints
func TestGenericQuitOnly(t *testing.T) {
	var quitEndpointCalled bool
	fmt.Println("Starting TestGenericQuitOnly")
	checkCallServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		quitEndpointCalled = true
	}))
	os.Setenv("GENERIC_QUIT_ONLY", "true")
	os.Setenv("GENERIC_QUIT_ENDPOINTS", fmt.Sprintf("http://%s", checkCallServer.Listener.Addr().String()))
	initTestingEnv()
	kill(0)
	if quitEndpointCalled == false {
		t.Error("Expected GENERIC_QUIT_ONLY to trigger GENERIC_QUIT_ENDPOINTS")
	}
	checkCallServer.Close()
}

// Tests scuttle does not fail when the /quitquitquit endpoint does not return a response
func TestNoQuitQuitQuitResponse(t *testing.T) {
	fmt.Println("Starting TestNoQuitQuitQuitResponse")
	os.Setenv("START_WITHOUT_ENVOY", "false")
	os.Setenv("ENVOY_ADMIN_API", goodServer.URL)
	os.Setenv("ISTIO_QUIT_API", "127.0.0.1:1111/idontexist")
	initTestingEnv()
	killIstioWithAPI()
	clearTestingEnv()
}

// Tests scuttle does not fail when the /quitquitquit endpoint is not a valid URL
func TestNoQuitQuitQuitMalformedUrl(t *testing.T) {
	fmt.Println("Starting TestNoQuitQuitQuitMalformedUrl")
	os.Setenv("START_WITHOUT_ENVOY", "false")
	os.Setenv("ENVOY_ADMIN_API", goodServer.URL)
	os.Setenv("ISTIO_QUIT_API", "notaurl^^")
	initTestingEnv()
	killIstioWithAPI()
	clearTestingEnv()
}

func TestQuitTimeout(t *testing.T) {
	fmt.Println("Starting TestQuitTimeout")
	os.Setenv("START_WITHOUT_ENVOY", "false")
	os.Setenv("ENVOY_ADMIN_API", goodServer.URL)
	os.Setenv("ISTIO_QUIT_API", slowQuitServer.URL)
	os.Setenv(
		"GENERIC_QUIT_ENDPOINTS",
		strings.Join([]string{slowQuitServer.URL, slowQuitServer.URL, genericQuitServer.URL, slowQuitServer.URL}, ", "),
	)
	os.Setenv("QUIT_REQUEST_TIMEOUT", "100ms")

	measureCheckFunc := func(targetFunc func(), errorPrefix string) {
		startCallCount := callCount
		startTime := time.Now()
		targetFunc()
		elapsedTime := time.Now().Sub(startTime)
		if elapsedTime > 500*time.Millisecond {
			t.Errorf("%s: took %dms, this exceeds the timeout significantly", errorPrefix, elapsedTime/time.Millisecond)
		}
		if callCount-startCallCount < 1 {
			t.Errorf("%s: quit endpoint was not called", errorPrefix)
		}
	}
	initTestingEnv()
	measureCheckFunc(killIstioWithAPI, "killIstioWithAPI()")
	measureCheckFunc(killGenericEndpoints, "killGenericEndpoints()")
	clearTestingEnv()
}

// Tests scuttle waits
func TestWaitTillTimeoutForEnvoy(t *testing.T) {
	fmt.Println("Starting TestWaitTillTimeoutForEnvoy")
	os.Setenv("START_WITHOUT_ENVOY", "false")
	os.Setenv("QUIT_WITHOUT_ENVOY_TIMEOUT", "500ms")
	os.Setenv("ENVOY_ADMIN_API", badServer.URL)
	initTestingEnv()
	defer clearTestingEnv()
	dur, _ := time.ParseDuration("500ms")
	config.QuitWithoutEnvoyTimeout = dur
	blockingCtx := waitForEnvoy()
	if blockingCtx == nil {
		t.Fatal("Blocking context was nil")
	}
	select {
	case <-time.After(1 * time.Second):
		t.Fatal("Context did not timeout")
	case <-blockingCtx.Done():
		if !errors.Is(blockingCtx.Err(), context.DeadlineExceeded) {
			t.Fatalf("Context contains wrong error: %s", blockingCtx.Err())
		}
	}
}

// Tests scuttle will continue after WAIT_FOR_ENVOY_TIMEOUT expires and envoy is not ready
func TestWaitForEnvoyTimeoutContinueWithoutEnvoy(t *testing.T) {
	fmt.Println("Starting TestWaitForEnvoyTimeoutContinueWithoutEnvoy")
	os.Setenv("START_WITHOUT_ENVOY", "false")
	os.Setenv("WAIT_FOR_ENVOY_TIMEOUT", "5s")
	os.Setenv("ENVOY_ADMIN_API", badServer.URL)
	initTestingEnv()
	defer clearTestingEnv()
	blockingCtx := waitForEnvoy()
	<-blockingCtx.Done()
	err := blockingCtx.Err()
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		fmt.Println("TestWaitForEnvoyTimeoutContinueWithoutEnvoy err", err)
		// Err is nil (envoy is up)
		// or Err is set, but is not a cancellation err
		// we expect a cancellation when the time is up
		t.Fail()
	}
}
