package webhook

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Status", func() {
	var ch *Channel
	BeforeEach(func() {
		ch = NewChannel()
	})
	Context("NewChannel()", func() {
		It("should be down by default", func() {
			Expect(ch.IsClosed()).To(BeTrue())
		})
	})
	Context("Open()", func() {
		It("should set channel open", func() {
			ch.Open()
			Expect(ch.IsOpen()).To(BeTrue())
		})
	})
	Context("Close()", func() {
		It("should close channel", func() {
			ch.Open()
			ch.Close()
			Expect(ch.IsOpen()).To(BeFalse())
		})
	})
	Context("IsOpen()", func() {
		It("should return closed by default", func() {
			Expect(ch.IsOpen()).To(BeFalse())
		})
	})
	Context("IsClosed()", func() {
		It("should return true by default", func() {
			ch.Open()
			Expect(ch.IsClosed()).To(BeFalse())
		})
	})
	Context("WaitUntilClosed()", func() {
		It("should return nil if channel close under limit", func() {
			ch.Open()
			ch.Close()
			Expect(ch.WaitUntilClosed(interval)).To(BeNil())
		})
		It("should return error if channel is open after limit", func() {
			ch.Open()
			go func() {
				time.Sleep(interval * 10)
				ch.Close()
			}()
			Expect(ch.WaitUntilClosed(interval * 5).Error()).To(ContainSubstring("timed out"))
		})
	})
	Context("WaitUntilOpened()", func() {
		It("should return error if limit below min sleep interval time", func() {
			Expect(ch.WaitUntilOpened(time.Nanosecond).Error()).To(ContainSubstring("limit arg value too low"))
		})
		It("should return nil if channel open under limit", func() {
			ch.Open()
			Expect(ch.WaitUntilOpened(interval)).To(BeNil())
		})
		It("should return error if channel didnt open before interval limit", func() {
			go func() {
				time.Sleep(interval * 10)
				ch.Open()
			}()
			Expect(ch.WaitUntilOpened(interval * 5).Error()).To(ContainSubstring("timed out"))
		})
	})
})
