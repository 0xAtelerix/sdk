fn main() {
    tonic_build::configure()
        .build_server(true)
        .build_client(true)
        .compile_protos(&["../proto/atelerix.proto"], &["../proto"])
        .expect("Failed to compile gRPC definitions");
}
