## NRI e2e test with kind

### How to test e2e

```
$ git clone https://github.com/k8snetworkplumbingwg/network-resources-injector.git
$ cd network-resources-injector/
$ ./scripts/e2e_get_tools.sh
$ ./scripts/e2e_setup_cluster.sh
$ go test ./test/e2e/...
```

### How to teardown cluster

```
$ ./scripts/e2e_teardown.sh
```

### How to cleanup test artifacts

```
$ ./scripts/e2e_cleanup.sh
```

### How to change default test image
By default, ```alpline:latest``` image is used as a base test image. To override this
set environment variables ```IMAGE_REGISTRY```and ```TEST_IMAGE```. For example:

```
$ export IMAGE_REGISTRY=localhost:5000
$ export TEST_IMAGE=nginx:latest
```

### Current test cases
* Test injection of one network
* Test injection of two networks
