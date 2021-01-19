package webhook

import (
	"errors"
	"time"

	nriMocks "github.com/k8snetworkplumbingwg/network-resources-injector/pkg/types/mocks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("mutate HTTP server", func() {
	t := GinkgoT()
	Describe("Service interface implementation for HTTP server", func() {
		const to = time.Millisecond * 50
		var (
			mutateSrv *mutateServer
			srvMock   *nriMocks.Server
		)
		BeforeEach(func() {
			mutateSrv = &mutateServer{&nriMocks.Server{}, to, NewChannel()}
			srvMock = &nriMocks.Server{}
			mutateSrv.instance = srvMock
		})
		Context("Run()", func() {
			It("should start server", func() {
				srvMock.On("Start").Return(nil)
				Expect(mutateSrv.Run()).To(BeNil())
				srvMock.AssertCalled(t, "Start")
			})
			It("should return error from server if startup generated an error", func() {
				expErr := errors.New("bad start of server")
				srvMock.On("Start").Return(expErr)
				Expect(mutateSrv.Run()).To(Equal(expErr))
			})
		})
		Context("Quit()", func() {
			It("should stop server", func() {
				srvMock.On("Stop", to).Return(nil)
				mutateSrv.status.Close() // Close in advance of call to ensure we do not get timeout error
				Expect(mutateSrv.Quit()).To(BeNil())
				srvMock.AssertCalled(t, "Stop", to)
			})
			It("should return error if shutdown generated an error", func() {
				expErr := errors.New("bad stop of server")
				srvMock.On("Stop", to).Return(expErr)
				mutateSrv.status.Close() // Close in advance of call to ensure we do not get timeout error
				Expect(mutateSrv.Quit()).To(Equal(expErr))
			})
		})
	})
	Describe("creation of new mutate server", func() {
		const (
			address  = "127.0.0.1"
			port     = 12345
			to       = time.Millisecond * 2
			insecure = false
		)
		var (
			pool    *nriMocks.ClientCAPool
			keyPair *nriMocks.KeyReloader
		)
		BeforeEach(func() {
			pool = &nriMocks.ClientCAPool{}
			pool.On("GetCertPool").Return(nil)
			keyPair = &nriMocks.KeyReloader{}
			keyPair.On("GetCertificateFunc").Return(nil)
		})
		Context("NewMutateServer()", func() {
			It("should retrieve cert pool", func() {
				NewMutateServer(address, port, insecure, to, to, to, to, pool, keyPair)
				pool.AssertCalled(t, "GetCertPool")
			})
			It("should retrieve certificate function", func() {
				NewMutateServer(address, port, insecure, to, to, to, to, pool, keyPair)
				keyPair.AssertCalled(t, "GetCertificateFunc")
			})
		})
	})
})
