package webhook

import (
	"crypto/tls"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	cliflag "k8s.io/component-base/cli/flag"
)

var _ = Describe("TLS Configuration", func() {

	Describe("CipherNamesToIDs", func() {
		DescribeTable("valid cipher names",
			func(name string, expectedID uint16) {
				ids, err := CipherNamesToIDs([]string{name})
				Expect(err).NotTo(HaveOccurred())
				Expect(ids).To(HaveLen(1))
				Expect(ids[0]).To(Equal(expectedID))
			},
			Entry("TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256", tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256),
			Entry("TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256),
			Entry("TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
				"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384", tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384),
			Entry("TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
				"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384", tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384),
			Entry("TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256",
				"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256", tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256),
			Entry("TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
				"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256", tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256),
			Entry("legacy alias without SHA256 suffix",
				"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305", tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256),
			Entry("TLS 1.3: TLS_AES_128_GCM_SHA256",
				"TLS_AES_128_GCM_SHA256", tls.TLS_AES_128_GCM_SHA256),
			Entry("TLS 1.3: TLS_AES_256_GCM_SHA384",
				"TLS_AES_256_GCM_SHA384", tls.TLS_AES_256_GCM_SHA384),
			Entry("TLS 1.3: TLS_CHACHA20_POLY1305_SHA256",
				"TLS_CHACHA20_POLY1305_SHA256", tls.TLS_CHACHA20_POLY1305_SHA256),
		)

		It("should convert multiple cipher names", func() {
			names := []string{
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
			}
			ids, err := CipherNamesToIDs(names)
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(Equal([]uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			}))
		})

		It("should trim whitespace from names", func() {
			ids, err := CipherNamesToIDs([]string{" TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 "})
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(Equal([]uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256}))
		})

		It("should return an error for unknown cipher names", func() {
			_, err := CipherNamesToIDs([]string{"UNKNOWN-CIPHER"})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not supported"))
			Expect(err.Error()).To(ContainSubstring("UNKNOWN-CIPHER"))
		})

		It("should return an error if any cipher in the list is invalid", func() {
			_, err := CipherNamesToIDs([]string{
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				"INVALID",
				"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("INVALID"))
		})

		It("should reject insecure cipher names", func() {
			insecureCiphers := cliflag.InsecureTLSCiphers()
			if len(insecureCiphers) == 0 {
				Skip("current Go runtime does not expose insecure cipher suites")
			}

			insecureName := ""
			for name := range insecureCiphers {
				insecureName = name
				break
			}

			_, err := CipherNamesToIDs([]string{insecureName})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("insecure and not allowed"))
			Expect(err.Error()).To(ContainSubstring(insecureName))
		})

		It("should return an error for empty cipher names", func() {
			_, err := CipherNamesToIDs([]string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", ""})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("empty TLS cipher suite name"))
		})

		It("should return nil for empty input", func() {
			ids, err := CipherNamesToIDs([]string{})
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(BeNil())
		})
	})

	Describe("ParseTLSCipherSuites", func() {
		It("should return nil for empty input", func() {
			ids, err := ParseTLSCipherSuites("")
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(BeNil())
		})

		It("should return nil for whitespace-only input", func() {
			ids, err := ParseTLSCipherSuites("   ")
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(BeNil())
		})

		It("should parse comma-separated names", func() {
			ids, err := ParseTLSCipherSuites("TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384")
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(Equal([]uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			}))
		})

		It("should trim entries in comma-separated names", func() {
			ids, err := ParseTLSCipherSuites(" TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 ,  TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384 ")
			Expect(err).NotTo(HaveOccurred())
			Expect(ids).To(Equal([]uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			}))
		})

		It("should return an error for trailing separators", func() {
			_, err := ParseTLSCipherSuites("TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("empty TLS cipher suite name"))
		})

		It("should return an error for repeated separators", func() {
			_, err := ParseTLSCipherSuites("TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("empty TLS cipher suite name"))
		})
	})

	Describe("TLSVersionToGo", func() {
		DescribeTable("valid TLS versions",
			func(version string, expected uint16) {
				v, err := TLSVersionToGo(version)
				Expect(err).NotTo(HaveOccurred())
				Expect(v).To(Equal(expected))
			},
			Entry("VersionTLS12", "VersionTLS12", uint16(tls.VersionTLS12)),
			Entry("VersionTLS13", "VersionTLS13", uint16(tls.VersionTLS13)),
		)

		DescribeTable("rejected weak TLS versions",
			func(version string) {
				_, err := TLSVersionToGo(version)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("VersionTLS12 or higher"))
				Expect(err.Error()).To(ContainSubstring(version))
			},
			Entry("VersionTLS10", "VersionTLS10"),
			Entry("VersionTLS11", "VersionTLS11"),
		)

		It("should trim whitespace from version string", func() {
			v, err := TLSVersionToGo("  VersionTLS12  ")
			Expect(err).NotTo(HaveOccurred())
			Expect(v).To(Equal(uint16(tls.VersionTLS12)))
		})

		It("should return an error for unknown TLS versions", func() {
			_, err := TLSVersionToGo("VersionTLS99")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown tls version"))
			Expect(err.Error()).To(ContainSubstring("VersionTLS99"))
		})

		It("should return default version for empty string", func() {
			v, err := TLSVersionToGo("")
			Expect(err).NotTo(HaveOccurred())
			Expect(v).To(Equal(uint16(tls.VersionTLS12)))
		})

		It("should return default version for whitespace-only string", func() {
			v, err := TLSVersionToGo("   ")
			Expect(err).NotTo(HaveOccurred())
			Expect(v).To(Equal(uint16(tls.VersionTLS12)))
		})
	})
})
