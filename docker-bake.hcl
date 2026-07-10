variable "IMAGE_PREFIX" {
  default = ""
}

variable "IMAGE_TAG" {
  default = "latest"
}

variable "TOPOLVM_VERSION" {
  default = "devel"
}

variable "PLATFORMS" {
  default = ["linux/amd64", "linux/arm64/v8", "linux/ppc64le"]
}

group "default" {
  targets = ["images"]
}

group "images" {
  targets = ["topolvm", "topolvm-with-sidecar"]
}

group "multi-platform-images" {
  targets = ["topolvm-multi-platform", "topolvm-with-sidecar-multi-platform"]
}

target "_common" {
  context    = "."
  dockerfile = "Dockerfile"

  args = {
    TOPOLVM_VERSION = TOPOLVM_VERSION
  }
}

target "topolvm" {
  inherits = ["_common"]
  target   = "topolvm"
  tags     = ["${IMAGE_PREFIX}topolvm:${IMAGE_TAG}"]
}

target "topolvm-with-sidecar" {
  inherits = ["_common"]
  target   = "topolvm-with-sidecar"
  tags     = ["${IMAGE_PREFIX}topolvm-with-sidecar:${IMAGE_TAG}"]
}

target "topolvm-multi-platform" {
  inherits  = ["topolvm"]
  platforms = PLATFORMS
}

target "topolvm-with-sidecar-multi-platform" {
  inherits  = ["topolvm-with-sidecar"]
  platforms = PLATFORMS
}
