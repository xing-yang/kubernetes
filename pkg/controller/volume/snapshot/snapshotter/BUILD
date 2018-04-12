load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["snapshotter.go"],
    importpath = "k8s.io/kubernetes/pkg/controller/volume/snapshot/snapshotter",
    visibility = ["//visibility:public"],
    deps = [
        "//pkg/controller/volume/snapshot/cache:go_default_library",
        "//pkg/util/goroutinemap:go_default_library",
        "//pkg/util/goroutinemap/exponentialbackoff:go_default_library",
        "//pkg/volume:go_default_library",
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/uuid:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
