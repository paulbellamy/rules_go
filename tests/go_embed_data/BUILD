load("@io_bazel_rules_go//go:def.bzl", "go_embed_data", "go_library", "go_test")

go_test(
    name = "go_default_test",
    srcs = ["embed_test.go"],
    data = [
        "BUILD",
        "//:README.md",
        "@io_bazel_rules_go_repository_tools//:BUILD.bazel",
    ],
    library = ":go_default_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "embed_empty.go",
        "embed_ext.go",
        "embed_flat.go",
        "embed_local.go",
        "embed_single.go",
        "embed_str.go",
    ],
)

go_embed_data(
    name = "empty",
    out = "embed_empty.go",
    package = "go_embed_data",
    var = "empty",
)

go_embed_data(
    name = "single",
    src = "//:README.md",
    out = "embed_single.go",
    package = "go_embed_data",
    var = "single",
)

go_embed_data(
    name = "local",
    srcs = [
        ":BUILD",
        "@io_bazel_rules_go//:README.md",
    ],
    out = "embed_local.go",
    package = "go_embed_data",
    var = "local",
)

go_embed_data(
    name = "ext",
    srcs = ["@io_bazel_rules_go_repository_tools//:BUILD.bazel"],
    out = "embed_ext.go",
    package = "go_embed_data",
    var = "ext",
)

go_embed_data(
    name = "flat",
    srcs = [":BUILD"],
    out = "embed_flat.go",
    flatten = True,
    package = "go_embed_data",
    var = "flat",
)

go_embed_data(
    name = "str",
    srcs = [":BUILD"],
    out = "embed_str.go",
    package = "go_embed_data",
    string = True,
    var = "str",
)
