package webhook

import (
	"errors"
	"io/ioutil"
	"os"
	"time"

	nriMocks "github.com/k8snetworkplumbingwg/network-resources-injector/pkg/types/mocks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("cert & key watcher", func() {
	t := GinkgoT()
	const (
		to        = time.Millisecond * 10
		interval  = to
		keyFName  = "nri-watcher-test-key"
		certFName = "nri-watcher-test-cert"
		TempDir   = "/tmp"
	)
	var (
		keyPair *nriMocks.KeyReloader
		kcw     *keyPairWatcher
		certF   *os.File
		keyF    *os.File
	)
	BeforeEach(func() {
		keyPair = &nriMocks.KeyReloader{}
		certF, _ = ioutil.TempFile(TempDir, certFName)
		keyF, _ = ioutil.TempFile(TempDir, keyFName)
		kcw = &keyPairWatcher{nil, nil, to, keyPair}
		keyPair.On("GetCertPath").Return(certF.Name())
		keyPair.On("GetKeyPath").Return(keyF.Name())
	})

	AfterEach(func() {
		kcw.Quit()
		os.Remove(certF.Name())
		os.Remove(keyF.Name())
	})

	Context("Run()", func() {
		It("should retrieve cert and key path", func() {
			keyPair.On("Reload").Return(nil)
			kcw.Run()
			keyPair.AssertCalled(t, "GetCertPath")
			keyPair.AssertCalled(t, "GetKeyPath")
		})
		It("should return error if cert doesn't exist", func() {
			os.Remove(certF.Name())
			Expect(kcw.Run().Error()).To(ContainSubstring("cert file does not exist"))
		})
		It("should return error if key doesn't exist", func() {
			os.Remove(keyF.Name())
			Expect(kcw.Run().Error()).To(ContainSubstring("key file does not exist"))
		})
		It("should not reload cert/key if only key is altered", func() {
			kcw.Run()
			os.Chtimes(certF.Name(), time.Now(), time.Now()) // touch file
			time.Sleep(interval)                             // wait for Reload function to be possibly called
			keyPair.AssertNotCalled(t, "Reload")
		})
		It("should not reload cert/key if only cert is altered", func() {
			kcw.Run()
			os.Chtimes(keyF.Name(), time.Now(), time.Now()) // touch file
			time.Sleep(interval)                            // wait for Reload function to be possibly called
			keyPair.AssertNotCalled(t, "Reload")
		})
		It("should reload cert/key if cert and key are altered", func() {
			keyPair.On("Reload").Return(nil)
			kcw.Run()
			os.Chtimes(certF.Name(), time.Now(), time.Now()) // touch file
			os.Chtimes(keyF.Name(), time.Now(), time.Now())
			time.Sleep(interval) // wait for Reload function to be called
			keyPair.AssertExpectations(t)
		})
		It("should terminate watcher when reload fails", func() {
			keyPair.On("Reload").Return(errors.New("failed to reload keys"))
			kcw.Run()
			os.Chtimes(certF.Name(), time.Now(), time.Now()) // touch file
			os.Chtimes(keyF.Name(), time.Now(), time.Now())
			time.Sleep(interval) // wait for Reload function to be called
			Expect(kcw.status.IsOpen()).To(BeFalse())
		})
		It("should tolerate restart", func() {
			kcw.Run()
			kcw.Quit()
			Expect(kcw.status.IsOpen()).To(BeFalse())
			kcw.Run() // restart
			Expect(kcw.status.IsOpen()).To(BeTrue())
			kcw.Quit()
			Expect(kcw.status.IsOpen()).To(BeFalse())
		})
	})

	Context("Quit()", func() {
		It("should terminate watcher", func() {
			kcw.Run()
			time.Sleep(interval)
			Expect(kcw.status.IsOpen()).To(BeTrue()) // ensure it is running before test
			Expect(kcw.Quit()).To(BeNil())
			Expect(kcw.status.IsClosed()).To(BeTrue())
		})
	})
})
