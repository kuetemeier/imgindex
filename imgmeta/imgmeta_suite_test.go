package imgmeta_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestImgmeta(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Imgmeta Suite")
}
