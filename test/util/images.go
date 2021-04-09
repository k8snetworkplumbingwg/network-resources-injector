package util

import (
	"fmt"
	"os"
)

var (
	registry string
	testImage string
)

func init() {
	registry = os.Getenv("IMAGE_REGISTRY")
	if registry == "" {
		registry = "docker.io"
	}
	testImage = os.Getenv("TEST_IMAGE")
	if testImage == "" {
		testImage = "alpine"
	}
}

//GetPodTestImage returns image to be used during testing
func GetPodTestImage() string {
	return fmt.Sprintf("%s/%s", registry, testImage)
}
